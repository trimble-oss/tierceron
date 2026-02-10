package oauth

import (
	"context"
	"fmt"
	"html"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

// BrowserLoginResult holds the result of a browser-based OAuth login
type BrowserLoginResult struct {
	Code  string
	State string
	Error string
}

// LocalServerConfig configures the local callback server
type LocalServerConfig struct {
	Port         int           // Port to listen on (0 for random)
	Path         string        // Callback path (default: "/callback")
	ReadTimeout  time.Duration // HTTP read timeout
	WriteTimeout time.Duration // HTTP write timeout
	// HandlerRegisterFunc allows external registration of the callback handler
	// If provided, LoginWithBrowser will not create its own HTTP server
	// Instead, it calls this function to register the handler and waits for callback
	HandlerRegisterFunc func(path string, handler http.Handler) error
}

// LoginWithBrowser initiates an OAuth flow by opening a browser and starting a local callback server
func LoginWithBrowser(ctx context.Context, client *Client, config *LocalServerConfig) (*TokenResponse, error) {
	// Generate security parameters
	state, err := GenerateState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	nonce, err := GenerateNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	pkce, err := GeneratePKCEChallenge()
	if err != nil {
		return nil, fmt.Errorf("failed to generate PKCE challenge: %w", err)
	}

	// Start local callback server
	if config == nil {
		config = &LocalServerConfig{
			Port:         8080,
			Path:         "/callback",
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}
	}
	if config.Path == "" {
		config.Path = "/callback"
	}

	resultChan := make(chan *BrowserLoginResult, 1)
	var serverErr error
	var wg sync.WaitGroup

	// Create callback handler
	callbackHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := &BrowserLoginResult{
			Code:  r.URL.Query().Get("code"),
			State: r.URL.Query().Get("state"),
			Error: r.URL.Query().Get("error"),
		}

		if result.Error != "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "<html><body><h1>Authentication Failed</h1><p>Error: %s</p><p>You can close this window.</p></body></html>", html.EscapeString(result.Error))
		} else {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "<html><body><h1>Authentication Successful</h1><p>You can close this window and return to the terminal.</p></body></html>")
		}

		select {
		case resultChan <- result:
		default:
		}
	})

	// Use external handler registration if provided
	if config.HandlerRegisterFunc != nil {
		// Register handler externally (e.g., with procurator)
		if err := config.HandlerRegisterFunc(config.Path, callbackHandler); err != nil {
			return nil, fmt.Errorf("failed to register OAuth callback handler: %w", err)
		}
	} else {
		// Create our own HTTP server
		wg.Add(1)
		mux := http.NewServeMux()
		mux.Handle(config.Path, callbackHandler)

		server := &http.Server{
			Addr:         fmt.Sprintf(":%d", config.Port),
			Handler:      mux,
			ReadTimeout:  config.ReadTimeout,
			WriteTimeout: config.WriteTimeout,
		}

		// Start server in background
		go func() {
			defer wg.Done()
			listener, err := net.Listen("tcp", server.Addr)
			if err != nil {
				serverErr = fmt.Errorf("failed to start callback server: %w", err)
				resultChan <- &BrowserLoginResult{Error: serverErr.Error()}
				return
			}
			defer listener.Close()

			// Update port if it was 0 (random)
			config.Port = listener.Addr().(*net.TCPAddr).Port

			if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
				serverErr = fmt.Errorf("callback server error: %w", err)
			}
		}()

		// Wait a moment for server to start
		time.Sleep(100 * time.Millisecond)
		if serverErr != nil {
			return nil, serverErr
		}

		// Cleanup server on exit
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			server.Shutdown(shutdownCtx)
			wg.Wait()
		}()
	}

	// Build authorization URL
	authURL := client.AuthorizationURL(state, nonce, pkce)

	// Open browser
	fmt.Printf("Opening browser for authentication...\n")
	fmt.Printf("If the browser doesn't open automatically, visit:\n%s\n\n", authURL)
	if err := openBrowser(authURL); err != nil {
		fmt.Printf("Failed to open browser automatically: %v\n", err)
	}

	// Wait for callback or context cancellation
	var result *BrowserLoginResult
	select {
	case result = <-resultChan:
	case <-ctx.Done():
		return nil, fmt.Errorf("login cancelled: %w", ctx.Err())
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("login timeout")
	}

	// Check for errors
	if result.Error != "" {
		return nil, fmt.Errorf("authentication error: %s", result.Error)
	}

	// Verify state
	if result.State != state {
		return nil, fmt.Errorf("state mismatch - possible CSRF attack")
	}

	// Exchange code for tokens
	tokens, err := client.ExchangeCode(ctx, result.Code, pkce)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for tokens: %w", err)
	}

	return tokens, nil
}

// openBrowser opens the default browser to the specified URL
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default: // linux, freebsd, openbsd, netbsd
		cmd = exec.Command("xdg-open", url)
	}

	return cmd.Start()
}

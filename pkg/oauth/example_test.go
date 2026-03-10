package oauth_test

import (
	"context"
	"fmt"
	"log"

	"github.com/trimble-oss/tierceron/pkg/oauth"
)

// Example demonstrates a complete OAuth login flow with browser
func Example_browserLogin() {
	ctx := context.Background()

	// Create OAuth client from discovery URL
	client, err := oauth.NewClientFromDiscovery(
		"https://tierceron.test/.well-known/openid-configuration",
		"your-client-id",
		"", // Empty for public client (no client secret)
		"http://localhost:8080/callback",
		[]string{"openid", "profile", "email"},
	)
	if err != nil {
		log.Fatal(err)
	}

	// Perform browser-based login
	// This will:
	// 1. Start a local callback server
	// 2. Open the default browser to the authorization URL
	// 3. Wait for the user to complete authentication
	// 4. Receive the callback and exchange the code for tokens
	tokens, err := oauth.LoginWithBrowser(ctx, client, &oauth.LocalServerConfig{
		Port: 8080, // Use 0 for random port
	})
	if err != nil {
		log.Fatal(err)
	}

	// Parse ID token to get user information
	claims, err := oauth.ParseIDToken(tokens.IDToken)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Logged in as: %s (%s)\n", claims.Name, claims.Email)
	fmt.Printf("User ID: %s\n", claims.Subject)
}

// Example demonstrates manual OAuth flow without browser automation
func Example_manualFlow() {
	ctx := context.Background()

	// Fetch discovery document
	discovery, err := oauth.GetDiscovery("https://tierceron.test/.well-known/openid-configuration")
	if err != nil {
		log.Fatal(err)
	}

	// Create client
	client := oauth.NewClient(&oauth.Config{
		ClientID:    "your-client-id",
		RedirectURL: "http://localhost:8080/callback",
		Scopes:      []string{"openid", "profile", "email"},
		Discovery:   discovery,
	})

	// Generate PKCE challenge
	pkce, err := oauth.GeneratePKCEChallenge()
	if err != nil {
		log.Fatal(err)
	}

	// Generate state and nonce for security
	state, _ := oauth.GenerateState()
	nonce, _ := oauth.GenerateNonce()

	// Build authorization URL
	authURL := client.AuthorizationURL(state, nonce, pkce)
	fmt.Printf("Visit this URL to authenticate:\n%s\n", authURL)

	// In a real implementation, you would:
	// 1. Direct user to authURL
	// 2. Handle callback to get 'code' parameter
	// 3. Verify state matches
	// 4. Exchange code for tokens

	// Example code exchange (assuming you got the code from callback)
	code := "received-authorization-code"
	tokens, err := client.ExchangeCode(ctx, code, pkce)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Access Token: %s\n", tokens.AccessToken)
}

// Example demonstrates token refresh
func Example_refreshToken() {
	ctx := context.Background()

	client, _ := oauth.NewClientFromDiscovery(
		"https://tierceron.test/.well-known/openid-configuration",
		"your-client-id",
		"",
		"http://localhost:8080/callback",
		[]string{"openid", "profile", "email"},
	)

	// Assume you have a refresh token from a previous login
	refreshToken := "your-refresh-token"

	// Exchange refresh token for new tokens
	tokens, err := client.RefreshToken(ctx, refreshToken)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("New access token: %s\n", tokens.AccessToken)
}

// Example demonstrates PKCE challenge generation
func Example_pkce() {
	// Generate PKCE challenge
	pkce, err := oauth.GeneratePKCEChallenge()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Code Verifier: %s\n", pkce.Verifier)
	fmt.Printf("Code Challenge: %s\n", pkce.Challenge)
	fmt.Printf("Challenge Method: %s\n", pkce.Method)
}

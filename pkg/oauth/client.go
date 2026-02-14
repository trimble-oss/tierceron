package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config holds the OAuth/OIDC client configuration
type Config struct {
	ClientID     string
	ClientSecret string // Optional for public clients
	RedirectURL  string
	Scopes       []string
	Discovery    *OIDCDiscovery
}

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// Client represents an OAuth/OIDC client
type Client struct {
	config *Config
}

// NewClient creates a new OAuth client with the given configuration
func NewClient(config *Config) *Client {
	return &Client{config: config}
}

// NewClientFromDiscovery creates a new OAuth client by fetching the discovery document
func NewClientFromDiscovery(discoveryURL, clientID, clientSecret, redirectURL string, scopes []string) (*Client, error) {
	discovery, err := GetDiscovery(discoveryURL)
	if err != nil {
		return nil, err
	}

	return &Client{
		config: &Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       scopes,
			Discovery:    discovery,
		},
	}, nil
}

// AuthorizationURL builds the authorization URL for the OAuth flow
func (c *Client) AuthorizationURL(state, nonce string, pkce *PKCEChallenge) string {
	return c.AuthorizationURLWithPrompt(state, nonce, pkce, false)
}

// AuthorizationURLWithPrompt builds the authorization URL with optional login prompt
// Set forceLoginPrompt to true to force a new login even if a session exists
func (c *Client) AuthorizationURLWithPrompt(state, nonce string, pkce *PKCEChallenge, forceLoginPrompt bool) string {
	params := url.Values{}
	params.Set("client_id", c.config.ClientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", c.config.RedirectURL)
	params.Set("scope", strings.Join(c.config.Scopes, " "))
	params.Set("state", state)

	if forceLoginPrompt {
		params.Set("prompt", "login")
	}

	if nonce != "" {
		params.Set("nonce", nonce)
	}

	if pkce != nil {
		params.Set("code_challenge", pkce.Challenge)
		params.Set("code_challenge_method", pkce.Method)
	}

	return c.config.Discovery.AuthorizationEndpoint + "?" + params.Encode()
}

// ExchangeCode exchanges an authorization code for tokens
func (c *Client) ExchangeCode(ctx context.Context, code string, pkce *PKCEChallenge) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", c.config.RedirectURL)
	data.Set("client_id", c.config.ClientID)

	if c.config.ClientSecret != "" {
		data.Set("client_secret", c.config.ClientSecret)
	}

	if pkce != nil {
		data.Set("code_verifier", pkce.Verifier)
	}

	return c.requestToken(ctx, data)
}

// RefreshToken exchanges a refresh token for new tokens
func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", c.config.ClientID)

	if c.config.ClientSecret != "" {
		data.Set("client_secret", c.config.ClientSecret)
	}

	return c.requestToken(ctx, data)
}

// requestToken sends a token request to the token endpoint
func (c *Client) requestToken(ctx context.Context, data url.Values) (*TokenResponse, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.config.Discovery.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &tokenResp, nil
}

// LogoutURL builds the logout URL for ending the session
func (c *Client) LogoutURL(idTokenHint, postLogoutRedirectURI string) string {
	if c.config.Discovery.EndSessionEndpoint == "" {
		return ""
	}

	params := url.Values{}
	if idTokenHint != "" {
		params.Set("id_token_hint", idTokenHint)
	}
	if postLogoutRedirectURI != "" {
		params.Set("post_logout_redirect_uri", postLogoutRedirectURI)
	}

	return c.config.Discovery.EndSessionEndpoint + "?" + params.Encode()
}

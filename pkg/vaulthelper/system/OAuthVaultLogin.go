package system

import (
	"context"
	"fmt"

	"github.com/trimble-oss/tierceron/pkg/oauth"
)

// OAuthLoginConfig holds configuration for OAuth-based Vault login
type OAuthLoginConfig struct {
	// OAuth/OIDC configuration
	OIDCDiscoveryURL string
	ClientID         string
	ClientSecret     string   // Optional for public clients
	RedirectURL      string   // e.g., "http://localhost:8080/callback"
	Scopes           []string // e.g., []string{"openid", "profile", "email"}

	// Local callback server configuration
	CallbackPort int // Port for local OAuth callback server (default: 8080)

	// Vault JWT configuration
	JWTRole string // Vault JWT role name to authenticate against

	// LocalServerConfig provides advanced callback configuration
	// If set, Port, Path, and HandlerRegisterFunc from this will be used
	// This allows OAuth callbacks to be handled by external servers (e.g., procurator)
	LocalServerConfig *oauth.LocalServerConfig
}

// OAuthLoginResult holds the result of an OAuth-based Vault login
type OAuthLoginResult struct {
	VaultToken   string
	IDToken      string
	AccessToken  string
	RefreshToken string
	UserEmail    string
	UserName     string
	UserSubject  string
}

// LoginWithOAuth performs a complete OAuth flow and authenticates to Vault using the resulting ID token
//
// This method:
// 1. Opens a browser for user to authenticate with OIDC provider (e.g., Identity provider)
// 2. Receives the OAuth callback with authorization code
// 3. Exchanges code for tokens (including ID token)
// 4. Authenticates to Vault using the ID token and specified JWT role
// 5. Returns Vault token and user information
//
// Example usage:
//
//	config := &OAuthLoginConfig{
//	    OIDCDiscoveryURL: "https://id.trimble.com/.well-known/openid-configuration",
//	    ClientID:         os.Getenv("OAUTH_CLIENT_ID"),
//	    ClientSecret:     "",  // Empty for public client
//	    RedirectURL:      "http://localhost:8080/callback",
//	    Scopes:           []string{"openid", "profile", "email"},
//	    CallbackPort:     8080,
//	    JWTRole:          "trcshhivez",
//	}
//
//	result, err := vault.LoginWithOAuth(context.Background(), config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	vault.SetToken(&result.VaultToken)
//	// Now vault client is authenticated and can access policies granted by JWT role
func (v *Vault) LoginWithOAuth(ctx context.Context, config *OAuthLoginConfig) (*OAuthLoginResult, error) {
	// Create OAuth client
	oauthClient, err := oauth.NewClientFromDiscovery(
		config.OIDCDiscoveryURL,
		config.ClientID,
		config.ClientSecret,
		config.RedirectURL,
		config.Scopes,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OAuth client: %w", err)
	}

	// Set callback port
	callbackPort := config.CallbackPort
	if callbackPort == 0 {
		callbackPort = 8080
	}

	// Configure local server for OAuth callback
	var serverConfig *oauth.LocalServerConfig
	if config.LocalServerConfig != nil {
		// Use provided configuration
		serverConfig = config.LocalServerConfig
	} else {
		// Create default configuration
		serverConfig = &oauth.LocalServerConfig{
			Port: callbackPort,
		}
	}

	// Perform browser-based OAuth login
	tokens, err := oauth.LoginWithBrowser(ctx, oauthClient, serverConfig)
	if err != nil {
		return nil, fmt.Errorf("OAuth login failed: %w", err)
	}

	// Parse ID token to get user information
	claims, err := oauth.ParseIDToken(tokens.IDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ID token: %w", err)
	}

	// Authenticate to Vault using ID token
	vaultToken, err := v.JWTLogin(tokens.IDToken, config.JWTRole)
	if err != nil {
		return nil, fmt.Errorf("Vault JWT authentication failed: %w", err)
	}

	return &OAuthLoginResult{
		VaultToken:   *vaultToken,
		IDToken:      tokens.IDToken,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		UserEmail:    claims.Email,
		UserName:     claims.Name,
		UserSubject:  claims.Subject,
	}, nil
}

// GetAppRoleCredentialsWithOAuth performs OAuth login and uses the resulting Vault token to retrieve AppRole credentials
//
// This is a convenience method that combines OAuth authentication with AppRole credential retrieval.
// It's useful for applications that need to:
// 1. Authenticate users via OAuth/OIDC
// 2. Use that authentication to get long-lived AppRole credentials from Vault
//
// The JWT role must have policies that allow:
// - read on auth/approle/role/<roleName>/role-id
// - update on auth/approle/role/<roleName>/secret-id
//
// Example usage:
//
//	config := &OAuthLoginConfig{
//	    OIDCDiscoveryURL: "https://id.trimble.com/.well-known/openid-configuration",
//	    ClientID:         os.Getenv("OAUTH_CLIENT_ID"),
//	    RedirectURL:      "http://localhost:8080/callback",
//	    Scopes:           []string{"openid", "profile", "email"},
//	    JWTRole:          "trcshhivez",
//	}
//
//	roleID, secretID, userInfo, err := vault.GetAppRoleCredentialsWithOAuth(ctx, config, "trcshhivez")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Store roleID and secretID securely for future use
//	// These credentials can be used without further OAuth authentication
func (v *Vault) GetAppRoleCredentialsWithOAuth(ctx context.Context, config *OAuthLoginConfig, appRoleName string) (roleID string, secretID string, userInfo *OAuthLoginResult, err error) {
	// Perform OAuth login and get Vault token
	result, err := v.LoginWithOAuth(ctx, config)
	if err != nil {
		return "", "", nil, err
	}

	// Set the Vault token from OAuth login
	v.SetToken(&result.VaultToken)

	// Get AppRole role-id
	roleID, _, err = v.GetRoleID(appRoleName)
	if err != nil {
		return "", "", result, fmt.Errorf("failed to get role-id: %w", err)
	}

	// Generate a new secret-id
	secretID, err = v.GetSecretID(appRoleName)
	if err != nil {
		return "", "", result, fmt.Errorf("failed to get secret-id: %w", err)
	}

	return roleID, secretID, result, nil
}

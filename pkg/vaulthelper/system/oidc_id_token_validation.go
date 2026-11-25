package system

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/trimble-oss/tierceron-core/v2/buildopts/memonly"
	"github.com/trimble-oss/tierceron-core/v2/identity"
	"github.com/trimble-oss/tierceron/pkg/oauth"
)

// ValidateOIDCIDToken verifies elevated access using Vault's JWT and role policy checks.
func (v *Vault) ValidateOIDCIDToken(ctx context.Context, token string) error {
	claims, err := oauth.ParseIDToken(token)
	if err != nil || claims.ExpiresAt == 0 {
		return fmt.Errorf("invalid OIDC ID token")
	}
	// Unverified expiry is only a reauthentication hint, never authorization.
	if claims.ExpiresAt <= time.Now().Unix() {
		return identity.ErrOIDCIDTokenExpired
	}
	if memonly.IsMemonly() {
		if err := protectMemory(nil, &token); err != nil {
			return err
		}
	}
	secret, err := v.client.Logical().WriteWithContext(ctx, "auth/jwt/login", map[string]any{
		"jwt": token, "role": "trcshunrestricted",
	})
	if err != nil {
		var response *api.ResponseError
		if errors.As(err, &response) && (response.StatusCode == 400 || response.StatusCode == 403) {
			if claims.ExpiresAt <= time.Now().Unix() {
				return identity.ErrOIDCIDTokenExpired
			}
			return fmt.Errorf("user authentication denied")
		}
		return identity.ErrUserAuthUnavailable
	}
	if secret == nil || secret.Auth == nil || secret.Auth.ClientToken == "" {
		return fmt.Errorf("user authentication returned no Vault identity")
	}
	clientToken := secret.Auth.ClientToken
	var protectionErr error
	if memonly.IsMemonly() {
		protectionErr = protectMemory(nil, &clientToken)
	}
	previousToken := v.client.Token()
	v.client.SetToken(clientToken)
	defer v.client.SetToken(previousToken)
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := v.client.Auth().Token().RevokeSelfWithContext(cleanup, clientToken); err != nil {
		return fmt.Errorf("%w: token cleanup failed", identity.ErrUserAuthUnavailable)
	}
	if protectionErr != nil {
		return fmt.Errorf("%w: token memory protection failed", identity.ErrUserAuthUnavailable)
	}
	return nil
}

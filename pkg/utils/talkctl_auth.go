package utils

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/trimble-oss/tierceron-core/v2/identity"
	"github.com/trimble-oss/tierceron/pkg/oauth"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

func TalkctlUserTokenProvider(driver *config.DriverConfig) func(context.Context, bool) (string, error) {
	var mu sync.Mutex
	return func(ctx context.Context, force bool) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		if err := ctx.Err(); err != nil {
			return "", err
		}
		cache := driver.CoreConfig.TokenCache
		token := cache.GetToken(identity.UserTokenCacheKey)
		if token == nil || *token == "" {
			return "", fmt.Errorf("no elevated user session; run su")
		}
		claims, err := oauth.ParseIDToken(*token)
		if err != nil {
			return "", fmt.Errorf("invalid elevated user session; run su")
		}
		if force || claims.ExpiresAt <= time.Now().Unix() {
			cache.RemoveToken(identity.UserTokenCacheKey)
			if err := KernelZOAuthForRole(driver, "trcshunrestricted", false, true, ctx); err != nil {
				if restoreErr := cache.AddToken(identity.UserTokenCacheKey, token); restoreErr != nil {
					return "", fmt.Errorf("reauthentication failed: %w; session could not be retained: %w", err, restoreErr)
				}
				return "", err
			}
			token = cache.GetToken(identity.UserTokenCacheKey)
		}
		if token == nil || *token == "" {
			return "", fmt.Errorf("user reauthentication returned no identity")
		}
		return *token, nil
	}
}

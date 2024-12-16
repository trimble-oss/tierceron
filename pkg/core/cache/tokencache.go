package cache

import (
	"errors"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
)

type TokenCache struct {
	cache *cmap.ConcurrentMap[string, *string] // tokenKey, *token
}

func NewTokenCacheEmpty() *TokenCache {
	ccmap := cmap.New[*string]()
	return &TokenCache{cache: &ccmap}
}

func NewTokenCache(tokenKey string, token *string) *TokenCache {
	if token == nil || len(*token) == 0 {
		return NewTokenCacheEmpty()
	}
	ccmap := cmap.New[*string]()
	ccmap.Set(tokenKey, token)
	return &TokenCache{cache: &ccmap}
}

func (tc *TokenCache) AddToken(tokenKey string, token *string) error {
	if len(tokenKey) == 0 {
		return errors.New("key cannot be empty")
	}
	if token == nil || len(*token) == 0 {
		return errors.New("token nil or empty")
	}
	if memonly.IsMemonly() {
		memprotectopts.MemProtect(nil, token)
	}
	tc.cache.Set(tokenKey, token)
	return nil
}

func (tc *TokenCache) GetToken(tokenKey string) *string {
	if tc.cache == nil {
		return nil
	}
	if token, ok := tc.cache.Get(tokenKey); ok {
		return token
	} else {
		return nil
	}
}

func (tc *TokenCache) Clear() {
	tc.cache.Clear()
}

package cache

import (
	"errors"

	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
)

type TokenCache struct {
	cache map[string]*string // tokenKey, *token
}

func NewTokenCacheEmpty() *TokenCache {
	return &TokenCache{cache: map[string]*string{}}
}

func NewTokenCache(tokenKey string, token *string) *TokenCache {
	if token == nil || len(*token) == 0 {
		return NewTokenCacheEmpty()
	}
	return &TokenCache{cache: map[string]*string{
		tokenKey: token,
	}}
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
	tc.cache[tokenKey] = token
	return nil
}

func (tc *TokenCache) GetToken(tokenKey string) *string {
	if tc.cache == nil {
		return nil
	}
	return tc.cache[tokenKey]
}

func (tc *TokenCache) Clear() {
	tc.cache = map[string]*string{}
}

package cache

import (
	"errors"
	"strings"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/trimble-oss/tierceron-core/v2/buildopts/memonly"
	"github.com/trimble-oss/tierceron-core/v2/buildopts/memprotectopts"
)

type TokenCache struct {
	VaultAddressPtr *string                                // Vault address
	rcache          *cmap.ConcurrentMap[string, *[]string] // role name, role/secret
	cache           *cmap.ConcurrentMap[string, *string]   // tokenKey, *token
}

func NewTokenCacheEmpty(varVaptr ...*string) *TokenCache {
	ccmap := cmap.New[*string]()
	rmap := cmap.New[*[]string]()
	tc := &TokenCache{rcache: &rmap, cache: &ccmap}
	if len(varVaptr) > 0 {
		tc.SetVaultAddress(varVaptr[0])
	}
	return tc
}

func NewTokenCache(tokenKey string, token *string, vaptr *string) *TokenCache {
	if token == nil || len(*token) == 0 {
		return NewTokenCacheEmpty(vaptr)
	}
	ccmap := cmap.New[*string]()
	ccmap.Set(tokenKey, token)
	rmap := cmap.New[*[]string]()
	tc := &TokenCache{rcache: &rmap, cache: &ccmap}
	tc.SetVaultAddress(vaptr)
	return tc
}

func (tc *TokenCache) IsEmpty() bool {
	return tc.cache.IsEmpty() && tc.rcache.IsEmpty()
}

func (tc *TokenCache) SetVaultAddress(vaptr *string) error {
	if vaptr == nil || len(*vaptr) == 0 {
		return errors.New("Vault address nil or empty")
	}
	if memonly.IsMemonly() {
		memprotectopts.MemProtect(nil, vaptr)
	}
	tc.VaultAddressPtr = vaptr
	return nil
}

func (tc *TokenCache) AddRoleStr(roleKey string, role *string) error {
	if len(roleKey) == 0 {
		return errors.New("key cannot be empty")
	}
	if role == nil || len(*role) == 0 {
		return errors.New("role nil or empty")
	}
	if memonly.IsMemonly() {
		memprotectopts.MemProtect(nil, role)
	}

	roleSlice := strings.Split(*role, ":")
	return tc.AddRole(roleKey, &roleSlice)
}

func (tc *TokenCache) AddRole(roleKey string, roleSlice *[]string) error {
	if len(roleKey) == 0 {
		return errors.New("key cannot be empty")
	}
	if roleSlice == nil || len(*roleSlice) == 0 {
		return errors.New("role nil or empty")
	}
	if memonly.IsMemonly() {
		for i := range len(*roleSlice) {
			memprotectopts.MemProtect(nil, &(*roleSlice)[i])
		}
	}
	tc.rcache.Set(roleKey, roleSlice)
	return nil
}

func (tc *TokenCache) GetRole(roleKey string) *[]string {
	return tc.GetRoleStr(&roleKey)
}

func (tc *TokenCache) GetRoleWithDefault(roleKey string, defaultRole string) *[]string {
	return tc.GetRoleStr(&roleKey)
}

func (tc *TokenCache) GetRoleStr(roleKey *string) *[]string {
	if roleKey == nil {
		return nil
	}
	if tc.rcache == nil {
		return nil
	}
	if role, ok := tc.rcache.Get(*roleKey); ok {
		return role
	} else {
		return nil
	}
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

func (tc *TokenCache) GetTokenStr(tokenKeyPtr *string) *string {
	return tc.GetToken(*tokenKeyPtr)
}

func (tc *TokenCache) Clear() {
	tc.cache.Clear()
}

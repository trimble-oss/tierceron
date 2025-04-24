package core

import (
	"fmt"
	"log"
	"strings"

	"github.com/trimble-oss/tierceron/pkg/core/cache"
)

// This structure contains core properties central to Secrets engine access
type CoreConfig struct {
	IsProdPtr *bool
	IsShell   bool // If tool running in shell.

	// Vault Configurations...
	Insecure             bool
	CurrentTokenNamePtr  *string // Pointer to one of the tokens in the cache...  changes depending on context.
	CurrentRoleEntityPtr *string // Pointer to one of the roles in the cache...  changes depending on context.
	TokenCache           *cache.TokenCache
	EnvBasis             string // dev,QA, etc....
	Env                  string // dev-1, dev-2, etc...
	Regions              []string

	DynamicPathFilter string // Seeds from a specific path.
	WantCerts         bool
	ExitOnFailure     bool // Exit on a failure or try to continue
	Log               *log.Logger
}

func (cc *CoreConfig) IsProd() bool {
	if cc.IsProdPtr == nil {
		cc.IsProdPtr = new(bool)
		var envCheck string
		if len(cc.EnvBasis) > 0 {
			envCheck = cc.EnvBasis
		} else {
			envCheck = cc.Env
		}
		*cc.IsProdPtr = strings.Contains(envCheck, "staging") || strings.Contains(envCheck, "prod")
	}
	return *cc.IsProdPtr
}

func (cc *CoreConfig) GetCurrentToken(tokenPattern string) *string {
	tokenPrefix := fmt.Sprintf(tokenPattern, cc.EnvBasis)
	if cc.CurrentTokenNamePtr != nil && strings.HasPrefix(*cc.CurrentTokenNamePtr, tokenPrefix) {
		return cc.CurrentTokenNamePtr
	} else {
		return &tokenPrefix
	}
}

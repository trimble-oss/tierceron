package core

import (
	"log"

	"github.com/trimble-oss/tierceron/pkg/core/cache"
)

// This structure contains core properties central to Secrets engine access
type CoreConfig struct {
	IsShell bool // If tool running in shell.

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

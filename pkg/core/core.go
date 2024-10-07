package core

import (
	"log"

	"github.com/trimble-oss/tierceron/pkg/core/cache"
)

// This structure contains core properties central to Secrets engine access
type CoreConfig struct {
	IsShell bool // If tool running in shell.

	// Vault Configurations...
	Insecure         bool
	TokenCache       *cache.TokenCache
	AppRoleConfigPtr *string
	VaultAddressPtr  *string
	EnvBasis         string // dev,QA, etc....
	Env              string // dev-1, dev-2, etc...
	Regions          []string

	DynamicPathFilter string // Seeds from a specific path.
	WantCerts         bool
	ExitOnFailure     bool // Exit on a failure or try to continue
	Log               *log.Logger
}

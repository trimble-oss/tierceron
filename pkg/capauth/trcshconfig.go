package capauth

import (
	"fmt"

	"github.com/trimble-oss/tierceron/pkg/core/cache"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

type TrcShConfig struct {
	IsShellRunner   bool
	Env             string
	EnvContext      string // Current env context...
	VaultAddressPtr *string
	TokenCache      *cache.TokenCache
	ConfigRolePtr   *string
	PubRolePtr      *string
	KubeConfigPtr   *string
}

func (trcshConfig *TrcShConfig) IsValid(agentConfigs *AgentConfigs) bool {
	if agentConfigs == nil {
		// Driver needs a lot more permissions to run...
		return eUtils.RefLength(trcshConfig.ConfigRolePtr) > 0 &&
			eUtils.RefLength(trcshConfig.PubRolePtr) > 0 &&
			eUtils.RefLength(trcshConfig.KubeConfigPtr) > 0 &&
			eUtils.RefLength(trcshConfig.VaultAddressPtr) > 0
	} else {
		if trcshConfig.IsShellRunner && trcshConfig.TokenCache != nil && trcshConfig.TokenCache.GetToken(fmt.Sprintf("config_token_%s_unrestricted", trcshConfig.Env)) != nil {
			return true
		} else {
			// Agent
			return eUtils.RefLength(trcshConfig.ConfigRolePtr) > 0 && eUtils.RefLength(trcshConfig.VaultAddressPtr) > 0
		}
	}
}

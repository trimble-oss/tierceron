package capauth

import (
	"fmt"

	"github.com/trimble-oss/tierceron/pkg/core/cache"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

type TrcShConfig struct {
	IsShellRunner bool
	Env           string
	EnvContext    string // Current env context...
	TokenCache    *cache.TokenCache
	KubeConfigPtr *string
}

func (trcshConfig *TrcShConfig) IsValid(agentConfigs *AgentConfigs) bool {
	if agentConfigs == nil {
		// Driver needs a lot more permissions to run...
		return eUtils.RefSliceLength(trcshConfig.TokenCache.GetRole("bamboo")) > 0 &&
			eUtils.RefSliceLength(trcshConfig.TokenCache.GetRole("pub")) > 0 &&
			eUtils.RefLength(trcshConfig.KubeConfigPtr) > 0 &&
			eUtils.RefLength(trcshConfig.TokenCache.VaultAddressPtr) > 0
	} else {
		if trcshConfig.IsShellRunner && trcshConfig.TokenCache != nil && trcshConfig.TokenCache.GetToken(fmt.Sprintf("config_token_%s_unrestricted", trcshConfig.Env)) != nil {
			return true
		} else {
			// Agent
			return eUtils.RefSliceLength(trcshConfig.TokenCache.GetRole("bamboo")) > 0 && eUtils.RefLength(trcshConfig.TokenCache.VaultAddressPtr) > 0
		}
	}
}

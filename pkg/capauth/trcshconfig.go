package capauth

import (
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

type TrcShConfig struct {
	Env             string
	EnvContext      string // Current env context...
	VaultAddressPtr *string
	TokenPtr        *string // Plugin token for read only access to plugin data
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
		// Agent
		return eUtils.RefLength(trcshConfig.ConfigRolePtr) > 0 && eUtils.RefLength(trcshConfig.VaultAddressPtr) > 0
	}
}

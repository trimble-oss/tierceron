package capauth

type TrcShConfig struct {
	Env          string
	EnvContext   string // Current env context...
	VaultAddress *string
	CToken       *string
	ConfigRole   *string
	PubRole      *string
	KubeConfig   *string
}

func (trcshConfig *TrcShConfig) IsValid(agentConfigs *AgentConfigs) bool {
	if agentConfigs == nil {
		// Driver needs a lot more permissions to run...
		return trcshConfig.ConfigRole != nil && trcshConfig.PubRole != nil &&
			trcshConfig.VaultAddress != nil && trcshConfig.KubeConfig != nil &&
			len(*trcshConfig.ConfigRole) > 0 && len(*trcshConfig.PubRole) > 0 &&
			len(*trcshConfig.VaultAddress) > 0 && len(*trcshConfig.KubeConfig) > 0
	} else {
		// Agent
		return trcshConfig.ConfigRole != nil && trcshConfig.VaultAddress != nil &&
			len(*trcshConfig.ConfigRole) > 0 && len(*trcshConfig.VaultAddress) > 0
	}
}

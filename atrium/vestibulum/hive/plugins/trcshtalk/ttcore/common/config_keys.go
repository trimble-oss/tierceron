package common

// Configuration keys
const (
	CfgTrcshTalkMode    = "trcshtalk_mode"
	CfgMode             = "mode"
	CfgServerMode       = "server_mode"
	CfgRemotePort       = "grpc_server_remote_port"
	CfgRemoteName       = "grpc_server_remote_name"
	CfgTrcshTalkHubPort = "trcshtalk_hub_port"
	CfgTrcshTalkHubName = "trcshtalk_hub_name"
	CfgTTBToken         = "ttb_token"
)

// Supported server modes
const (
	ModeStandard       = "standard"
	ModeTalkback       = "trcshtalkback"
	ModeTalkbackKernel = "talkback-kernel-plugin"
	ModeBoth           = "both"
	ModeHubClient      = "trcshtalkhubclient"
)

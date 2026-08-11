package common

// Configuration keys
const (
	CfgTrcshTalkMode    = "trcshtalk_mode"
	CfgMode             = "mode"
	CfgRemotePort       = "grpc_server_remote_port"
	CfgRemoteName       = "grpc_server_remote_name"
	CfgTrcshTalkHubPort = "trcshtalk_hub_port"
	CfgTrcshTalkHubName = "trcshtalk_hub_name"
	CfgTTBToken         = "ttb_token"
)

// Supported server modes
const (
	ModeTalkback  = "trcshtalkback"
	ModeHub       = "trcshtalkhub"
	ModeHubClient = "trcshtalkhubclient"
)

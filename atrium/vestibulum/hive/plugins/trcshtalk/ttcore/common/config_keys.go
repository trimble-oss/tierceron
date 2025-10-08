package common

// Configuration keys
const (
	CfgServerMode  = "server_mode"
	CfgRemotePort  = "grpc_server_remote_port"
	CfgRemoteName  = "grpc_server_remote_name"
	CfgTTBToken    = "ttb_token"
)

// Supported server modes
const (
	ModeStandard        = "standard"
	ModeTalkback        = "trcshtalkback"
	ModeTalkbackKernel  = "talkback-kernel-plugin"
	ModeBoth            = "both"
)

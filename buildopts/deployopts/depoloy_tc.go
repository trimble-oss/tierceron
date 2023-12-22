//go:build tc
// +build tc

package deployopts

import (
	"VaultConfig.TenantConfig/util/buildopts/deployers"
)

func InitSupportedDeployers() []string {
	deployers.InitSupportedDeployers()
}

func GetDecodedDeployerId(sessionId string) (string, error) {
	return deployers.GetDecodedDeployerId(*featherCtx.SessionIdentifier)
}

func GetEncodedDeployerId(deployment string, env string) (string, bool) {
	return deployers.GetEncodedDeployerId(deployment, *gAgentConfig.Env)
}

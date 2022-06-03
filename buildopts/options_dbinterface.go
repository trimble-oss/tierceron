//go:build dbinterface
// +build dbinterface

package buildopts

import (
	flowcore "tierceron/trcflow/core"

	tcutil "VaultConfig.TenantConfig/util"
	tcbuildopts "VaultConfig.TenantConfig/util/buildopts"

	tclib "VaultConfig.TenantConfig/lib"
	tvUtils "VaultConfig.TenantConfig/util/buildtrcprefix"
	tcharbinger "VaultConfig.TenantConfig/util/harbinger"
)

func SetLogger(logger interface{}) {
	return tclib.SetLogger(logger)
}

func SetErrorLogger(logger interface{}) {
	return tclib.SetErrorLogger(logger)
}

func GetLocalVaultAddr() string {
	return tcutil.GetLocalVaultAddr()
}

func GetSupportedSourceRegions() []string {
	return tcutil.GetSupportedSourceRegions()
}

func ProcessDeployPluginEnvConfig(pluginEnvConfig map[string]interface{}) map[string]interface{} {
	return tcutil.ProcessDeployPluginEnvConfig(pluginEnvConfig)
}

func ProcessFlowController(tfmContext *flowcore.TrcFlowMachineContext, trcFlowContext *flowcore.TrcFlowContext) error {
	return tcutil.ProcessFlowController(tfmContext, trcFlowContext)
}

func GetTestDeployConfig(token string) map[string]interface{} {
	return tcutil.GetTestDeployConfig(token)
}

func ProcessPluginEnvConfig(pluginEnvConfig map[string]interface{}) map[string]interface{} {
	return tcutil.ProcessPluginEnvConfig(pluginEnvConfig)
}

func GetSyncedTables() []string {
	return tcbuildopts.GetSyncedTables()
}

func GetDatabaseName() string {
	return tcutil.GetDatabaseName()
}

func GetExtensionAuthComponents(config map[string]interface{}) map[string]interface{} {
	return tcutil.GetExtensionAuthComponents(config)
}

// Build interface
func BuildInterface(config *eUtils.DriverConfig, goMod *kv.Modifier, tfmContext *flowcore.TrcFlowMachineContext, vaultDatabaseConfig map[string]interface{}) error {
	return tcharbinger.BuildInterface(config, goMod, tfmContext, vaultDatabaseConfig)
}

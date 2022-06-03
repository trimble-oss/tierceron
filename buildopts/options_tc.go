//go:build tc
// +build tc

package buildopts

import (
	"errors"
	flowcore "tierceron/trcflow/core"
	eUtils "tierceron/utils"

	tcutil "VaultConfig.TenantConfig/util"
	tcbuildopts "VaultConfig.TenantConfig/util/buildopts"

	tclib "VaultConfig.TenantConfig/lib"
	tcharbinger "VaultConfig.TenantConfig/util/harbinger"
	helperkv "tierceron/vaulthelper/kv"
)

func SetLogger(logger interface{}) {
	tclib.SetLogger(logger)
}

func SetErrorLogger(logger interface{}) {
	tclib.SetErrorLogger(logger)
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

func ProcessTestFlowController(tfmContext *flowcore.TrcFlowMachineContext, trcFlowContext *flowcore.TrcFlowContext) error {
	return errors.New("Table not implemented.")
}

func GetAdditionalFlows() []flowcore.FlowNameType {
	return []flowcore.FlowNameType{} // Noop
}

func GetAdditionalFlowsByState(teststate string) []flowcore.FlowNameType {
	return []flowcore.FlowNameType{} // Noop
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
func BuildInterface(config *eUtils.DriverConfig, goMod *helperkv.Modifier, tfmContext *flowcore.TrcFlowMachineContext, vaultDatabaseConfig map[string]interface{}) error {
	return tcharbinger.BuildInterface(config, goMod, tfmContext, vaultDatabaseConfig)
}

//go:build tc
// +build tc

package buildopts

import (
	tcbuildopts "VaultConfig.TenantConfig/util/buildopts"
	tccoreutil "VaultConfig.TenantConfig/util/core"

	tclib "VaultConfig.TenantConfig/lib"

	configcore "VaultConfig.Bootstrap/configcore"

	"database/sql"
	trcf "github.com/trimble-oss/tierceron/trcflow/core/flowcorehelper"
)

func SetLogger(logger interface{}) {
	tclib.SetLogger(logger)
}

func SetErrorLogger(logger interface{}) {
	tclib.SetErrorLogger(logger)
}

func GetLocalVaultAddr() string {
	return tccoreutil.GetLocalVaultAddr()
}

func GetSupportedSourceRegions() []string {
	return tccoreutil.GetSupportedSourceRegions()
}

func GetTestDeployConfig(token string) map[string]interface{} {
	return tccoreutil.GetTestDeployConfig(token)
}

func ProcessPluginEnvConfig(pluginEnvConfig map[string]interface{}) map[string]interface{} {
	return tccoreutil.ProcessPluginEnvConfig(pluginEnvConfig)
}

func GetFlowDatabaseName() string {
	return trcf.GetFlowDBName()
}

func GetExtensionAuthComponents(config map[string]interface{}) map[string]interface{} {
	return tccoreutil.GetExtensionAuthComponents(config)
}

func GetSyncedTables() []string {
	return tcbuildopts.GetSyncedTables()
}

func Authorize(db *sql.DB, userIdentifier string, userPassword string) (bool, string, error) {
	return configcore.Authorize(db, userIdentifier, userPassword)
}

// Whether to memlock data.
func CheckMemLock(bucket string, key string) bool {
	return tcbuildopts.CheckMemLock(bucket, key)
}

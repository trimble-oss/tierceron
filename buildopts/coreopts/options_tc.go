//go:build tc
// +build tc

package coreopts

import (
	bcore "VaultConfig.Bootstrap/configcore"
	tcbuildopts "VaultConfig.TenantConfig/util/buildopts"
	trcprefix "VaultConfig.TenantConfig/util/buildopts/trcprefix"
	tccore "VaultConfig.TenantConfig/util/core"

	"database/sql"
)

//
func GetFolderPrefix() string {
	return trcprefix.GetFolderPrefix()
}

func GetSupportedTemplates() []string {
	return bcore.GetSupportedTemplates(GetFolderPrefix())
}

func GetLocalHost() string {
	return bcore.LocalHost
}

func GetVaultHost() string {
	return bcore.VaultHost
}

func GetVaultHostPort() string {
	return bcore.VaultHostPort
}

// Begin old Active Sessions interface
func GetUserNameField() string {
	return bcore.UserNameField
}

func GetUserCodeField() string {
	return bcore.UserCodeField
}

func ActiveSessions(db *sql.DB) ([]map[string]interface{}, error) {
	return bcore.ActiveSessions(db)
}

// End old Active Sessions interface

func FindIndexForService(project string, service string) (string, []string, error) {
	return tcbuildopts.FindIndexForService(project, service)
}

func GetSyncedTables() []string {
	return tcbuildopts.GetSyncedTables()
}

func ProcessDeployPluginEnvConfig(pluginEnvConfig map[string]interface{}) map[string]interface{} {
	return tccore.ProcessDeployPluginEnvConfig(pluginEnvConfig)
}

func DecryptSecretConfig(tenantConfiguration map[string]interface{}, config map[string]interface{}) string {
	return bcore.DecryptSecretConfig(tenantConfiguration, config)
}

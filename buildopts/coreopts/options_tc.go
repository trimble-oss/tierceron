//go:build tc
// +build tc

package coreopts

import (
	configcore "VaultConfig.Bootstrap/configcore"
	tcbuildopts "VaultConfig.TenantConfig/util/buildopts"
	tvUtils "VaultConfig.TenantConfig/util/buildtrcprefix"

	"database/sql"
	sqle "github.com/dolthub/go-mysql-server/sql"
)

func GetSyncedTables() []string {
	return tcbuildopts.GetSyncedTables()
}

func GetIdColumnType(table string) sqle.Type {
	return tcbuildopts.GetIdColumnType(table)
}

func GetFolderPrefix() string {
	return tvUtils.GetFolderPrefix()
}

func DecryptSecretConfig(tenantConfiguration map[string]interface{}, config map[string]interface{}) string {
	return configcore.DecryptSecretConfig(tenantConfiguration, config)
}

func GetSupportedTemplates() []string {
	return configcore.GetSupportedTemplates()
}

func GetLocalHost() string {
	return configcore.LocalHost
}

func GetVaultHost() string {
	return configcore.VaultHost
}

func GetVaultHostPort() string {
	return configcore.VaultHostPort
}

func ActiveSessions(db *sql.DB) ([]map[string]interface{}, error) {
	return configcore.ActiveSessions(db)
}
func GetUserNameField() string {
	return configcore.UserNameField
}

func GetUserCodeField() string {
	return configcore.UserCodeField
}

func FindIndexForService(project string, service string) (string, error) {
	return tcbuildopts.FindIndexForService(project, service)
}

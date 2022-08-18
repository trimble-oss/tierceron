//go:build harbinger
// +build harbinger

package harbingeropts

import (
	configcore "VaultConfig.Bootstrap/configcore"
	tcbuildopts "VaultConfig.TenantConfig/util/buildopts"
	harbinger "VaultConfig.TenantConfig/util/buildopts/harbinger"
	//	trcprefix "VaultConfig.TenantConfig/util/buildopts/trcprefix"

	"database/sql"
	sqle "github.com/dolthub/go-mysql-server/sql"
)

// Database name to use for interface
func GetDatabaseName() string {
	return tcutil.GetDatabaseName()
}

// Build interface
func BuildInterface(config *eUtils.DriverConfig, goMod *helperkv.Modifier, tfmContext interface{}, vaultDatabaseConfig map[string]interface{}, serverListener interface{}) error {
	return harbinger.BuildInterface(config, goMod, tfmContext, vaultDatabaseConfig, serverListener)
}

func DecryptSecretConfig(tenantConfiguration map[string]interface{}, config map[string]interface{}) string {
	return configcore.DecryptSecretConfig(tenantConfiguration, config)
}

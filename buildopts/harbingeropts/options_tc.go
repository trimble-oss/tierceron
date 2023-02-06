//go:build harbinger
// +build harbinger

package harbingeropts

import (
	harbinger "VaultConfig.TenantConfig/util/buildopts/harbinger"
	//	trcprefix "VaultConfig.TenantConfig/util/buildopts/trcprefix"
	tccore "VaultConfig.TenantConfig/util/core"
	eUtils "github.com/trimble-oss/tierceron/utils"
	"github.com/trimble-oss/tierceron/vaulthelper/kv"
)

// Database name to use for interface
func GetDatabaseName() string {
	return tccore.GetDatabaseName()
}

// Build interface
func BuildInterface(config *eUtils.DriverConfig, goMod *kv.Modifier, tfmContext interface{}, vaultDatabaseConfig map[string]interface{}, serverListener interface{}) error {
	return harbinger.BuildInterface(config, goMod, tfmContext, vaultDatabaseConfig, serverListener)
}

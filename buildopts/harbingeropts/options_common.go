//go:build !harbinger
// +build !harbinger

package harbingeropts

import (
	"tierceron/vaulthelper/kv"

	eUtils "tierceron/utils"
)

func GetFolderPrefix() string {
	return "trc"
}

// Database name to use for interface
func GetDatabaseName() string {
	return ""
}

// Build interface
func BuildInterface(config *eUtils.DriverConfig, goMod *kv.Modifier, tfmContext interface{}, vaultDatabaseConfig map[string]interface{}, serverListener interface{}) error {
	return nil
}

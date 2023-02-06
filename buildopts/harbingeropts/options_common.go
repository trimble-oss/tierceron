//go:build !harbinger
// +build !harbinger

package harbingeropts

import (
	"github.com/trimble-oss/tierceron/vaulthelper/kv"

	eUtils "github.com/trimble-oss/tierceron/utils"
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

//go:build !harbinger
// +build !harbinger

package harbingeropts

import (
	"strings"

	"github.com/trimble-oss/tierceron/vaulthelper/kv"

	eUtils "github.com/trimble-oss/tierceron/utils"
)

func GetFolderPrefix(custom []string) string {
	if len(custom) > 0 && len(custom[0]) > 0 {
		var ti, endTi int
		ti = strings.Index(custom[0], "_templates")
		endTi = 0

		for endTi = ti; endTi > 0; endTi-- {
			if custom[0][endTi] == '/' {
				endTi = endTi + 1
				break
			}
		}
		return custom[0][endTi:ti]
	}
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

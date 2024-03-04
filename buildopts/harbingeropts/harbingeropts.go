package harbingeropts

import (
	"strings"

	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"

	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

// Folder prefix for _seed and _templates.  This function takes a list of paths and looking
// at the first entry, retrieve an embedded folder prefix.
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

// GetDatabaseName - returns a name to be used by TrcDb.
func GetDatabaseName() string {
	return ""
}

// Used to define a database interface for querying TrcDb.
func BuildInterface(config *eUtils.DriverConfig, goMod *kv.Modifier, tfmContext interface{}, vaultDatabaseConfig map[string]interface{}, serverListener interface{}) error {
	return nil
}

package coreopts

import (
	"database/sql"
	"errors"
	"os"
	"strings"
	"time"
)

// Folder prefix for _seed and _templates.  This function takes a list of paths and looking
// at the first entry, retrieve an embedded folder prefix.
func GetFolderPrefix(custom []string) string {
	if len(custom) > 0 && len(custom[0]) > 0 {
		var ti, endTi int
		ti = strings.Index(custom[0], "_templates")
		endTi = 0

		for endTi = ti; endTi > 0; endTi-- {
			if custom[0][endTi] == '/' || custom[0][endTi] == os.PathSeparator {
				endTi = endTi + 1
				break
			}
		}
		return custom[0][endTi:ti]
	}
	return "trc"
}

// GetSupportedTemplates - override to provide a list of supported certificate templates.
// This function serves as a gateway so that unintentional certificates or files are not picked
// up and inadvetently seeded into vault.
// example return value:
//
//	return []string{
//		folderPrefix + "_templates/Common/my.cer.mf.tmpl",
//		folderPrefix + "_templates/Common/my.pem.mf.tmpl",
//	}
func GetSupportedTemplates(custom []string) []string {
	return []string{}
}

// GetSupportedEndpoints - return a list of supported endpoints.  Override this function to provide
// a list of supported endpoints.
func GetSupportedEndpoints(prod bool) []string {
	return []string{}
}

// GetSupportedDomains - return a list of supported domains.  Override this function to provide
// a list of supported domains.
func GetSupportedDomains(prod bool) []string {
	return []string{}
}

// GetLocalHost - return the local host name.  Override this function to provide a custom local host name.
func GetLocalHost() string {
	return ""
}

// GetRegion - return the region.  Override this function to provide default region given a host name.
func GetRegion(hostName string) string {
	return ""
}

// GetVaultHost - return the vault host.  Override this function to provide a custom vault host.
func GetVaultHost() string {
	return ""
}

// GetVaultHost - return the vault host and port.  Override this function to provide a custom vault host and port.
func GetVaultHostPort() string {
	return ""
}

// GetUserNameField - return the user name field.  Override this function to provide a custom user name field.
// Used to provide active sessions in the web interface -- not maintained..
func GetUserNameField() string {
	return ""
}

// GetUserCodeField - return the user code field.  Override this function to provide a custom user code field.
// Used to provide active sessions in the web interface -- not maintained..
func GetUserCodeField() string {
	return ""
}

// Override to provide a map of active sessions by querying the provided database connection.
// Used to provide active sessions in the web interface -- not maintained..
func ActiveSessions(db *sql.DB) ([]map[string]interface{}, error) {
	return nil, errors.New("Not implemented")
}

// FindIndexForService - override to provide a custom index for a given service.  This should return
// the name of the column that is to be treated as the index for the table.
// TODO: This function is miss-named.  It should be called FindInexForTable where project = databaseName and service = tableName.
func FindIndexForService(project string, service string) (string, []string, string, error) {
	return "", nil, "", errors.New("Not implemented")
}

// GetSyncedTables - return a list of synced tables from a remote source in TrcDb.
// Override this function to provide a list of synced tables.
func GetSyncedTables() []string {
	return []string{}
}

// Utilized by carrier to indicate the following map attributes:
//
//		exitOnFailure - if true, the plugin will exit on failure
//		regions - a list of regions to be supported by the carrier
//		pluginNameList - a list of plugins to be supported by the carrier
//		               the carrier is responsible for keeping the indicated plugins
//		               up to date and deployed with certified code...
//	          example values: trcsh, trc-vault-plugin
//
//		templatePath - a list of template paths (presently 1 template) to the certification
//		               template utilized by plugins.  This template references the published template
//		               originating from the source:
//		                  installation/trcdb/trc_templates/TrcVault/Certify/config.yml.tmpl
//		logNamespace - a log namespace to be used by the carrier in logging.
func ProcessDeployPluginEnvConfig(pluginEnvConfig map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{}
}

// DecryptSecretConfig
//   - provides the secret to be used in obtaining a database connection when provided
//     with source database configuration attributes.  The config map contains
//     additional global attributes that can be utilized in decrypting an
//     encrypted password found within the source database configuration.
//
// returns: the decrypted password to be used in establishing a database connection.
func DecryptSecretConfig(sourceDatabaseConfigs map[string]interface{}, config map[string]interface{}) string {
	return ""
}

// Utlized to provide Data Flow Statistics components: database name in which the DFS resides and the index
// of the DataFlowStatistics table (argosId)
func GetDFSPathName() (string, string) {
	return "databasename", "argosId"
}

// GetDatabaseName - returns a name to be used by TrcDb.
func GetDatabaseName() string {
	return ""
}

const RFC_ISO_8601 = "2006-01-02 15:04:05 -0700 MST"

// Compares the lastModified field in the provided data flow statistics maps.
// It returns true if the lastModified fields are equal.  False otherwise.
// Override to provide alternate fields to match on in your flows for comparing lastModified or
// even other fields...
func CompareLastModified(dfStatMapA map[string]interface{}, dfStatMapB map[string]interface{}) bool {
	//Check if a & b are time.time
	//Check if they match.
	var lastModifiedA time.Time
	var lastModifiedB time.Time
	var timeErr error
	if lastMA, ok := dfStatMapA["lastModified"].(time.Time); !ok {
		if lmA, ok := dfStatMapA["lastModified"].(string); ok {
			lastModifiedA, timeErr = time.Parse(RFC_ISO_8601, lmA)
			if timeErr != nil {
				return false
			}
		}
	} else {
		lastModifiedA = lastMA
	}

	if lastMB, ok := dfStatMapA["lastModified"].(time.Time); !ok {
		if lmB, ok := dfStatMapA["lastModified"].(string); ok {
			lastModifiedB, timeErr = time.Parse(RFC_ISO_8601, lmB)
			if timeErr != nil {
				return false
			}
		}
	} else {
		lastModifiedB = lastMB
	}

	if lastModifiedA != lastModifiedB {
		return false
	}

	return true
}

// PreviousStateCheck - provides the previous state of a flow given the provided current state.
// All states for flows rotate in a 0-3 cycle.
func PreviousStateCheck(currentState int) int {
	switch currentState {
	case 0:
		return 3
	case 1:
		return 0
	case 2:
		return 1
	case 3:
		return 2
	default:
		return 3
	}
}

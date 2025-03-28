package coreopts

import (
	"database/sql"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/trimble-oss/tierceron/pkg/trcnet"
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
// up and inadvertently seeded into vault.
// example return value:
//
//	return []string{
//		folderPrefix + "_templates/Common/my.cer.mf.tmpl",
//		folderPrefix + "_templates/Common/my.pem.mf.tmpl",
//	}
func GetSupportedTemplates(custom []string) []string {
	return []string{}
}

// Determines if running tierceron in the default local development mode
// with the default test host.
func IsLocalEndpoint(addr string) bool {
	return strings.HasPrefix(addr, "https://tierceron.test:1234")
}

func GetVaultInstallRoot() string {
	return "/usr/local/vault"
}

// GetSupportedEndpoints - return a list of supported endpoints.  Override this function to provide
// a list of supported endpoints.
func GetSupportedEndpoints(prod bool) [][]string {
	if prod {
		return [][]string{
			{
				"prodtierceron.test",
				"n/a",
			},
		}
	} else {
		return [][]string{
			{
				"tierceron.test:1234",
				"127.0.0.1",
			},
		}
	}
}

// GetSupportedDomains - return a list of supported domains.  Override this function to provide
// a list of supported domains.
func GetSupportedDomains(prod bool) []string {
	return []string{"tierceron.test:1234"}
}

// GetLocalHost - return the local host name.  Override this function to provide a custom local host name.
func GetLocalHost() string {
	return "https://tierceron.test:1234"
}

// GetRegion - return the region.  Override this function to provide default region given a host name.
func GetRegion(hostName string) string {
	return "west"
}

// GetVaultHost - return the vault host.  Override this function to provide a custom vault host.
func GetVaultHost() string {
	return "tierceron.test:1234"
}

// GetVaultHost - return the vault host and port.  Override this function to provide a custom vault host and port.
func GetVaultHostPort() string {
	return "tierceron.test:1234"
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

// DecryptSecretConfig
//   - provides the secret to be used in obtaining a database connection when provided
//     with source database configuration attributes.  The config map contains
//     additional global attributes that can be utilized in decrypting an
//     encrypted password found within the source database configuration.
//
// returns: the decrypted password to be used in establishing a database connection.
func DecryptSecretConfig(sourceDatabaseConfigs map[string]interface{}, config map[string]interface{}) (string, error) {
	return "", nil
}

// Utlized to provide Data Flow Statistics components: database name in which the DFS resides and the index
// of the DataFlowStatistics table (argosId)
func GetDFSPathName() (string, string) {
	return "databasename", "argosId"
}

// GetDatabaseName - returns a name to be used by TrcDb.
func GetDatabaseName() string {
	return "tiercerondb"
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

func IsValidIP(ipaddr string) (bool, error) {
	return true, nil
}

func GetMachineID() string {
	netIP, err := trcnet.NetIpAddr(IsValidIP)
	if err != nil {
		return ""
	}
	return netIP
}

func GetPluginRestrictedMappings() map[string][][]string {
	return map[string][][]string{
		"trcsh-curator": {
			[]string{"-templateFilter=TrcVault/TrcshCurator", "-restricted=TrcshCurator", "-serviceFilter=config", "-indexFilter=config"},
			[]string{"-templateFilter=TrcVault/PluginTool", "-restricted=PluginTool", "-serviceFilter=config", "-indexFilter=config"},
		},
		"trcshqaw": {
			[]string{"-templateFilter=APIMConfig/APIMConfig", "-restricted=APIMConfig", "-serviceFilter=config", "-indexFilter=config"},
		},
		"trcshqk": {
			[]string{"-templateFilter=APIMConfig/APIMConfig", "-restricted=APIMConfig", "-serviceFilter=config", "-indexFilter=config"},
		},
		"trcsh-cursor-aw": {
			[]string{"-templateFilter=TrcVault/TrcshCursorAW", "-restricted=TrcshCursorAW", "-serviceFilter=config", "-indexFilter=config"},
		},
		"trcsh-cursor-k": {
			[]string{"-templateFilter=TrcVault/TrcshCursorK", "-restricted=TrcshCursorK", "-serviceFilter=config", "-indexFilter=config"},
		},
		"trc-vault-plugin": {
			[]string{"-templateFilter=FlumeDatabase/TierceronFlow", "-indexed=FlumeDatabase", "-serviceFilter=TierceronFlow", "-indexFilter=flowName"},
			[]string{"-templateFilter=TrcVault/Database", "-indexed=TrcVault", "-serviceFilter=Database", "-indexFilter=regionId"},
			[]string{"-templateFilter=TrcVault/Identity", "-restricted=Identity", "-serviceFilter=config", "-indexFilter=config"},
			[]string{"-templateFilter=TrcVault/VaultDatabase", "-restricted=VaultDatabase", "-serviceFilter=config", "-indexFilter=config"},
			[]string{"-templateFilter=TrcVault/SpiralDatabase", "-restricted=SpiralDatabase", "-serviceFilter=config", "-indexFilter=config"},
		},
		"trchelloworld": {
			[]string{"-templateFilter=Common/hello.crt,Common/hellokey.key,HelloProjectPlugin/HelloServicePlugin"},
		},
	}
}

func GetConfigPaths(pluginName string) []string {
	switch pluginName {
	// An example mutabilis plugin -- not really implemented as such.
	case "healthcheck":
		return []string{
			"Common/serviceclientcert.pem.mf.tmpl",
			"Common/servicecert.crt.mf.tmpl",
			"Common/servicekey.key.mf.tmpl",
			"/local_config/application",
			"/local_config/contrast",
			"/local_config/logback",
			"/local_config/newrelic",
		}
	default:
		return []string{}
	}
}

func GetSupportedCertIssuers() []string {
	return []string{"http://r3.i.lencr.org/"}
}

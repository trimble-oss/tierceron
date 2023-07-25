//go:build !tc
// +build !tc

package coreopts

import (
	"database/sql"
	"errors"
	"strings"
)

// Folder prefix for _seed, etc...
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

func GetSupportedTemplates(custom []string) []string {
	return []string{}
}

func GetSupportedEndpoints() []string {
	return []string{}
}

func GetLocalHost() string {
	return ""
}

func GetVaultHost() string {
	return ""
}

func GetVaultHostPort() string {
	return ""
}

// Begin old Active Sessions interface
func GetUserNameField() string {
	return ""
}

func GetUserCodeField() string {
	return ""
}

func ActiveSessions(db *sql.DB) ([]map[string]interface{}, error) {
	return nil, errors.New("Not implemented")
}

// End old Active Sessions interface

func FindIndexForService(project string, service string) (string, []string, string, error) {
	return "", nil, "", errors.New("Not implemented")
}

// Which tables synced..
func GetSyncedTables() []string {
	return []string{}
}

// Enrich plugin configs
func ProcessDeployPluginEnvConfig(pluginEnvConfig map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{}
}

func DecryptSecretConfig(tenantConfiguration map[string]interface{}, config map[string]interface{}) string {
	return ""
}

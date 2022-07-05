//go:build !tc
// +build !tc

package coreopts

import (
	"database/sql"
	"errors"

	sqle "github.com/dolthub/go-mysql-server/sql"
)

// Which tables synced..
func GetSyncedTables() []string {
	return []string{}
}

func GetIdColumnType(table string) sqle.Type {
	return sqle.Text
}

func GetFolderPrefix() string {
	return "trc"
}

func GetSupportedTemplates() []string {
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

func DecryptSecretConfig(tenantConfiguration map[string]interface{}, config map[string]interface{}) string {
	return ""
}

func ActiveSessions(db *sql.DB) ([]map[string]interface{}, error) {
	return nil, errors.New("Not implemented")
}

func GetUserNameField() string {
	return ""
}

func GetUserCodeField() string {
	return ""
}

func FindIndexForService(project string, service string) (string, error) {
	return "", errors.New("Not implemented")
}

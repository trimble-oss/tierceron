//go:build !tc
// +build !tc

package tcopts

//	"time"

const RFC_ISO_8601 = "2006-01-02 15:04:05 -0700 MST"

func CheckIncomingColumnName(col string) bool {
	return false
}

func CheckMysqlFileIncoming(secretColumns map[string]string, secretValue string, dbName string, tableName string) ([]byte, string, string, string, error) {
	return nil, "", "", "", nil
}

func CheckIncomingAliasColumnName(col string) bool {
	return false
}

func GetTrcDbUrl(data map[string]interface{}) string {
	return ""
}

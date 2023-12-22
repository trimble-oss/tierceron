//go:build !tc
// +build !tc

package tclibc

//	"time"

const RFC_ISO_8601 = ""

func CheckIncomingColumnName(col string) bool {
	return false
}

func CheckMysqlFileIncoming(secretColumns map[string]string, secretValue string, flowSourceAlias string, tableName string) ([]uint8, string, string, string, error) {
	return nil, "", "", "", nil
}

func CheckIncomingAliasColumnName(col string) bool {
	return false
}

func GetTrcDbUrl(data map[string]interface{}) string {
	return ""
}

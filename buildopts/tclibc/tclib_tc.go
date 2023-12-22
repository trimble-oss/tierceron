//go:build tc
// +build tc

package tclibc

import (
	tclibc "VaultConfig.TenantConfig/lib/libsqlc"
)

func CheckIncomingColumnName(col string) bool {
	return tclibc.CheckIncomingColumnName(col)
}

func CheckMysqlFileIncoming(secretColumns map[string]string, secretValue string, flowSourceAlias string, tableName string) (interface{}, string, string, string, error) {
	return tclibc.CheckMysqlFileIncoming(secretColumns, secretValue, flowSourceAlias, tableName)
}

func CheckIncomingAliasColumnName(col string) bool {
	return tclibc.CheckIncomingAliasColumnName(co;)
}

func GetTrcDbUrl(data map[string]interface{}) string {
	return ""
}

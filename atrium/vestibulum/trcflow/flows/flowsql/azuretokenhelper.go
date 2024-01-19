package flowsql

import (
	"strings"
)

func GetAzureTokenUpsert(azureTokenHandle map[string]interface{}, dbName string, tableName string, col1 string, col2 string) map[string]interface{} {
	if len(azureTokenHandle) != 6 { //A query was made not containing defaults yet.
		azureTokenHandle["accessKey"] = ""
		azureTokenHandle["permission"] = ""
		azureTokenHandle["keyExpiration"] = ""
		azureTokenHandle["lastModified"] = ""
	}

	azureTokenHandle = EscapeForQuery(azureTokenHandle)

	sqlstr := map[string]interface{}{
		"TrcQuery": "INSERT INTO " + dbName + `.` + tableName + `(` + col1 + `, ` + col2 + `, accessKey, permission, keyExpiration, lastModified) VALUES ('` +
			azureTokenHandle[col1].(string) + `','` +
			azureTokenHandle[col2].(string) + `','` +
			azureTokenHandle["accessKey"].(string) + `','` +
			azureTokenHandle["permission"].(string) + `','` +
			azureTokenHandle["keyExpiration"].(string) + `','` +
			azureTokenHandle["lastModified"].(string) + `')` +
			` ON DUPLICATE KEY UPDATE ` +
			`` + col1 + ` = VALUES(` + col1 + `), ` + col2 + ` = VALUES(` + col2 + `), accessKey = VALUES(accessKey), permission = VALUES(permission), keyExpiration = VALUES(keyExpiration), lastModified = VALUES(lastModified)`,
		"TrcChangeId":  []string{azureTokenHandle[col1].(string), azureTokenHandle[col2].(string)},
		"TrcChangeCol": []string{col1, col2},
	}

	return sqlstr
}

func EscapeForQuery(theMap map[string]interface{}) map[string]interface{} {
	for index, data := range theMap {
		if strings.HasSuffix(index, "Content") {
			continue
		}
		if _, ok := data.(string); ok {
			theMap[index] = strings.ReplaceAll(data.(string), "'", "\\'")
		}
	}
	return theMap
}

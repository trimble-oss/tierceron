package flowsql

import (
	"strconv"
	"strings"
)

const DataflowTestNameColumn = "flowName"
const DataflowTestIdColumn = "argosId"
const DataflowTestStateCodeColumn = "stateCode"

func GetDataFlowStatisticInsert(tenantId string, statisticData map[string]interface{}, dbName string, tableName string) map[string]interface{} {
	tenantId = strings.ReplaceAll(tenantId, "/", "")
	sqlstr := map[string]interface{}{
		"TrcQuery": `INSERT IGNORE INTO ` + dbName + `.` + tableName + `(` + DataflowTestNameColumn + `, ` + DataflowTestIdColumn + `, flowGroup, mode, stateCode, stateName, timeSplit, lastTestedDate) VALUES ('` +
			statisticData["flowName"].(string) + `','` + tenantId + `','` +
			statisticData["flowGroup"].(string) + `','` + strconv.Itoa(statisticData["mode"].(int)) +
			`','` + statisticData["stateCode"].(string) + `','` + statisticData["stateName"].(string) +
			`','` + statisticData["timeSplit"].(string) + `','` + statisticData["lastTestedDate"].(string) + `')` +
			` ON DUPLICATE KEY UPDATE ` +
			DataflowTestNameColumn + `= VALUES(` + DataflowTestNameColumn + `),` + DataflowTestIdColumn + `= VALUES(` + DataflowTestIdColumn + `),flowGroup = VALUES(flowGroup),mode = VALUES(mode),stateCode = VALUES(stateCode),stateName = VALUES(stateName),timeSplit = VALUES(timeSplit), lastTestedDate = VALUES(lastTestedDate)`,
		"TrcChangeId": []string{statisticData["flowName"].(string), tenantId, statisticData["stateCode"].(string)},
	}
	return sqlstr
}

func GetDataFlowStatisticLM(tenantId string, statisticData map[string]interface{}, dbName string, tableName string) map[string]interface{} {
	tenantId = strings.ReplaceAll(tenantId, "/", "")
	sqlstr := map[string]interface{}{
		"TrcQuery": `select lastModified from ` + dbName + `.` + tableName + `where ` + DataflowTestNameColumn + `=` + statisticData["flowName"].(string) + ` and ` +
			DataflowTestIdColumn + "=" + tenantId + ` and ` + DataflowTestStateCodeColumn + ` = ` + statisticData["stateCode"].(string),
	}
	return sqlstr
}

func GetDataFlowStatisticUpdate(tenantId string, statisticData map[string]interface{}, dbName string, tableName string) map[string]interface{} {
	tenantId = strings.ReplaceAll(tenantId, "/", "")
	sqlstr := map[string]interface{}{
		"TrcQuery": `INSERT IGNORE INTO ` + dbName + `.` + tableName + `(` + DataflowTestNameColumn + `, ` + DataflowTestIdColumn + `, flowGroup, mode, stateCode, stateName, timeSplit, lastTestedDate) VALUES ('` +
			statisticData["flowName"].(string) + `','` + tenantId + `','` +
			statisticData["flowGroup"].(string) + `','` + strconv.Itoa(statisticData["mode"].(int)) +
			`','` + statisticData["stateCode"].(string) + `','` + statisticData["stateName"].(string) +
			`','` + statisticData["timeSplit"].(string) + `','` + statisticData["lastTestedDate"].(string) + `')` +
			` ON DUPLICATE KEY UPDATE ` +
			DataflowTestNameColumn + `= VALUES(` + DataflowTestNameColumn + `),` + DataflowTestIdColumn + `= VALUES(` + DataflowTestIdColumn + `),flowGroup = VALUES(flowGroup),mode = VALUES(mode),stateCode = VALUES(stateCode),stateName = VALUES(stateName),timeSplit = VALUES(timeSplit), lastTestedDate = VALUES(lastTestedDate)`,
		"TrcChangeId": []string{statisticData["flowName"].(string), tenantId, statisticData["stateCode"].(string)},
	}
	return sqlstr
}

func DataFlowStatisticsArrayToMap(dfs []interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	m[DataflowTestNameColumn] = dfs[0]
	m[DataflowTestIdColumn] = dfs[1]
	m["flowGroup"] = dfs[2]
	m["mode"] = dfs[3]
	m["stateCode"] = dfs[4]
	m["stateName"] = dfs[5]
	m["timeSplit"] = dfs[6]
	m["lastTestedDate"] = dfs[7]
	m["lastModified"] = m["lastTestedDate"] //This is for lastModified comparison -> not used in table or queries
	return m
}

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
	} //Add trcChangedID
	return sqlstr
}

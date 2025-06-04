package flows

import (
	"errors"
	"strings"

	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"

	sqle "github.com/dolthub/go-mysql-server/sql"
)

const flowGroupName = "System"

var refresh = false
var endRefreshChan = make(chan bool, 1)

func getIndexColumnNames() []string {
	return []string{"argosId"}
}

func getFlowIndexComplex() (string, []string, string, error) {
	return "argosId", nil, "", nil
}

func getIndexedPathExt(engine interface{}, rowDataMap map[string]interface{}, indexColumnNames interface{}, databaseName string, tableName string, dbCallBack func(interface{}, map[string]interface{}) (string, []string, [][]interface{}, error)) (string, error) {
	return "", errors.New("could not find argossocii index")
}

func GetDataFlowUpdateTrigger(databaseName string, tableName string, iden1 string, iden2 string, iden3 string) string {
	return `CREATE TRIGGER trcUpdateTrigger_` + tableName + ` AFTER UPDATE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + iden1 + `,new.` + iden2 + `,new.` + iden3 + `,current_timestamp());` +
		` END;`
}

func GetDataFlowInsertTrigger(databaseName string, tableName string, iden1 string, iden2 string, iden3 string) string {
	return `CREATE TRIGGER trcInsertTrigger_` + tableName + ` AFTER INSERT ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + iden1 + `,new.` + iden2 + `,new.` + iden3 + `,current_timestamp());` +
		` END;`
}

func GetDataFlowDeleteTrigger(databaseName string, tableName string, iden1 string, iden2 string, iden3 string) string {
	return `CREATE TRIGGER trcDeleteTrigger_` + tableName + ` AFTER DELETE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (old.` + iden1 + `,old.` + iden2 + `,old.` + iden3 + `,current_timestamp());` +
		` END;`
}

func getSchema(tableName string) interface{} {
	return sqle.NewPrimaryKeySchema(sqle.Schema{
		{Name: "argosId", Type: sqle.Text, Source: tableName, PrimaryKey: true},
		{Name: "argosIdentitasNomen", Type: sqle.Text, Source: tableName},
		{Name: "argosProiectum", Type: sqle.Text, Source: tableName},
		{Name: "argosServitium", Type: sqle.Text, Source: tableName, PrimaryKey: true},
		{Name: "argosNotitia", Type: sqle.Text, Source: tableName},
	})
}

func getTableMapFromArray(dfs []interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	m["argosId"] = dfs[0]
	m["argosIdentitasNomen"] = dfs[1]
	m["argosProiectum"] = dfs[2]
	m["argosServitium"] = dfs[3]
	m["argosNotitia"] = dfs[4]
	return m
}

func getTableConfigurationInsertUpdate(data map[string]interface{}, dbName string, tableName string) map[string]interface{} {
	argosId := data["argosId"].(string)
	argosId = strings.ReplaceAll(argosId, "/", "")
	return getInsertUpdateById(argosId, data, dbName, tableName)
}

func getInsertUpdateById(argosId string, data map[string]interface{}, dbName string, tableName string) map[string]interface{} {
	argosId = strings.ReplaceAll(argosId, "/", "")
	sqlstr := map[string]interface{}{
		"TrcQuery": `INSERT IGNORE INTO ` + dbName + `.` + tableName + `(argosId, argosIdentitasNomen, argosProiectum, argosServitium, argosNotitia) VALUES ('` +
			argosId + `','` +
			data["argosIdentitasNomen"].(string) + `','` + data["argosIdentitasNomen"].(string) +
			`','` + data["argosProiectum"].(string) + `','` + data["argosProiectum"].(string) +
			`','` + data["argosServitium"].(string) + `','` + data["argosServitium"].(string) +
			`','` + data["argosNotitia"].(string) + `','` + data["argosNotitia"].(string) + `')` +
			` ON DUPLICATE KEY UPDATE ` +
			`argosId = VALUES(argosId),argosIdentitasNomen = VALUES(argosIdentitasNomen),argosProiectum = VALUES(argosProiectum),argosServitium = VALUES(argosServitium),argosNotitia = VALUES(argosNotitia)`,
		"TrcChangeId": []string{data["flowName"].(string), argosId, data["stateCode"].(string)},
	}
	return sqlstr
}
func GetProcessFlowDefinition() *flowcore.FlowDefinitionContext {
	return &flowcore.FlowDefinitionContext{
		GetTableConfigurationById:   nil, //not pulling from remote
		GetTableConfigurations:      nil, //not pulling from remote
		CreateTableTriggers:         nil, // use default provided by tierceron.
		GetTableMap:                 nil, //not pulling from remote
		GetTableFromMap:             nil, //not pulling from remote
		GetFilterFieldFromConfig:    nil, //not pushing remote
		GetTableMapFromArray:        getTableMapFromArray,
		GetTableConfigurationInsert: getTableConfigurationInsertUpdate,
		GetTableConfigurationUpdate: getTableConfigurationInsertUpdate,
		ApplyDependencies:           nil, //not pushing remote
		GetTableSchema:              getSchema,
		GetIndexedPathExt:           getIndexedPathExt,
		GetTableIndexColumnNames:    getIndexColumnNames,
		GetFlowIndexComplex:         getFlowIndexComplex,
	}
}

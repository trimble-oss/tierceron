package flows

import (
	"errors"

	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	"github.com/trimble-oss/tierceron/atrium/buildopts/flowcoreopts"
	"github.com/trimble-oss/tierceron/atrium/trcflow/core"
	trcflowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"

	dfssql "github.com/trimble-oss/tierceron/atrium/vestibulum/trcflow/flows/flowsql"

	sqle "github.com/dolthub/go-mysql-server/sql"
)

const flowGroupName = "Ninja"

var refresh = false
var endRefreshChan = make(chan bool, 1)

func getDataFlowStatisticsIndexColumnNames() []string {
	return []string{flowcoreopts.DataflowTestIdColumn, flowcoreopts.DataflowTestNameColumn, flowcoreopts.DataflowTestStateCodeColumn}
}

func GetDataflowStatIndexedPathExt(engine interface{}, rowDataMap map[string]interface{}, indexColumnNames interface{}, databaseName string, tableName string, dbCallBack func(interface{}, map[string]interface{}) (string, []string, [][]interface{}, error)) (string, error) {
	return "", errors.New("could not find argossocii index")
}

func GetDataFlowUpdateTrigger(databaseName string, tableName string, iden1 string, iden2 string, iden3 string) string {
	return `CREATE TRIGGER asUpdateTrigger AFTER UPDATE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + iden1 + `,new.` + iden2 + `,new.` + iden3 + `,current_timestamp());` +
		` END;`
}

func GetDataFlowInsertTrigger(databaseName string, tableName string, iden1 string, iden2 string, iden3 string) string {
	return `CREATE TRIGGER asInsertTrigger AFTER INSERT ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + iden1 + `,new.` + iden2 + `,new.` + iden3 + `,current_timestamp());` +
		` END;`
}

func GetDataFlowDeleteTrigger(databaseName string, tableName string, iden1 string, iden2 string, iden3 string) string {
	return `CREATE TRIGGER asDeleteTrigger AFTER DELETE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (old.` + iden1 + `,old.` + iden2 + `,old.` + iden3 + `,current_timestamp());` +
		` END;`
}

func getDataFlowStatisticsSchema(tableName string) interface{} {
	return sqle.NewPrimaryKeySchema(sqle.Schema{
		{Name: flowcoreopts.DataflowTestNameColumn, Type: sqle.Text, Source: tableName, PrimaryKey: true}, //composite key
		{Name: flowcoreopts.DataflowTestIdColumn, Type: sqle.Text, Source: tableName, PrimaryKey: true},
		{Name: "flowGroup", Type: sqle.Text, Source: tableName},
		{Name: "mode", Type: sqle.Text, Source: tableName},
		{Name: "stateCode", Type: sqle.Text, Source: tableName, PrimaryKey: true},
		{Name: "stateName", Type: sqle.Text, Source: tableName},
		{Name: "timeSplit", Type: sqle.Text, Source: tableName},
		{Name: "lastTestedDate", Type: sqle.Text, Source: tableName},
	})
}

func CreateTableTriggers(tfmContextI flowcore.FlowMachineContext, tfContextI flowcore.FlowContext) {
	tfmContext := tfmContextI.(*core.TrcFlowMachineContext)
	tfContext := tfContextI.(*core.TrcFlowContext)
	tfmContext.GetTableModifierLock().Lock()
	changeTableName := tfContext.Flow.TableName() + "_Changes"
	tfmContext.CallDBQuery(tfContext, map[string]interface{}{"TrcQuery": "DROP TABLE " + tfmContext.TierceronEngine.Database.Name() + "." + changeTableName}, nil, false, "DELETE", nil, "")
	changeTableErr := tfmContext.TierceronEngine.Database.CreateTable(tfmContext.TierceronEngine.Context, changeTableName, sqle.NewPrimaryKeySchema(sqle.Schema{
		{Name: flowcoreopts.DataflowTestNameColumn, Type: sqle.Text, Source: changeTableName, PrimaryKey: true},
		{Name: flowcoreopts.DataflowTestIdColumn, Type: sqle.Text, Source: changeTableName, PrimaryKey: true},
		{Name: flowcoreopts.DataflowTestStateCodeColumn, Type: sqle.Text, Source: changeTableName, PrimaryKey: true},
		{Name: "updateTime", Type: sqle.Timestamp, Source: changeTableName},
	}),
		trcflowcore.TableCollationIdGen(changeTableName),
	)
	if changeTableErr != nil {
		tfmContext.Log("Error creating dfs change table", changeTableErr)
	}
	tfmContext.CreateDataFlowTableTriggers(tfContext, flowcoreopts.DataflowTestNameColumn, flowcoreopts.DataflowTestIdColumn, flowcoreopts.DataflowTestStateCodeColumn, GetDataFlowInsertTrigger, GetDataFlowUpdateTrigger, GetDataFlowDeleteTrigger)
	tfmContext.GetTableModifierLock().Unlock()
}

func ProcessArgosSociiConfigurations(tfmContext flowcore.FlowMachineContext, tfContext flowcore.FlowContext) error {
	if tfContext.GetFlowDefinitionContext() == nil {
		flowDefinitionContext := &flowcore.FlowDefinitionContext{
			GetTableConfigurationById:   nil, //not pulling from remote
			GetTableConfigurations:      nil, //not pulling from remote
			CreateTableTriggers:         CreateTableTriggers,
			GetTableMap:                 nil, //not pulling from remote
			GetTableFromMap:             nil, //not pulling from remote
			GetFilterFieldFromConfig:    nil, //not pushing remote
			GetTableMapFromArray:        dfssql.GetDataFlowStatisticsFromArray,
			GetTableConfigurationInsert: dfssql.GetDataFlowStatisticInsert,
			GetTableConfigurationUpdate: dfssql.GetDataFlowStatisticUpdate,
			ApplyDependencies:           nil, //not pushing remote
			GetTableSchema:              getDataFlowStatisticsSchema,
			GetIndexedPathExt:           GetDataflowStatIndexedPathExt,
			GetTableIndexColumnNames:    getDataFlowStatisticsIndexColumnNames,
		}
		tfContext.SetFlowDefinitionContext(flowDefinitionContext)
	}

	return flowcore.ProcessTableConfigurations(tfmContext, tfContext)
}

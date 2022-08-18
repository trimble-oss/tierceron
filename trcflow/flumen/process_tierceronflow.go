package flumen

import (
	"errors"
	flowcore "tierceron/trcflow/core"
	"time"

	sqle "github.com/dolthub/go-mysql-server/sql"
)

const tierceronFlowIdColumnName = "flowName"
const tierceronFlowConfigurationTableName = "TierceronFlow"

var flowInit = true

func UpdateTierceronFlowState(newState string, flowName string) string {
	//rows := tfmContext.CallDBQuery(tfContext, "update " + tierceronFlowConfigurationTableName + "." + flowopts.GetFlowDatabaseName() + "set state=" + newState +" where flowName='TenantConfiguration' "+, nil, false, "SELECT", nil, "")
	return ""
}

func GetTierceronFlowConfigurationIndexedPathExt(engine interface{}, rowDataMap map[string]interface{}, vaultIndexColumnName string, databaseName string, tableName string, dbCallBack func(interface{}, string) (string, []string, [][]interface{}, error)) (string, error) {
	indexName, idValue := "", ""
	if tierceronFlowName, ok := rowDataMap[vaultIndexColumnName].(string); ok {
		indexName = vaultIndexColumnName
		idValue = tierceronFlowName
	} else {
		return "", errors.New("flowName not found for TierceronFlow: " + rowDataMap[vaultIndexColumnName].(string))
	}
	return "/" + indexName + "/" + idValue, nil
}

func GetTierceronTableNames() []string {
	return []string{tierceronFlowConfigurationTableName}
}

func getTierceronFlowSchema(tableName string) sqle.PrimaryKeySchema {
	return sqle.NewPrimaryKeySchema(sqle.Schema{
		{Name: tierceronFlowIdColumnName, Type: sqle.Text, Source: tableName, PrimaryKey: true},
		{Name: "state", Type: sqle.Int64, Source: tableName},
		{Name: "syncMode", Type: sqle.Text, Source: tableName},
		{Name: "lastModified", Type: sqle.Timestamp, Source: tableName},
	})
}

//cancel contex through all the flows to cancel and stop all the sync cycles.

func arrayToTierceronFlow(arr []interface{}) map[string]interface{} {
	tfFlow := make(map[string]interface{})
	if len(arr) == 4 {
		tfFlow[tierceronFlowIdColumnName] = arr[0]
		tfFlow["state"] = arr[1]
		tfFlow["syncMode"] = arr[2]
		tfFlow["lastModified"] = arr[3]
	}
	return tfFlow
}

func tierceronFlowImport(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) ([]map[string]interface{}, error) {
	flowControllerMap := tfContext.RemoteDataSource["flowControllerMap"].(map[string]chan int64)
	rows := tfmContext.CallDBQuery(tfContext, "select * from "+tfContext.FlowSourceAlias+"."+string(tfContext.Flow), nil, false, "SELECT", nil, "")
	for _, value := range rows {
		tfFlow := arrayToTierceronFlow(value)
		if len(tfFlow) == 4 {
			stateChannel := flowControllerMap[tfFlow[tierceronFlowIdColumnName].(string)]
			stateMsg := tfFlow["state"].(int64)
			stateChannel <- stateMsg
		}
	}

	if flowInit { //Used to signal other flows to begin, now that states have been loaded on init
		<-tfContext.RemoteDataSource["vaultImportChannel"].(chan bool)
		flowInit = false
	}

	return nil, nil
}

//Only pull from vault on init
//Listen to a change channel ->
func ProcessTierceronFlows(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) error {
	tfmContext.AddTableSchema(getTierceronFlowSchema(tfContext.Flow.TableName()), tfContext.Flow.TableName())
	tfmContext.CreateTableTriggers(tfContext, tierceronFlowIdColumnName)

	//cancelCtx, _ := context.WithCancel(context.Background())
	tfmContext.SyncTableCycle(tfContext, tierceronFlowIdColumnName, tierceronFlowIdColumnName, "", GetTierceronFlowConfigurationIndexedPathExt, nil)
	sqlIngestInterval := tfContext.RemoteDataSource["dbingestinterval"].(time.Duration)
	if sqlIngestInterval > 0 {
		// Implement pull from remote data source.
		// Only pull if ingest interval is set to > 0 value.
		afterTime := time.Duration(0)
		for {
			select {
			case <-time.After(time.Millisecond * afterTime):
				afterTime = 5000
				tfmContext.Log("Tierceron Flows... checking for changes.", nil)
				// Periodically checks the table for updates and send out state changes to flows.
				_, err := tierceronFlowImport(tfmContext, tfContext)
				if err != nil {
					tfmContext.Log("Error grabbing configurations for tierceron flows", err)
					continue
				}
			}
		}
	}
	return nil
}

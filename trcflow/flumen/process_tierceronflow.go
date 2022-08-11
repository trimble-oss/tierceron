package flumen

import (
	"errors"
	flowcore "tierceron/trcflow/core"
	"time"

	sqle "github.com/dolthub/go-mysql-server/sql"
)

const tierceronFlowIdColumnName = "flowName"
const tierceronFlowConfigurationTableName = "TierceronFlow"

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

func tierceronFlowImport(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) ([]map[string]interface{}, error) {

	//go to vault, grab latest settings, see which is the newest using lastModified -> update

	//tfmContext.CallDBQuery(tfContext, "INSERT IGNORE INTO TierceronFlow(flowName, state, syncMode, lastModified) VALUES ('TenantConfiguration', '0', '0', current_timestamp());", nil, true, "INSERT", nil, "")
	return nil, nil
}

//Only pull from vault on init
//Listen to a change channel ->
func ProcessTierceronFlows(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) error {
	tfmContext.AddTableSchema(getTierceronFlowSchema(tfContext.Flow.TableName()), tfContext.Flow.TableName())
	tfmContext.CreateTableTriggers(tfContext, tierceronFlowIdColumnName)

	tfmContext.SyncTableCycle(tfContext, tierceronFlowIdColumnName, tierceronFlowIdColumnName, "", GetTierceronFlowConfigurationIndexedPathExt, nil)
	sqlIngestInterval := tfContext.RemoteDataSource["dbingestinterval"].(time.Duration)
	if sqlIngestInterval > 0 {
		// Implement pull from remote data source.
		// Only pull if ingest interval is set to > 0 value.
		afterTime := time.Duration(0)
		for {
			select {
			case <-time.After(time.Millisecond * afterTime):
				afterTime = sqlIngestInterval
				tfmContext.Log("Tierceron Flows... checking for changes.", nil)
				// 3. Retrieve tenant configurations from tenant config table.
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

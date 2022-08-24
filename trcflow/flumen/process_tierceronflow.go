package flumen

import (
	"context"
	"errors"
	"sync"

	flowcore "tierceron/trcflow/core"

	flowcorehelper "tierceron/trcflow/core/flowcorehelper"
	"time"

	sqle "github.com/dolthub/go-mysql-server/sql"
)

const tierceronFlowIdColumnName = "flowName"

var tableChangeAlertChan = make(chan string, 1)
var tableChangeLock = &sync.Mutex{}
var flowInit = true

func GetTierceronFlowConfigurationIndexedPathExt(engine interface{}, rowDataMap map[string]interface{}, vaultIndexColumnName string, databaseName string, tableName string, dbCallBack func(interface{}, string) (string, []string, [][]interface{}, error)) (string, error) {
	indexName, idValue := "", ""
	if tierceronFlowName, ok := rowDataMap[vaultIndexColumnName].(string); ok {
		indexName = vaultIndexColumnName
		idValue = tierceronFlowName
		tableChangeAlertChan <- tierceronFlowName

	} else {
		return "", errors.New("flowName not found for TierceronFlow: " + vaultIndexColumnName)
	}
	return "/" + indexName + "/" + idValue, nil
}

func GetTierceronTableNames() []string {
	return []string{flowcorehelper.TierceronFlowConfigurationTableName}
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

func sendUpdates(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext, flowControllerMap map[string]chan flowcorehelper.CurrentFlowState, tierceronFlowName string) {
	tableChangeLock.Lock()
	var rows [][]interface{}
	if tierceronFlowName != "" {
		rows = tfmContext.CallDBQuery(tfContext, "select * from "+tfContext.FlowSourceAlias+"."+string(tfContext.Flow)+" WHERE "+tierceronFlowIdColumnName+"='"+tierceronFlowName+"'", nil, false, "SELECT", nil, "")
	} else {
		rows = tfmContext.CallDBQuery(tfContext, "select * from "+tfContext.FlowSourceAlias+"."+string(tfContext.Flow), nil, false, "SELECT", nil, "")
	}
	for _, value := range rows {
		tfFlow := arrayToTierceronFlow(value)
		if len(tfFlow) == 4 {
			stateChannel := flowControllerMap[tfFlow[tierceronFlowIdColumnName].(string)]
			if stateChannel == nil {
				tfmContext.Log("Tierceron Flow could not find the flow:"+tfFlow[tierceronFlowIdColumnName].(string), errors.New("State channel for flow controller was nil."))
				continue
			}
			if stateMsg, ok := tfFlow["state"].(int64); ok {
				if syncModeMsg, ok := tfFlow["syncMode"].(string); ok {
					go func() {
						stateChannel <- flowcorehelper.CurrentFlowState{State: stateMsg, SyncMode: syncModeMsg}
					}()
				}
			}
		}
	}
	tableChangeLock.Unlock()
}

func tierceronFlowImport(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) ([]map[string]interface{}, error) {
	if flowControllerMap, ok := tfContext.RemoteDataSource["flowStateControllerMap"].(map[string]chan flowcorehelper.CurrentFlowState); ok {
		if flowControllerMap == nil {
			return nil, errors.New("Channel map for flow controller was nil.")
		}

		sendUpdates(tfmContext, tfContext, flowControllerMap, "")

		if flowInit { //Sending off listener for state updates
			go func() {
				for {
					select {
					case tierceronFlowName, ok := <-tableChangeAlertChan:
						if ok {
							sendUpdates(tfmContext, tfContext, flowControllerMap, tierceronFlowName)
						}
					}
				}
			}()
		}
	} else {
		return nil, errors.New("Flow controller map is the wrong type.")
	}

	if flowInit { //Used to signal other flows to begin, now that states have been loaded on init
		if initAlertChan, ok := tfContext.RemoteDataSource["flowStateInitAlert"].(chan bool); ok {
			if initAlertChan == nil {
				return nil, errors.New("Alert channel for flow controller was nil.")
			}
			select {
			case initAlertChan <- flowInit:
				flowInit = false
			default:
			}
		} else {
			return nil, errors.New("Alert channel for flow controller is wrong type.")
		}

		if flowStateReceiverMap, ok := tfContext.RemoteDataSource["flowStateReceiverMap"].(map[string]chan flowcorehelper.FlowStateUpdate); ok {
			if flowStateReceiverMap == nil {
				return nil, errors.New("Receiver map channel for flow controller was nil.")
			}
			for _, receiver := range flowStateReceiverMap { //Receiver is used to update the flow state for shutdowns & inits from other flows
				go func(currentReceiver chan flowcorehelper.FlowStateUpdate) {
					for {
						select {
						case x, ok := <-currentReceiver:
							if ok {
								tfmContext.CallDBQuery(tfContext, flowcorehelper.UpdateTierceronFlowState(x.FlowName, x.StateUpdate), nil, true, "UPDATE", nil, "")
							}
						}
					}
				}(receiver)
			}
			return nil, nil
		} else {
			return nil, errors.New("Receiver map for flow controller is wrong type.")
		}
	}

	return nil, nil
}

func getRestartTrigger(databaseName string, tableName string, idColumnName string) string {
	return `CREATE TRIGGER tcRestartTrigger AFTER UPDATE ON ` + databaseName + `.` + tableName +
		`UPDATE ` + tableName + ` SET state=1 WHERE flowName=NEW.flowName;`
}

//Only pull from vault on init
//Listen to a change channel ->
func ProcessTierceronFlows(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) error {
	tfmContext.AddTableSchema(getTierceronFlowSchema(tfContext.Flow.TableName()), tfContext.Flow.TableName())
	tfmContext.CreateTableTriggers(tfContext, tierceronFlowIdColumnName)

	var resTrigger sqle.TriggerDefinition
	resTrigger.Name = "tcRestartTrigger_" + tfContext.Flow.TableName()
	resTrigger.CreateStatement = getRestartTrigger(tfmContext.TierceronEngine.Database.Name(), tfContext.Flow.TableName(), tierceronFlowIdColumnName)
	//tfmContext.TierceronEngine.Database.CreateTrigger(tfmContext.TierceronEngine.Context, resTrigger)

	cancelCtx, _ := context.WithCancel(context.Background())
	tfmContext.SyncTableCycle(tfContext, tierceronFlowIdColumnName, tierceronFlowIdColumnName, "", GetTierceronFlowConfigurationIndexedPathExt, nil, cancelCtx, false)
	sqlIngestInterval := tfContext.RemoteDataSource["dbingestinterval"].(time.Duration)
	if sqlIngestInterval > 0 {
		// Implement pull from remote data source.
		// Only pull if ingest interval is set to > 0 value.
		afterTime := time.Duration(0)
		for {
			select {
			case <-time.After(time.Millisecond * afterTime):
				afterTime = sqlIngestInterval
				tfmContext.Log("Tierceron Flows is running and checking for changes.", nil)
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

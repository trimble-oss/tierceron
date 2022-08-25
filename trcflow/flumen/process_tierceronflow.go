package flumen

import (
	"context"
	"errors"
	"sync"

	flowcore "tierceron/trcflow/core"

	flowcorehelper "tierceron/trcflow/core/flowcorehelper"
	"time"

	sqle "github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/expression"
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
	stateDefault, _ := sqle.NewColumnDefaultValue(expression.NewLiteral(0, sqle.Int64), sqle.Int64, true, false)
	syncModeDefault, _ := sqle.NewColumnDefaultValue(expression.NewLiteral("0", sqle.Text), sqle.Text, true, false)
	syncFilterDefault, _ := sqle.NewColumnDefaultValue(expression.NewLiteral("", sqle.Text), sqle.Text, true, false)
	timestampDefault, _ := sqle.NewColumnDefaultValue(expression.NewLiteral(time.Now().UTC(), sqle.Timestamp), sqle.Timestamp, true, false)
	return sqle.NewPrimaryKeySchema(sqle.Schema{
		{Name: tierceronFlowIdColumnName, Type: sqle.Text, Source: tableName, PrimaryKey: true},
		{Name: "state", Type: sqle.Int64, Source: tableName, Default: stateDefault},
		{Name: "syncMode", Type: sqle.Text, Source: tableName, Default: syncModeDefault},
		{Name: "syncFilter", Type: sqle.Text, Source: tableName, Default: syncFilterDefault},
		{Name: "lastModified", Type: sqle.Timestamp, Source: tableName, Default: timestampDefault},
	})
}

//cancel contex through all the flows to cancel and stop all the sync cycles.

func arrayToTierceronFlow(arr []interface{}) map[string]interface{} {
	tfFlow := make(map[string]interface{})
	if len(arr) == 5 {
		tfFlow[tierceronFlowIdColumnName] = arr[0]
		tfFlow["state"] = arr[1]
		tfFlow["syncMode"] = arr[2]
		tfFlow["syncFilter"] = arr[3]
		tfFlow["lastModified"] = arr[4]
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
		if flowId, ok := tfFlow[tierceronFlowIdColumnName].(string); ok {
			stateChannel := flowControllerMap[flowId]
			if stateChannel == nil {
				tfmContext.Log("Tierceron Flow could not find the flow:"+tfFlow[tierceronFlowIdColumnName].(string), errors.New("State channel for flow controller was nil."))
				continue
			}
			if stateMsg, ok := tfFlow["state"].(int64); ok {
				if syncModeMsg, ok := tfFlow["syncMode"].(string); ok {
					if syncFilterMsg, ok := tfFlow["syncFilter"].(string); ok {
						go func(sc chan flowcorehelper.CurrentFlowState, stateMessage int64, syncModeMessage string, syncFilterMessage string) {
							sc <- flowcorehelper.CurrentFlowState{State: stateMessage, SyncMode: syncModeMessage, SyncFilter: syncFilterMessage}
						}(stateChannel, stateMsg, syncModeMsg, syncFilterMsg)
					}
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
			go func(tfmc *flowcore.TrcFlowMachineContext, tfc *flowcore.TrcFlowContext, fcmap map[string]chan flowcorehelper.CurrentFlowState) {
				for {
					select {
					case tierceronFlowName, ok := <-tableChangeAlertChan:
						if ok {
							sendUpdates(tfmc, tfc, fcmap, tierceronFlowName)
						}
					}
				}
			}(tfmContext, tfContext, flowControllerMap)
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
				go func(currentReceiver chan flowcorehelper.FlowStateUpdate, tfmc *flowcore.TrcFlowMachineContext) {
					for {
						select {
						case x, ok := <-currentReceiver:
							if ok {
								tfmc.CallDBQuery(tfContext, flowcorehelper.UpdateTierceronFlowState(x.FlowName, x.StateUpdate, x.SyncFilter), nil, true, "UPDATE", nil, "")
							}
						}
					}
				}(receiver, tfmContext)
			}
			return nil, nil
		} else {
			return nil, errors.New("Receiver map for flow controller is wrong type.")
		}
	}

	return nil, nil
}

//Only pull from vault on init
//Listen to a change channel ->
func ProcessTierceronFlows(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) error {
	tfmContext.AddTableSchema(getTierceronFlowSchema(tfContext.Flow.TableName()), tfContext.Flow.TableName())
	tfmContext.CreateTableTriggers(tfContext, tierceronFlowIdColumnName)

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
				afterTime = sqlIngestInterval * 2
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

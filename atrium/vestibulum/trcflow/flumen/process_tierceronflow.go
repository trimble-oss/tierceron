package flumen

import (
	"errors"

	flowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"

	"time"

	flowcorehelper "github.com/trimble-oss/tierceron/atrium/trcflow/core/flowcorehelper"

	sqle "github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/expression"
)

const tierceronFlowIdColumnName = "flowName"

func GetTierceronFlowIdColName() string {
	return tierceronFlowIdColumnName
}

func GetTierceronFlowConfigurationIndexedPathExt(engine interface{}, rowDataMap map[string]interface{}, indexColumnNameInterface interface{}, databaseName string, tableName string, dbCallBack func(interface{}, map[string]interface{}) (string, []string, [][]interface{}, error)) (string, error) {
	indexName, idValue := "", ""
	if indexColumnNameSlice, iOk := indexColumnNameInterface.([]string); iOk && len(indexColumnNameSlice) == 1 { // 1 ID
		if tierceronFlowName, ok := rowDataMap[indexColumnNameSlice[0]].(string); ok {
			indexName = indexColumnNameSlice[0]
			idValue = tierceronFlowName

		} else {
			return "", errors.New("flowName not found for TierceronFlow: " + indexColumnNameSlice[0])
		}
		return "/" + indexName + "/" + idValue, nil
	}
	return "", errors.New("Too many columnIDs for incoming TierceronFlow change")
}

func GetTierceronTableNames() []string {
	return []string{flowcorehelper.TierceronFlowConfigurationTableName}
}

func getTierceronFlowSchema(tableName string) sqle.PrimaryKeySchema {
	stateDefault, _ := sqle.NewColumnDefaultValue(expression.NewLiteral(0, sqle.Int64), sqle.Int64, true, false, false)
	syncModeDefault, _ := sqle.NewColumnDefaultValue(expression.NewLiteral("nosync", sqle.Text), sqle.Text, true, false, false)
	syncFilterDefault, _ := sqle.NewColumnDefaultValue(expression.NewLiteral("", sqle.Text), sqle.Text, true, false, false)
	timestampDefault, _ := sqle.NewColumnDefaultValue(expression.NewLiteral(time.Now().UTC(), sqle.Timestamp), sqle.Timestamp, true, false, false)
	return sqle.NewPrimaryKeySchema(sqle.Schema{
		{Name: tierceronFlowIdColumnName, Type: sqle.Text, Source: tableName, PrimaryKey: true},
		{Name: "state", Type: sqle.Int64, Source: tableName, Default: stateDefault},
		{Name: "syncMode", Type: sqle.Text, Source: tableName, Default: syncModeDefault},
		{Name: "syncFilter", Type: sqle.Text, Source: tableName, Default: syncFilterDefault},
		{Name: "flowAlias", Type: sqle.Text, Source: tableName, Default: syncFilterDefault},
		{Name: "lastModified", Type: sqle.Timestamp, Source: tableName, Default: timestampDefault},
	})
}

//cancel contex through all the flows to cancel and stop all the sync cycles.

func arrayToTierceronFlow(arr []interface{}) map[string]interface{} {
	tfFlow := make(map[string]interface{})
	if len(arr) == 6 {
		tfFlow[tierceronFlowIdColumnName] = arr[0]
		tfFlow["state"] = arr[1]
		tfFlow["syncMode"] = arr[2]
		tfFlow["syncFilter"] = arr[3]
		tfFlow["flowAlias"] = arr[4]
		tfFlow["lastModified"] = arr[5]
	}
	return tfFlow
}

func sendUpdates(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext, flowControllerMap map[string]chan flowcorehelper.CurrentFlowState, tierceronFlowName string) {
	var rows [][]interface{}
	if tierceronFlowName != "" {
		rows = tfmContext.CallDBQuery(tfContext, map[string]interface{}{"TrcQuery": "select * from " + tfContext.FlowSourceAlias + "." + string(tfContext.Flow) + " WHERE " + tierceronFlowIdColumnName + "='" + tierceronFlowName + "'"}, nil, false, "SELECT", nil, "")
	} else {
		rows = tfmContext.CallDBQuery(tfContext, map[string]interface{}{"TrcQuery": "select * from " + tfContext.FlowSourceAlias + "." + string(tfContext.Flow)}, nil, false, "SELECT", nil, "")
	}
	for _, value := range rows {
		tfFlow := arrayToTierceronFlow(value)
		if flowId, ok := tfFlow[tierceronFlowIdColumnName].(string); ok {
			tfmContext.FlowControllerUpdateLock.Lock()
			stateChannel := flowControllerMap[flowId]
			tfmContext.FlowControllerUpdateLock.Unlock()
			if stateChannel == nil {
				tfmContext.Log("Tierceron Flow could not find the flow:"+tfFlow[tierceronFlowIdColumnName].(string), errors.New("State channel for flow controller was nil."))
				continue
			}
			if stateMsg, ok := tfFlow["state"].(int64); ok {
				if syncModeMsg, ok := tfFlow["syncMode"].(string); ok {
					if syncFilterMsg, ok := tfFlow["syncFilter"].(string); ok {
						if flowAliasMsg, ok := tfFlow["flowAlias"].(string); ok {
							go func(sc chan flowcorehelper.CurrentFlowState, stateMessage int64, syncModeMessage string, syncFilterMessage string, fId string, flowAlias string) {
								tfmContext.Log("Queuing state change: "+fId, nil)
								sc <- flowcorehelper.CurrentFlowState{State: stateMessage, SyncMode: syncModeMessage, SyncFilter: syncFilterMessage, FlowAlias: flowAlias}
							}(stateChannel, stateMsg, syncModeMsg, syncFilterMsg, flowId, flowAliasMsg)
						}
					}
				}
			}
		}
	}
}

func tierceronFlowImport(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) ([]map[string]interface{}, error) {
	if flowControllerMap, ok := tfContext.RemoteDataSource["flowStateControllerMap"].(map[string]chan flowcorehelper.CurrentFlowState); ok {
		if flowControllerMap == nil {
			return nil, errors.New("Channel map for flow controller was nil.")
		}

		sendUpdates(tfmContext, tfContext, flowControllerMap, "")

		if tfmContext.FlowControllerInit { //Sending off listener for state updates
			go func(tfmc *flowcore.TrcFlowMachineContext, tfc *flowcore.TrcFlowContext, fcmap map[string]chan flowcorehelper.CurrentFlowState) {
				for {
					select {
					case tierceronFlowName, ok := <-tfmContext.FlowControllerUpdateAlert:
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

	if tfmContext.FlowControllerInit { //Used to signal other flows to begin, now that states have been loaded on init
		if initAlertChan, ok := tfContext.RemoteDataSource["flowStateInitAlert"].(chan bool); ok {
			if initAlertChan == nil {
				return nil, errors.New("Alert channel for flow controller was nil.")
			}
			select {
			case initAlertChan <- tfmContext.FlowControllerInit:
				tfmContext.FlowControllerInit = false
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
								tfmc.CallDBQuery(tfContext, flowcorehelper.UpdateTierceronFlowState(x.FlowName, x.StateUpdate, x.SyncFilter, x.SyncMode, x.FlowAlias), nil, true, "UPDATE", []flowcore.FlowNameType{flowcore.FlowNameType(flowcorehelper.TierceronFlowConfigurationTableName)}, "")
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

// Only pull from vault on init
// Listen to a change channel ->
func ProcessTierceronFlows(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) error {
	tfmContext.AddTableSchema(getTierceronFlowSchema(tfContext.Flow.TableName()), tfContext)
	tfmContext.CreateTableTriggers(tfContext, tierceronFlowIdColumnName)

	tfmContext.SyncTableCycle(tfContext, tierceronFlowIdColumnName, []string{tierceronFlowIdColumnName}, GetTierceronFlowConfigurationIndexedPathExt, nil, false)
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

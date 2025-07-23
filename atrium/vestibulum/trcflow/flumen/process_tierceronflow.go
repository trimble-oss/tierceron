package flumen

import (
	"errors"
	"strings"

	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	trcflowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"

	"time"

	flowcorehelper "github.com/trimble-oss/tierceron/atrium/trcflow/core/flowcorehelper"

	sqle "github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/expression"
)

const tierceronFlowIdColumnName = "flowName"

func GetTierceronFlowIdColName() string {
	return tierceronFlowIdColumnName
}

func GetTierceronFlowConfigurationIndexedPathExt(engine any, rowDataMap map[string]any, indexColumnNameInterface any, databaseName string, tableName string, dbCallBack func(any, map[string]any) (string, []string, [][]any, error)) (string, error) {
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
	return []string{flowcore.TierceronControllerFlow.TableName()}
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

func arrayToTierceronFlow(arr []any) map[string]any {
	tfFlow := make(map[string]any)
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

func sendUpdates(tfmContext *trcflowcore.TrcFlowMachineContext, tfContext *trcflowcore.TrcFlowContext, flowControllerMap map[string]chan flowcore.CurrentFlowState, tierceronFlowName string) {
	var rows [][]any
	if tierceronFlowName != "" {
		rows, _ = tfmContext.CallDBQuery(tfContext, map[string]any{"TrcQuery": "select * from " + tfContext.FlowHeader.SourceAlias + "." + string(tfContext.FlowHeader.Name) + " WHERE " + tierceronFlowIdColumnName + "='" + tierceronFlowName + "'"}, nil, false, "SELECT", nil, "")
	} else {
		rows, _ = tfmContext.CallDBQuery(tfContext, map[string]any{"TrcQuery": "select * from " + tfContext.FlowHeader.SourceAlias + "." + string(tfContext.FlowHeader.Name)}, nil, false, "SELECT", nil, "")
	}
	for _, value := range rows {
		tfFlow := arrayToTierceronFlow(value)
		if flowId, ok := tfFlow[tierceronFlowIdColumnName].(string); ok {
			tfmContext.FlowControllerUpdateLock.Lock()
			stateChannel := flowControllerMap[flowId]
			tfmContext.FlowControllerUpdateLock.Unlock()
			theFlow := tfmContext.GetFlowContext(flowcore.FlowNameType(flowId))
			if stateChannel == nil {
				continue
			}
			var stateMsg int64
			var stateChanged, syncModeChanged, syncFilterChanged bool
			var syncModeMsg, syncFilterMsg, flowAliasMsg string
			var ok bool
			if stateMsg, ok = tfFlow["state"].(int64); ok {
				if theFlow != nil {
					stateChanged = theFlow.GetFlowStateState() != stateMsg
				}
			}
			if syncModeMsg, ok = tfFlow["syncMode"].(string); ok {
				if theFlow != nil {
					syncModeChanged = TrimFlowColumnForCompare(theFlow.GetFlowSyncMode()) != TrimFlowColumnForCompare(syncModeMsg)
					if syncModeChanged {
						tfContext.FlowState.SyncMode = syncModeMsg
					}
				}
			}
			if syncFilterMsg, ok = tfFlow["syncFilter"].(string); ok {
				if theFlow != nil {
					syncFilterChanged = TrimFlowColumnForCompare(theFlow.GetFlowStateSyncFilterRaw()) != TrimFlowColumnForCompare(syncFilterMsg)
					if syncFilterChanged {
						tfContext.FlowState.SyncFilter = syncFilterMsg
					}
				}
			}
			if flowAliasMsg, ok = tfFlow["flowAlias"].(string); !ok {
				flowAliasMsg = ""
			}
			if tfmContext.FlowControllerInit || stateChanged || syncModeChanged || syncFilterChanged {
				go func(sc chan flowcore.CurrentFlowState, stateMessage int64, syncModeMessage string, syncFilterMessage string, fId string, flowAlias string) {
					tfmContext.Log("Queuing state change: "+fId, nil)
					sc <- flowcorehelper.CurrentFlowState{State: stateMessage, SyncMode: syncModeMessage, SyncFilter: syncFilterMessage, FlowAlias: flowAlias}
				}(stateChannel, stateMsg, syncModeMsg, syncFilterMsg, flowId, flowAliasMsg)
			}
		}
	}
}

func TrimFlowColumnForCompare(flowSyncFilter string) string {
	flowSyncFilterForCompare := strings.ToLower(flowSyncFilter)
	flowSyncFilterForCompare = strings.Trim(flowSyncFilterForCompare, "'")
	flowSyncFilterForCompare = strings.Replace(flowSyncFilterForCompare, "nosync", "", 1)
	flowSyncFilterForCompare = strings.Replace(flowSyncFilterForCompare, "n/a", "", 1)
	return flowSyncFilterForCompare
}

func tierceronFlowImport(tfmContext *trcflowcore.TrcFlowMachineContext, tfContext *trcflowcore.TrcFlowContext) ([]map[string]any, error) {
	if flowControllerMap, ok := tfContext.RemoteDataSource["flowStateControllerMap"].(map[string]chan flowcore.CurrentFlowState); ok {
		if flowControllerMap == nil {
			return nil, errors.New("channel map for flow controller was nil")
		}

		sendUpdates(tfmContext, tfContext, flowControllerMap, "")

		if tfmContext.FlowControllerInit { //Sending off listener for state updates
			go func(tfmc *trcflowcore.TrcFlowMachineContext, tfc *trcflowcore.TrcFlowContext, fcmap map[string]chan flowcore.CurrentFlowState) {
				for tierceronFlowName := range tfmc.FlowControllerUpdateAlert {
					sendUpdates(tfmc, tfc, fcmap, tierceronFlowName)
				}
			}(tfmContext, tfContext, flowControllerMap)
		}
	} else {
		return nil, errors.New("flow controller map is the wrong type")
	}

	if tfmContext.FlowControllerInit { //Used to signal other flows to begin, now that states have been loaded on init
		if initAlertChan, ok := tfContext.RemoteDataSource["flowStateInitAlert"].(chan bool); ok {
			if initAlertChan == nil {
				return nil, errors.New("alert channel for flow controller was nil")
			}
			select {
			case initAlertChan <- tfmContext.FlowControllerInit:
				tfmContext.FlowControllerInit = false
			default:
			}
		} else {
			return nil, errors.New("alert channel for flow controller is wrong type")
		}

		if flowStateReceiverMap, ok := tfContext.RemoteDataSource["flowStateReceiverMap"].(map[string]chan flowcore.FlowStateUpdate); ok {
			if flowStateReceiverMap == nil {
				return nil, errors.New("receiver map channel for flow controller was nil")
			}
			for _, receiver := range flowStateReceiverMap { //Receiver is used to update the flow state for shutdowns & inits from other flows
				go func(currentReceiver chan flowcore.FlowStateUpdate, tfmc *trcflowcore.TrcFlowMachineContext) {
					for xi := range currentReceiver {
						x := xi.(flowcorehelper.FlowStateUpdate)
						tfmc.CallDBQuery(tfContext, flowcorehelper.UpdateTierceronFlowState(tfmContext, x.FlowName, x.StateUpdate, x.SyncFilter, x.SyncMode, x.FlowAlias), nil, true, "UPDATE", []flowcore.FlowNameType{flowcore.TierceronControllerFlow.Name}, "")
					}
				}(receiver, tfmContext)
			}
			return nil, nil
		} else {
			return nil, errors.New("receiver map for flow controller is wrong type")
		}
	}

	return nil, nil
}

// Only pull from vault on init
// Listen to a change channel ->
func ProcessTierceronFlows(tfmContext *trcflowcore.TrcFlowMachineContext, tfContext *trcflowcore.TrcFlowContext) error {
	tfmContext.AddTableSchema(getTierceronFlowSchema(tfContext.FlowHeader.TableName()), tfContext)
	tfmContext.CreateTableTriggers(tfContext, []string{tierceronFlowIdColumnName})
	tfContext.InitNotify()
	tfmContext.SyncTableCycle(tfContext, []string{tierceronFlowIdColumnName}, []string{tierceronFlowIdColumnName}, GetTierceronFlowConfigurationIndexedPathExt, nil, false)
	sqlIngestInterval := tfContext.RemoteDataSource["dbingestinterval"].(time.Duration)
	if sqlIngestInterval > 0 {
		// Implement pull from remote data source.
		// Only pull if ingest interval is set to > 0 value.
		_, err := tierceronFlowImport(tfmContext, tfContext)
		if err != nil {
			tfmContext.Log("Error grabbing configurations for tierceron flows", err)
		}

		ticker := time.NewTicker(time.Second * sqlIngestInterval)
		defer ticker.Stop()

		for range ticker.C {
			tfmContext.Log("Tierceron Flows is running and checking for changes.", nil)
			// Periodically checks the table for updates and send out state changes to flows.
			_, err := tierceronFlowImport(tfmContext, tfContext)
			if err != nil {
				tfmContext.Log("Error grabbing configurations for tierceron flows", err)
				continue
			}
		}
	}
	return nil
}

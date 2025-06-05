package dataflowstatistics

import (
	"errors"
	"strings"
	"time"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	"github.com/trimble-oss/tierceron/atrium/buildopts/flowcoreopts"
	"github.com/trimble-oss/tierceron/atrium/trcflow/core"
	trcflowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"

	flowcorehelper "github.com/trimble-oss/tierceron/atrium/trcflow/core/flowcorehelper"

	tcflow "github.com/trimble-oss/tierceron-core/v2/flow"
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
	tenantIndexPath, _ := coreopts.BuildOptions.GetDFSPathName()
	if _, ok := rowDataMap[flowcoreopts.DataflowTestIdColumn].(string); ok {
		if _, ok := rowDataMap[flowcoreopts.DataflowTestNameColumn].(string); ok {
			if _, ok := rowDataMap[flowcoreopts.DataflowTestStateCodeColumn].(string); ok {
				if _, ok := rowDataMap["flowGroup"].(string); ok {
					if first, second, third, fourth := rowDataMap[flowcoreopts.DataflowTestIdColumn].(string), rowDataMap[flowcoreopts.DataflowTestNameColumn].(string), rowDataMap[flowcoreopts.DataflowTestStateCodeColumn].(string), rowDataMap["flowGroup"].(string); first != "" && second != "" && third != "" && fourth != "" {
						return "super-secrets/PublicIndex/" + tenantIndexPath + "/" + flowcoreopts.DataflowTestIdColumn + "/" + rowDataMap[flowcoreopts.DataflowTestIdColumn].(string) + "/DataFlowStatistics/DataFlowGroup/" + rowDataMap["flowGroup"].(string) + "/dataFlowName/" + rowDataMap[flowcoreopts.DataflowTestNameColumn].(string) + "/" + rowDataMap[flowcoreopts.DataflowTestStateCodeColumn].(string), nil
					}
				}
			}
		}
	}

	return "", errors.New("could not find data flow statistic index")
}

func GetDataFlowUpdateTrigger(databaseName string, tableName string, iden1 string, iden2 string, iden3 string) string {
	return `CREATE TRIGGER tcUpdateTrigger_DataFlowStatistics AFTER UPDATE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + iden1 + `,new.` + iden2 + `,new.` + iden3 + `,current_timestamp());` +
		` END;`
}

func GetDataFlowInsertTrigger(databaseName string, tableName string, iden1 string, iden2 string, iden3 string) string {
	return `CREATE TRIGGER tcInsertTrigger_DataFlowStatistics AFTER INSERT ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + iden1 + `,new.` + iden2 + `,new.` + iden3 + `,current_timestamp());` +
		` END;`
}

func GetDataFlowDeleteTrigger(databaseName string, tableName string, iden1 string, iden2 string, iden3 string) string {
	return `CREATE TRIGGER tcDeleteTrigger_DataFlowStatistics AFTER DELETE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
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

func dataFlowStatPullRemote(tfmContextI flowcore.FlowMachineContext, tfContextI flowcore.FlowContext) error {
	tfmContext := tfmContextI.(*trcflowcore.TrcFlowMachineContext)
	tfContext := tfContextI.(*trcflowcore.TrcFlowContext)
	tenantIndexPath, tenantDFSIdPath := coreopts.BuildOptions.GetDFSPathName()
	tenantListData, tenantListErr := tfContext.GoMod.List("super-secrets/PublicIndex/"+tenantIndexPath+"/"+tenantDFSIdPath, tfmContext.DriverConfig.CoreConfig.Log)
	if tenantListErr != nil {
		return tenantListErr
	}

	if tenantListData == nil {
		return nil
	}

	for _, tenantIdList := range tenantListData.Data {
		for _, tenantId := range tenantIdList.([]interface{}) {
			flowGroupNameListData, flowGroupNameListErr := tfContext.GoMod.List("super-secrets/PublicIndex/"+tenantIndexPath+"/"+tenantDFSIdPath+"/"+tenantId.(string)+"/DataFlowStatistics/DataFlowGroup", tfmContext.DriverConfig.CoreConfig.Log)
			if flowGroupNameListErr != nil {
				return flowGroupNameListErr
			}

			for _, flowGroupNameList := range flowGroupNameListData.Data {
				for _, flowGroup := range flowGroupNameList.([]interface{}) {
					listData, listErr := tfContext.GoMod.List("super-secrets/PublicIndex/"+tenantIndexPath+"/"+tenantDFSIdPath+"/"+tenantId.(string)+"/DataFlowStatistics/DataFlowGroup/"+flowGroup.(string)+"/dataFlowName/", tfmContext.DriverConfig.CoreConfig.Log)
					if listData == nil {
						continue
					}

					if listErr != nil {
						return listErr
					}

					for _, testNameList := range listData.Data {
						for _, testName := range testNameList.([]interface{}) {
							testName = strings.ReplaceAll(testName.(string), "/", "")
							dfGroup := tccore.InitDataFlow(nil, flowGroup.(string), false)
							dfctx, _, err := dfGroup.GetDeliverStatCtx()
							if err != nil {
								tfmContext.Log("Failed to retrieve statistic", err)
								continue
							}
							if listData != nil {
								err := core.RetrieveStatistic(tfContext.GoMod, dfGroup, tenantId.(string), tenantIndexPath, tenantDFSIdPath, flowGroup.(string), testName.(string), tfmContext.DriverConfig.CoreConfig.Log)
								if err != nil {
									tfmContext.Log("Failed to retrieve statistic", err)
								}
							}

							//Push to table using this object
							if len(dfGroup.ChildNodes) > 0 {
								for _, dfstat := range dfGroup.ChildNodes {
									dfStatMap := dfstat.StatisticToMap()
									core.UpdateLastTestedDate(tfContext.GoMod, dfctx, dfStatMap)
									rows, _ := tfmContext.CallDBQuery(tfContext, dfssql.GetDataFlowStatisticLM(tenantId.(string), dfStatMap, tfContext.FlowSourceAlias, tfContext.Flow.TableName()), nil, false, "SELECT", nil, "")
									//dfgroup to table
									if len(rows) == 0 {
										if strings.Contains(flowGroup.(string), flowGroupName) {
											tfmContext.CallDBQuery(tfContext, dfssql.GetDataFlowStatisticInsertById(tenantId.(string), dfStatMap, tfContext.FlowSourceAlias, tfContext.Flow.TableName()), nil, false, "INSERT", nil, "") //true gets ninja tested time inside statisticToMap
										} else {
											statMap := dfstat.StatisticToMap()
											tfmContext.CallDBQuery(tfContext, dfssql.GetDataFlowStatisticInsertById(tenantId.(string), statMap, tfContext.FlowSourceAlias, tfContext.Flow.TableName()), nil, false, "INSERT", nil, "")
										}
									} else {
										statMap := dfstat.StatisticToMap()
										for _, value := range rows {
											if coreopts.BuildOptions.CompareLastModified(dfStatMap, dfssql.DataFlowStatisticsSparseArrayToMap(value)) { //If equal-> do nothing
												continue
											} else { //If not equal -> update
												tfmContext.CallDBQuery(tfContext, dfssql.GetDataFlowStatisticUpdateById(tenantId.(string), statMap, tfContext.FlowSourceAlias, tfContext.Flow.TableName()), nil, false, "INSERT", []tcflow.FlowNameType{tcflow.FlowNameType(tfContext.Flow.TableName())}, "")
											}
										}
									}
								}
							} else {
								if len(dfGroup.MashupDetailedElement.Data) > 0 {
									dfgStatMap := dfGroup.StatisticToMap()
									tfmContext.CallDBQuery(tfContext, dfssql.GetDataFlowStatisticInsertById(tenantId.(string), dfgStatMap, tfContext.FlowSourceAlias, tfContext.Flow.TableName()), nil, false, "INSERT", []tcflow.FlowNameType{tcflow.FlowNameType(tfContext.Flow.TableName())}, "")
								}
							}
						}
					}
				}
			}
		}
	}

	if tfContext.Init { //Alert interface that the table is ready for permissions
		tfmContext.PermissionChan <- trcflowcore.PermissionUpdate{TableName: tfContext.Flow.TableName(), CurrentState: tfContext.GetFlowStateState()}
		tfContext.Init = false
	}

	return nil
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

func ProcessDataFlowStatConfigurations(tfmContext flowcore.FlowMachineContext, tfContext flowcore.FlowContext) error {
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
		tfContext.SetCustomSeedTrcdbFunc(dataFlowStatPullRemote)
	}

	return flowcore.ProcessTableConfigurations(tfmContext, tfContext)
}

func KickOffTimedRefresh(tfContext *trcflowcore.TrcFlowContext, timing string) bool {
	switch { //Always at midnight
	case timing == "Daily":
		tfContext.PushState("flowStateReceiver", flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: "refreshingDaily", FlowAlias: tfContext.FlowState.FlowAlias})
		loc, _ := time.LoadLocation("America/Los_Angeles")
		now := time.Now().In(loc)
		midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, loc)
		//midnight := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second()+3, 0, loc)
		timeTilMidnight := midnight.Sub(now)
		go func(tfc *trcflowcore.TrcFlowContext, tilMidnight time.Duration) {
			refresh = true
			time.Sleep(tilMidnight)
			refreshTime := time.Duration(time.Second * 0)
			for {
				select {
				case <-endRefreshChan:
					tfContext.Log("Daily Refresh Ended - no longer refreshing DFS", nil)
					return
				case <-time.After(refreshTime):
					tfContext.Log("Daily Refresh Triggered - refreshing DFS", nil)
					tfContext.PushState("flowStateReceiver", tfc.NewFlowStateUpdate("3", "refreshingDaily"))
					refreshTime = time.Duration(time.Hour * 24)
				}

			}
		}(tfContext, timeTilMidnight)
	case timing == "End":
		endRefreshChan <- true
		refresh = false
		tfContext.PushState("flowStateReceiver", tfContext.NewFlowStateUpdate("2", "refreshEnded"))
	case timing == "Ended":
		for len(endRefreshChan) > 0 {
			<-endRefreshChan
		}
		return true
	default:
		return false
	}

	return true
}

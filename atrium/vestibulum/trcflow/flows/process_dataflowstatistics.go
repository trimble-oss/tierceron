package flows

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/trimble-oss/tierceron/atrium/buildopts/flowcoreopts"
	flowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"

	flowcorehelper "github.com/trimble-oss/tierceron/atrium/trcflow/core/flowcorehelper"
	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"
	"github.com/trimble-oss/tierceron/pkg/trcx/extract"

	dfssql "github.com/trimble-oss/tierceron/atrium/vestibulum/trcflow/flows/flowsql"

	sqle "github.com/dolthub/go-mysql-server/sql"
)

const flowGroupName = "Ninja"

var refresh = false
var endRefreshChan = make(chan bool, 1)

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
	return `CREATE TRIGGER tcUpdateTrigger AFTER UPDATE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + iden1 + `,new.` + iden2 + `,new.` + iden3 + `,current_timestamp());` +
		` END;`
}

func GetDataFlowInsertTrigger(databaseName string, tableName string, iden1 string, iden2 string, iden3 string) string {
	return `CREATE TRIGGER tcInsertTrigger AFTER INSERT ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + iden1 + `,new.` + iden2 + `,new.` + iden3 + `,current_timestamp());` +
		` END;`
}

func GetDataFlowDeleteTrigger(databaseName string, tableName string, iden1 string, iden2 string, iden3 string) string {
	return `CREATE TRIGGER tcDeleteTrigger AFTER DELETE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (old.` + iden1 + `,old.` + iden2 + `,old.` + iden3 + `,current_timestamp());` +
		` END;`
}

func getDataFlowStatisticsSchema(tableName string) sqle.PrimaryKeySchema {
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

func dataFlowStatPullRemote(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) error {
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
							dfGroup := flowcore.InitDataFlow(nil, flowGroup.(string), false)
							if listData != nil {
								err := dfGroup.RetrieveStatistic(tfContext.GoMod, tenantId.(string), tenantIndexPath, tenantDFSIdPath, flowGroup.(string), testName.(string), tfmContext.DriverConfig.CoreConfig.Log)
								if err != nil {
									tfmContext.Log("Failed to retrieve statistic", err)
								}
							}

							//Push to table using this object
							if len(dfGroup.ChildNodes) > 0 {
								for _, dfstat := range dfGroup.ChildNodes {
									dfStatMap := dfGroup.StatisticToMap(tfContext.GoMod, dfstat, true)
									rows, _ := tfmContext.CallDBQuery(tfContext, dfssql.GetDataFlowStatisticLM(tenantId.(string), dfStatMap, tfContext.FlowSourceAlias, tfContext.Flow.TableName()), nil, false, "SELECT", nil, "")
									//dfgroup to table
									if len(rows) == 0 {
										if strings.Contains(flowGroup.(string), flowGroupName) {
											tfmContext.CallDBQuery(tfContext, dfssql.GetDataFlowStatisticInsert(tenantId.(string), dfGroup.StatisticToMap(tfContext.GoMod, dfstat, true), tfContext.FlowSourceAlias, tfContext.Flow.TableName()), nil, false, "INSERT", nil, "") //true gets ninja tested time inside statisticToMap
										} else {
											tfmContext.CallDBQuery(tfContext, dfssql.GetDataFlowStatisticInsert(tenantId.(string), dfGroup.StatisticToMap(tfContext.GoMod, dfstat, false), tfContext.FlowSourceAlias, tfContext.Flow.TableName()), nil, false, "INSERT", nil, "")
										}
									} else {
										for _, value := range rows {
											if coreopts.BuildOptions.CompareLastModified(dfStatMap, dfssql.DataFlowStatisticsSparseArrayToMap(value)) { //If equal-> do nothing
												continue
											} else { //If not equal -> update
												tfmContext.CallDBQuery(tfContext, dfssql.GetDataFlowStatisticUpdate(tenantId.(string), dfGroup.StatisticToMap(tfContext.GoMod, dfstat, false), tfContext.FlowSourceAlias, tfContext.Flow.TableName()), nil, false, "INSERT", []flowcore.FlowNameType{flowcore.FlowNameType(tfContext.Flow.TableName())}, "")
											}
										}
									}
								}
							} else {
								if len(dfGroup.MashupDetailedElement.Data) > 0 {
									tfmContext.CallDBQuery(tfContext, dfssql.GetDataFlowStatisticInsert(tenantId.(string), dfGroup.StatisticToMap(tfContext.GoMod, dfGroup, false), tfContext.FlowSourceAlias, tfContext.Flow.TableName()), nil, false, "INSERT", []flowcore.FlowNameType{flowcore.FlowNameType(tfContext.Flow.TableName())}, "")
								}
							}
						}
					}
				}
			}
		}
	}

	tfContext.FlowLock.Lock()
	if tfContext.Init { //Alert interface that the table is ready for permissions
		tfmContext.PermissionChan <- flowcore.PermissionUpdate{TableName: tfContext.Flow.TableName(), CurrentState: tfContext.FlowState.State}
		tfContext.Init = false
	}
	tfContext.FlowLock.Unlock()

	return nil
}

func prepareDataFlowChangeTable(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) {
	tfmContext.GetTableModifierLock().Lock()
	changeTableName := tfContext.Flow.TableName() + "_Changes"
	tfmContext.CallDBQuery(tfContext, map[string]interface{}{"TrcQuery": "DROP TABLE " + tfmContext.TierceronEngine.Database.Name() + "." + changeTableName}, nil, false, "DELETE", nil, "")
	changeTableErr := tfmContext.TierceronEngine.Database.CreateTable(tfmContext.TierceronEngine.Context, changeTableName, sqle.NewPrimaryKeySchema(sqle.Schema{
		{Name: flowcoreopts.DataflowTestNameColumn, Type: sqle.Text, Source: changeTableName, PrimaryKey: true},
		{Name: flowcoreopts.DataflowTestIdColumn, Type: sqle.Text, Source: changeTableName, PrimaryKey: true},
		{Name: flowcoreopts.DataflowTestStateCodeColumn, Type: sqle.Text, Source: changeTableName, PrimaryKey: true},
		{Name: "updateTime", Type: sqle.Timestamp, Source: changeTableName},
	}),
		flowcore.TableCollationIdGen(changeTableName),
	)
	if changeTableErr != nil {
		tfmContext.Log("Error creating ninja change table", changeTableErr)
	}
	tfmContext.CreateDataFlowTableTriggers(tfContext, flowcoreopts.DataflowTestNameColumn, flowcoreopts.DataflowTestIdColumn, flowcoreopts.DataflowTestStateCodeColumn, GetDataFlowInsertTrigger, GetDataFlowUpdateTrigger, GetDataFlowDeleteTrigger)
	tfmContext.GetTableModifierLock().Unlock()
}

func ProcessDataFlowStatConfigurations(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) error {
	tfmContext.AddTableSchema(getDataFlowStatisticsSchema(tfContext.Flow.TableName()), tfContext)
	prepareDataFlowChangeTable(tfmContext, tfContext) //Change table needs to be set again due to composite key - different from other tables
	tfContext.FlowLock.Lock()
	tfContext.ReadOnly = false //Change this to false for writeback***
	tfContext.FlowState = <-tfContext.RemoteDataSource["flowStateController"].(chan flowcorehelper.CurrentFlowState)
	tfContext.FlowState.SyncFilter = "N/A"
	tfContext.CustomSeedTrcDb = dataFlowStatPullRemote

	/*if tfContext.FlowState.State != 1 && tfContext.FlowState.State != 2 {
		tfmContext.PermissionChan <- tfContext.Flow.TableName()
		tfContext.Init = false
	}*/

	tfContext.FlowLock.Unlock()
	stateUpdateChannel := tfContext.RemoteDataSource["flowStateReceiver"].(chan flowcorehelper.FlowStateUpdate)

	go func(tfs flowcorehelper.CurrentFlowState, sL *sync.Mutex, sPC chan flowcorehelper.FlowStateUpdate) {
		sL.Lock()
		previousState := tfs
		sL.Unlock()
		for {
			select {
			case stateUpdate := <-tfContext.RemoteDataSource["flowStateController"].(chan flowcorehelper.CurrentFlowState):
				stateUpdate.SyncFilter = "N/A"
				if previousState.State == stateUpdate.State && previousState.SyncMode == stateUpdate.SyncMode && previousState.SyncFilter == stateUpdate.SyncFilter && previousState.FlowAlias == stateUpdate.FlowAlias {
					continue
				} else if previousState.SyncMode == "refreshingDaily" && stateUpdate.SyncMode != "refreshEnd" && stateUpdate.State == 2 && int(previousState.State) != coreopts.BuildOptions.PreviousStateCheck(int(stateUpdate.State)) {
					sPC <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: strconv.Itoa(int(stateUpdate.State)), SyncFilter: stateUpdate.SyncFilter, SyncMode: previousState.SyncMode, FlowAlias: tfContext.FlowState.FlowAlias}
					break
				} else if int(previousState.State) != coreopts.BuildOptions.PreviousStateCheck(int(stateUpdate.State)) && stateUpdate.State != previousState.State {
					sPC <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: strconv.Itoa(int(previousState.State)), SyncFilter: stateUpdate.SyncFilter, SyncMode: stateUpdate.SyncMode, FlowAlias: tfContext.FlowState.FlowAlias}
					continue
				}
				previousState = stateUpdate
				sL.Lock()
				tfContext.FlowState = stateUpdate
				sL.Unlock()
			}
		}
	}(tfContext.FlowState, tfContext.FlowLock, stateUpdateChannel)

	tfContext.Init = true

	sqlIngestInterval := tfContext.RemoteDataSource["dbingestinterval"].(time.Duration)
	if sqlIngestInterval > 0 {
		// Implement pull from remote data source
		// Only pull if ingest interval is set to > 0 value.
		afterTime := time.Duration(0)
		for {
			select {
			case <-time.After(time.Millisecond * afterTime):
				afterTime = sqlIngestInterval
				tfContext.FlowLock.Lock()
				if tfContext.FlowState.State == 3 {
					tfContext.FlowLock.Unlock()
					tfmContext.PermissionChan <- flowcore.PermissionUpdate{TableName: tfContext.Flow.TableName(), CurrentState: tfContext.FlowState.State}
					if tfContext.CancelContext != nil {
						tfContext.CancelContext() //This cancel also pushes any final changes to vault before closing sync cycle.
						var baseTableTemplate extract.TemplateResultData
						trcvutils.LoadBaseTemplate(tfmContext.DriverConfig, &baseTableTemplate, tfContext.GoMod, tfContext.FlowSource, tfContext.Flow.ServiceName(), tfContext.FlowPath)
						tfContext.FlowData = &baseTableTemplate
					}
					tfmContext.Log("DataFlowStatistics flow is being stopped...", nil)
					stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "0", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: tfContext.FlowState.SyncMode, FlowAlias: tfContext.FlowState.FlowAlias}
					continue
				} else if tfContext.FlowState.State == 0 {
					tfContext.FlowLock.Unlock()
					tfmContext.Log("DataFlowStatistics flow is currently offline...", nil)
					if tfContext.FlowState.SyncMode == "refreshingDaily" {
						refresh = true
						stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "1", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: tfContext.FlowState.SyncMode, FlowAlias: tfContext.FlowState.FlowAlias}
					}
					continue
				} else if tfContext.FlowState.State == 1 {
					tfContext.FlowLock.Unlock()
					tfmContext.Log("DataFlowStatistics flow is restarting...", nil)
					tfContext.Init = true
					tfmContext.CallDBQuery(tfContext, map[string]interface{}{"TrcQuery": "truncate " + tfContext.FlowSourceAlias + "." + tfContext.Flow.TableName()}, nil, false, "DELETE", nil, "")
					stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: tfContext.FlowState.SyncMode, FlowAlias: tfContext.FlowState.FlowAlias}
					continue
				} else if tfContext.FlowState.State == 2 {
					tfContext.FlowLock.Unlock()
					if tfContext.Init {
						go tfmContext.SyncTableCycle(tfContext, flowcoreopts.DataflowTestNameColumn, []string{flowcoreopts.DataflowTestIdColumn, flowcoreopts.DataflowTestStateCodeColumn, flowcoreopts.DataflowTestNameColumn}, GetDataflowStatIndexedPathExt, nil, false)
					}
				} else {
					tfContext.FlowLock.Unlock()
					tfmContext.Log("Ignoring invalid flow.", nil)
					continue
				}

				tfContext.FlowLock.Lock()
				if strings.HasPrefix(tfContext.FlowState.SyncMode, "refresh") { //This is to refresh from vault - different from pulling/pushing.
					refreshSuffix, _ := strings.CutPrefix(tfContext.FlowState.SyncMode, "refresh")
					if tfContext.FlowState.SyncMode == "refreshingDaily" {
						if !refresh { //This is for if trcdb loads up in "refreshingDaily" -> need to kick off refresh again.
							KickOffTimedRefresh(tfContext, stateUpdateChannel, "Daily")
						}
					} else if !KickOffTimedRefresh(tfContext, stateUpdateChannel, refreshSuffix) {
						tfmContext.Log("DataFlowStatistics has an invalid refresh timing"+flowcorehelper.SyncCheck(tfContext.FlowState.SyncMode)+".", nil)
						tfContext.FlowState.SyncMode = "InvalidRefreshMode"
						stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: "InvalidRefreshMode", FlowAlias: tfContext.FlowState.FlowAlias}
					}
				}
				tfContext.FlowLock.Unlock()

				tfContext.FlowLock.Lock()
				if tfContext.FlowState.SyncMode == "pullonce" {
					tfContext.FlowState.SyncMode = "pullsynccomplete"
					stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: "pullsynccomplete", FlowAlias: tfContext.FlowState.FlowAlias}
				}
				tfContext.FlowLock.Unlock()

				tfContext.FlowLock.Lock()
				tfmContext.Log("DataFlowStatistics is running and checking for changes"+flowcorehelper.SyncCheck(tfContext.FlowState.SyncMode)+".", nil)
				tfContext.FlowLock.Unlock()
			}
		}
	}
	tfContext.CancelContext()
	return nil
}

func KickOffTimedRefresh(tfContext *flowcore.TrcFlowContext, stateUpdateChannel chan flowcorehelper.FlowStateUpdate, timing string) bool {
	switch { //Always at midnight
	case timing == "Daily":
		stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: "refreshingDaily", FlowAlias: tfContext.FlowState.FlowAlias}
		loc, _ := time.LoadLocation("America/Los_Angeles")
		now := time.Now().In(loc)
		midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, loc)
		//midnight := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second()+3, 0, loc)
		timeTilMidnight := midnight.Sub(now)
		go func(tfc *flowcore.TrcFlowContext, tilMidnight time.Duration) {
			refresh = true
			time.Sleep(tilMidnight)
			refreshTime := time.Duration(time.Second * 0)
			for {
				select {
				case <-endRefreshChan:
					tfContext.Log.Println("Daily Refresh Ended - no longer refreshing DFS")
					return
				case <-time.After(refreshTime):
					tfContext.Log.Println("Daily Refresh Triggered - refreshing DFS")
					stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfc.Flow.TableName(), StateUpdate: "3", SyncFilter: tfc.FlowState.SyncFilter, SyncMode: "refreshingDaily", FlowAlias: tfc.FlowState.FlowAlias}
					refreshTime = time.Duration(time.Hour * 24)
				}

			}
		}(tfContext, timeTilMidnight)
	case timing == "End":
		endRefreshChan <- true
		refresh = false
		stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: "refreshEnded", FlowAlias: tfContext.FlowState.FlowAlias}
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

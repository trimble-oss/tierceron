package flows

import (
	"errors"
	"strings"
	"sync"
	flowcore "tierceron/trcflow/core"

	flowcorehelper "tierceron/trcflow/core/flowcorehelper"
	flowutil "tierceron/trcvault/flowutil"
	trcvutils "tierceron/trcvault/util"
	"tierceron/trcx/extract"
	"time"

	utilcore "VaultConfig.TenantConfig/util/core"

	dfssql "tierceron/trcflow/flows/flowsql"

	"VaultConfig.TenantConfig/util/core"
	sqle "github.com/dolthub/go-mysql-server/sql"
)

const flowGroupName = "Ninja"

func GetDataflowStatIndexedPathExt(engine interface{}, rowDataMap map[string]interface{}, indexColumnNames interface{}, databaseName string, tableName string, dbCallBack func(interface{}, map[string]interface{}) (string, []string, [][]interface{}, error)) (string, error) {
	tenantIndexPath, _ := core.GetDFSPathName()

	if first, second, third, fourth := rowDataMap[dfssql.DataflowTestIdColumn].(string), rowDataMap[dfssql.DataflowTestNameColumn].(string), rowDataMap[dfssql.DataflowTestStateCodeColumn].(string), rowDataMap["flowGroup"].(string); first != "" && second != "" && third != "" && fourth != "" {
		return "super-secrets/PublicIndex/" + tenantIndexPath + "/" + dfssql.DataflowTestIdColumn + "/" + rowDataMap[dfssql.DataflowTestIdColumn].(string) + "/DataFlowStatistics/DataFlowGroup/" + rowDataMap["flowGroup"].(string) + "/dataFlowName/" + rowDataMap[dfssql.DataflowTestNameColumn].(string) + "/" + rowDataMap[dfssql.DataflowTestStateCodeColumn].(string), nil
	}
	return "", errors.New("Could not find data flow statistic index.")
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

func getDataFlowStatisticsSchema(tableName string) sqle.PrimaryKeySchema {
	return sqle.NewPrimaryKeySchema(sqle.Schema{
		{Name: dfssql.DataflowTestNameColumn, Type: sqle.Text, Source: tableName, PrimaryKey: true}, //composite key
		{Name: dfssql.DataflowTestIdColumn, Type: sqle.Text, Source: tableName, PrimaryKey: true},
		{Name: "flowGroup", Type: sqle.Text, Source: tableName},
		{Name: "mode", Type: sqle.Text, Source: tableName},
		{Name: "stateCode", Type: sqle.Text, Source: tableName, PrimaryKey: true},
		{Name: "stateName", Type: sqle.Text, Source: tableName},
		{Name: "timeSplit", Type: sqle.Text, Source: tableName},
		{Name: "lastTestedDate", Type: sqle.Text, Source: tableName},
	})
}

func dataFlowStatPullRemote(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) error {
	tenantIndexPath, tenantDFSIdPath := core.GetDFSPathName()
	tenantListData, tenantListErr := tfContext.GoMod.List("super-secrets/PublicIndex/"+tenantIndexPath+"/"+tenantDFSIdPath, tfmContext.Config.Log)
	if tenantListErr != nil {
		return tenantListErr
	}

	if tenantListData == nil {
		return nil
	}

	for _, tenantIdList := range tenantListData.Data {
		for _, tenantId := range tenantIdList.([]interface{}) {
			flowGroupNameListData, flowGroupNameListErr := tfContext.GoMod.List("super-secrets/PublicIndex/"+tenantIndexPath+"/"+tenantDFSIdPath+"/"+tenantId.(string)+"/DataFlowStatistics/DataFlowGroup", tfmContext.Config.Log)
			if flowGroupNameListErr != nil {
				return flowGroupNameListErr
			}

			for _, flowGroupNameList := range flowGroupNameListData.Data {
				for _, flowGroup := range flowGroupNameList.([]interface{}) {
					listData, listErr := tfContext.GoMod.List("super-secrets/PublicIndex/"+tenantIndexPath+"/"+tenantDFSIdPath+"/"+tenantId.(string)+"/DataFlowStatistics/DataFlowGroup/"+flowGroup.(string)+"/dataFlowName/", tfmContext.Config.Log)
					if listData == nil {
						continue
					}

					if listErr != nil {
						return listErr
					}

					for _, testNameList := range listData.Data {
						for _, testName := range testNameList.([]interface{}) {
							testName = strings.ReplaceAll(testName.(string), "/", "")
							dfGroup := flowutil.InitDataFlow(nil, flowGroup.(string), false)
							if listData != nil {
								err := dfGroup.RetrieveStatistic(tfContext.GoMod, tenantId.(string), tenantIndexPath, tenantDFSIdPath, flowGroup.(string), testName.(string), tfmContext.Config.Log)
								if err != nil {
									tfmContext.Log("Failed to retrieve statistic", err)
								}
							}

							//Push to table using this object
							if len(dfGroup.ChildNodes) > 0 {
								for _, dfstat := range dfGroup.ChildNodes {
									dfStatMap := dfGroup.StatisticToMap(tfContext.GoMod, dfstat, true)
									rows := tfmContext.CallDBQuery(tfContext, dfssql.GetDataFlowStatisticLM(tenantId.(string), dfStatMap, tfContext.FlowSourceAlias, tfContext.Flow.TableName()), nil, false, "SELECT", nil, "")
									//dfgroup to table
									if len(rows) == 0 {
										if strings.Contains(flowGroup.(string), flowGroupName) {
											tfmContext.CallDBQuery(tfContext, dfssql.GetDataFlowStatisticInsert(tenantId.(string), dfGroup.StatisticToMap(tfContext.GoMod, dfstat, true), tfContext.FlowSourceAlias, tfContext.Flow.TableName()), nil, false, "INSERT", nil, "") //true gets ninja tested time inside statisticToMap
										} else {
											tfmContext.CallDBQuery(tfContext, dfssql.GetDataFlowStatisticInsert(tenantId.(string), dfGroup.StatisticToMap(tfContext.GoMod, dfstat, false), tfContext.FlowSourceAlias, tfContext.Flow.TableName()), nil, false, "INSERT", nil, "")
										}
									} else {
										for _, value := range rows {
											if utilcore.CompareLastModified(dfStatMap, dfssql.DataFlowStatisticsArrayToMap(value)) { //If equal-> do nothing
												continue
											} else { //If not equal -> update
												tfmContext.CallDBQuery(tfContext, dfssql.GetDataFlowStatisticUpdate(tenantId.(string), dfGroup.StatisticToMap(tfContext.GoMod, dfGroup, false), tfContext.FlowSourceAlias, tfContext.Flow.TableName()), nil, false, "INSERT", []flowcore.FlowNameType{flowcore.FlowNameType(tfContext.Flow.TableName())}, "")
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
		tfmContext.PermissionChan <- tfContext.Flow.TableName()
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
		{Name: dfssql.DataflowTestNameColumn, Type: sqle.Text, Source: changeTableName, PrimaryKey: true},
		{Name: dfssql.DataflowTestIdColumn, Type: sqle.Text, Source: changeTableName, PrimaryKey: true},
		{Name: dfssql.DataflowTestStateCodeColumn, Type: sqle.Text, Source: changeTableName, PrimaryKey: true},
		{Name: "updateTime", Type: sqle.Timestamp, Source: changeTableName},
	}),
		flowcore.TableCollationIdGen(changeTableName),
	)
	if changeTableErr != nil {
		tfmContext.Log("Error creating ninja change table", changeTableErr)
	}
	tfmContext.CreateDataFlowTableTriggers(tfContext, dfssql.DataflowTestNameColumn, dfssql.DataflowTestIdColumn, dfssql.DataflowTestStateCodeColumn, GetDataFlowInsertTrigger, GetDataFlowUpdateTrigger)
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

	if tfContext.FlowState.State != 1 && tfContext.FlowState.State != 2 {
		tfmContext.PermissionChan <- tfContext.Flow.TableName()
		tfContext.Init = false
	}

	tfContext.FlowLock.Unlock()
	go func(tfs flowcorehelper.CurrentFlowState, sL *sync.Mutex) {
		for {
			select {
			case stateUpdate := <-tfContext.RemoteDataSource["flowStateController"].(chan flowcorehelper.CurrentFlowState):
				sL.Lock()
				tfContext.FlowState = stateUpdate
				tfContext.FlowState.SyncFilter = "N/A" //Overwrites any changes to syncFilter as this flow doesn't support it
				tfContext.FlowState.SyncMode = "N/A"
				sL.Unlock()
			}
		}
	}(tfContext.FlowState, tfContext.FlowLock)

	stateUpdateChannel := tfContext.RemoteDataSource["flowStateReceiver"].(chan flowcorehelper.FlowStateUpdate)
	syncInit := true

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
					if tfContext.CancelContext != nil {
						tfContext.CancelContext() //This cancel also pushes any final changes to vault before closing sync cycle.
						var baseTableTemplate extract.TemplateResultData
						trcvutils.LoadBaseTemplate(tfmContext.Config, &baseTableTemplate, tfContext.GoMod, tfContext.FlowSource, tfContext.Flow.ServiceName(), tfContext.FlowPath)
						tfContext.FlowData = &baseTableTemplate
					}
					tfmContext.Log("DataFlowStatistics flow is being stopped...", nil)
					stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "0", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: tfContext.FlowState.SyncMode}
					continue
				} else if tfContext.FlowState.State == 0 {
					tfContext.FlowLock.Unlock()
					tfmContext.Log("DataFlowStatistics flow is currently offline...", nil)
					continue
				} else if tfContext.FlowState.State == 1 {
					tfContext.FlowLock.Unlock()
					tfmContext.Log("DataFlowStatistics flow is restarting...", nil)
					syncInit = true
					tfmContext.CallDBQuery(tfContext, map[string]interface{}{"TrcQuery": "truncate " + tfContext.FlowSourceAlias + "." + tfContext.Flow.TableName()}, nil, false, "DELETE", nil, "")
					stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: tfContext.FlowState.SyncMode}
					continue
				} else if tfContext.FlowState.State == 2 {
					tfContext.FlowLock.Unlock()
					if syncInit {
						go tfmContext.SyncTableCycle(tfContext, dfssql.DataflowTestNameColumn, []string{dfssql.DataflowTestIdColumn, dfssql.DataflowTestStateCodeColumn}, GetDataflowStatIndexedPathExt, nil, false)
						syncInit = false
					}
				} else {
					tfContext.FlowLock.Unlock()
					tfmContext.Log("Ignoring invalid flow.", nil)
					continue
				}
				tfContext.FlowLock.Lock()
				if tfContext.FlowState.SyncMode == "pullonce" {
					tfContext.FlowState.SyncMode = "pullsynccomplete"
					stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: "pullsynccomplete"}
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

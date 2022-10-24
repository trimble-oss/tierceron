package flows

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	flowcore "tierceron/trcflow/core"

	flowcorehelper "tierceron/trcflow/core/flowcorehelper"
	flowutil "tierceron/trcvault/flowutil"
	trcvutils "tierceron/trcvault/util"
	"tierceron/trcx/extract"
	"time"

	"VaultConfig.TenantConfig/util/core"
	sqle "github.com/dolthub/go-mysql-server/sql"
)

const flowGroupName = "Ninja"
const dataflowTestNameColumn = "flowName"
const dataflowTestIdColumn = "argosId"
const dataflowTestStateCodeColumn = "stateCode"

func GetDataflowStatIndexedPathExt(engine interface{}, rowDataMap map[string]interface{}, vaultIndexColumnName string, databaseName string, tableName string, dbCallBack func(interface{}, map[string]string) (string, []string, [][]interface{}, error)) (string, error) {
	tenantIndexPath, _ := core.GetDFSPathName()

	if first, second, third, fourth := rowDataMap[dataflowTestIdColumn].(string), rowDataMap[dataflowTestNameColumn].(string), rowDataMap[dataflowTestStateCodeColumn].(string), rowDataMap["flowGroup"].(string); first != "" && second != "" && third != "" && fourth != "" {
		return "super-secrets/PublicIndex/" + tenantIndexPath + "/" + dataflowTestIdColumn + "/" + rowDataMap[dataflowTestIdColumn].(string) + "/DataFlowStatistics/DataFlowGroup/" + rowDataMap["flowGroup"].(string) + "/dataFlowName/" + rowDataMap[dataflowTestNameColumn].(string) + "/" + rowDataMap[dataflowTestStateCodeColumn].(string), nil
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
		{Name: dataflowTestNameColumn, Type: sqle.Text, Source: tableName, PrimaryKey: true}, //composite key
		{Name: dataflowTestIdColumn, Type: sqle.Text, Source: tableName, PrimaryKey: true},
		{Name: "flowGroup", Type: sqle.Text, Source: tableName},
		{Name: "mode", Type: sqle.Text, Source: tableName},
		{Name: "stateCode", Type: sqle.Text, Source: tableName, PrimaryKey: true},
		{Name: "stateName", Type: sqle.Text, Source: tableName},
		{Name: "timeSplit", Type: sqle.Text, Source: tableName},
		{Name: "lastTestedDate", Type: sqle.Text, Source: tableName},
	})
}

func GetDataFlowStatisticInsert(tenantId string, ninjaStatistic map[string]interface{}, dbName string, tableName string) map[string]string {
	tenantId = strings.ReplaceAll(tenantId, "/", "")
	sqlstr := map[string]string{
		"TrcQuery": `INSERT IGNORE INTO ` + dbName + `.` + tableName + `(` + dataflowTestNameColumn + `, ` + dataflowTestIdColumn + `, flowGroup, mode, stateCode, stateName, timeSplit, lastTestedDate) VALUES ('` +
			ninjaStatistic["flowName"].(string) + `','` + tenantId + `','` +
			ninjaStatistic["flowGroup"].(string) + `','` + strconv.Itoa(ninjaStatistic["mode"].(int)) +
			`','` + ninjaStatistic["stateCode"].(string) + `','` + ninjaStatistic["stateName"].(string) +
			`','` + ninjaStatistic["timeSplit"].(string) + `','` + ninjaStatistic["lastTestedDate"].(string) + `')` +
			` ON DUPLICATE KEY UPDATE ` +
			dataflowTestNameColumn + `= VALUES(` + dataflowTestNameColumn + `),` + dataflowTestIdColumn + `= VALUES(` + dataflowTestIdColumn + `),flowGroup = VALUES(flowGroup),mode = VALUES(mode),stateCode = VALUES(stateCode),stateName = VALUES(stateName),timeSplit = VALUES(timeSplit), lastTestedDate = VALUES(lastTestedDate)`,
	}
	return sqlstr
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
									//dfgroup to table
									if strings.Contains(flowGroup.(string), flowGroupName) {
										tfmContext.CallDBQuery(tfContext, GetDataFlowStatisticInsert(tenantId.(string), dfGroup.StatisticToMap(tfContext.GoMod, dfstat, true), tfContext.FlowSourceAlias, tfContext.Flow.TableName()), nil, true, "INSERT", []flowcore.FlowNameType{flowcore.FlowNameType(tfContext.Flow.TableName())}, "") //true gets ninja tested time inside statisticToMap
									} else {
										tfmContext.CallDBQuery(tfContext, GetDataFlowStatisticInsert(tenantId.(string), dfGroup.StatisticToMap(tfContext.GoMod, dfstat, false), tfContext.FlowSourceAlias, tfContext.Flow.TableName()), nil, true, "INSERT", []flowcore.FlowNameType{flowcore.FlowNameType(tfContext.Flow.TableName())}, "")
									}
								}
							} else {
								if len(dfGroup.MashupDetailedElement.Data) > 0 {
									tfmContext.CallDBQuery(tfContext, GetDataFlowStatisticInsert(tenantId.(string), dfGroup.StatisticToMap(tfContext.GoMod, dfGroup, false), tfContext.FlowSourceAlias, tfContext.Flow.TableName()), nil, true, "INSERT", []flowcore.FlowNameType{flowcore.FlowNameType(tfContext.Flow.TableName())}, "")
								}
							}
						}
					}
				}
			}
		}
	}
	return nil
}

func prepareDataFlowChangeTable(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) {
	tfmContext.GetTableModifierLock().Lock()
	changeTableName := tfContext.Flow.TableName() + "_Changes"
	tfmContext.CallDBQuery(tfContext, map[string]string{"TrcQuery": "DROP TABLE " + tfmContext.TierceronEngine.Database.Name() + "." + changeTableName}, nil, false, "DELETE", nil, "")
	changeTableErr := tfmContext.TierceronEngine.Database.CreateTable(tfmContext.TierceronEngine.Context, changeTableName, sqle.NewPrimaryKeySchema(sqle.Schema{
		{Name: dataflowTestNameColumn, Type: sqle.Text, Source: changeTableName, PrimaryKey: true},
		{Name: dataflowTestIdColumn, Type: sqle.Text, Source: changeTableName, PrimaryKey: true},
		{Name: dataflowTestStateCodeColumn, Type: sqle.Text, Source: changeTableName, PrimaryKey: true},
		{Name: "updateTime", Type: sqle.Timestamp, Source: changeTableName},
	}),
		flowcore.TableCollationIdGen(changeTableName),
	)
	if changeTableErr != nil {
		tfmContext.Log("Error creating ninja change table", changeTableErr)
	}
	tfmContext.CreateDataFlowTableTriggers(tfContext, dataflowTestNameColumn, dataflowTestIdColumn, dataflowTestStateCodeColumn, GetDataFlowInsertTrigger, GetDataFlowUpdateTrigger)
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
					tfmContext.CallDBQuery(tfContext, map[string]string{"TrcQuery": "truncate " + tfContext.FlowSourceAlias + "." + tfContext.Flow.TableName()}, nil, false, "DELETE", nil, "")
					stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: tfContext.FlowState.SyncMode}
					continue
				} else if tfContext.FlowState.State == 2 {
					tfContext.FlowLock.Unlock()
					if syncInit {
						go tfmContext.SyncTableCycle(tfContext, dataflowTestNameColumn, dataflowTestIdColumn, dataflowTestStateCodeColumn, GetDataflowStatIndexedPathExt, nil, false)
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

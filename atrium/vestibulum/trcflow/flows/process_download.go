package flows

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	flowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"
	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"
	"github.com/trimble-oss/tierceron/pkg/trcx/extract"

	flowcorehelper "github.com/trimble-oss/tierceron/atrium/trcflow/core/flowcorehelper"
	flowsql "github.com/trimble-oss/tierceron/atrium/vestibulum/trcflow/flows/flowsql"

	sqle "github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/expression"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	tcopts "github.com/trimble-oss/tierceron/buildopts/tcopts"
)

const fileIdColumnName = "fileId"
const argosIdentifierColumnName = "argosIdentifier"

var downloadQueryStartedAlert chan map[string]string

var downloadQueryFinishedAlert chan bool

func getDownloadColumnName() string {
	return argosIdentifierColumnName
}

func getDownloadSecondColumnName() string {
	return fileIdColumnName
}

func GetDownloadInsertAlertChan() chan map[string]string {
	if downloadQueryStartedAlert == nil {
		downloadQueryStartedAlert = make(chan map[string]string, 1)
	}

	return downloadQueryStartedAlert
}

func GetDownloadQueryFinishedAlert() chan bool {
	if downloadQueryFinishedAlert == nil {
		downloadQueryFinishedAlert = make(chan bool, 1)
	}

	return downloadQueryFinishedAlert
}

func GetDownloadIndexedPathExt(engine interface{}, downloadDataMap map[string]interface{}, indexColumnNames interface{}, databaseName string, tableName string, dbCallBack func(interface{}, map[string]interface{}) (string, []string, [][]interface{}, error)) (string, error) {
	if downloadDataMap["changed"].(int8) == 0 {
		return "invalid - not an actual change", nil
	}
	indexName, idValue, secondIndexName, secondIdValue := "", "", "", ""
	if argosIdentifier, ok := downloadDataMap[indexColumnNames.([]string)[0]].(string); ok {
		indexName = indexColumnNames.([]string)[0]
		idValue = argosIdentifier
	} else {
		return "", errors.New(fmt.Sprintf("fileId not found for download: %v", downloadDataMap))
	}
	if fileId, ok := downloadDataMap[indexColumnNames.([]string)[1]].(string); ok {
		secondIndexName = indexColumnNames.([]string)[1]
		secondIdValue = fileId
	} else {
		return "", errors.New(fmt.Sprintf("argosIdentifier not found for download: %v", downloadDataMap))
	}
	return "/" + indexName + "/" + idValue + "/downloads/" + secondIndexName + "/" + secondIdValue, nil
}

func getDownloadSchema(tableName string) sqle.PrimaryKeySchema {
	timestampDefault, _ := sqle.NewColumnDefaultValue(expression.NewLiteral(time.Now().UTC(), sqle.Timestamp), sqle.Timestamp, true, false, false)
	blankDefault, _ := sqle.NewColumnDefaultValue(expression.NewLiteral("", sqle.Text), sqle.Text, true, false, false)
	boolDefault, _ := sqle.NewColumnDefaultValue(expression.NewLiteral(false, sqle.Boolean), sqle.Boolean, true, false, false)

	return sqle.NewPrimaryKeySchema(sqle.Schema{
		{Name: argosIdentifierColumnName, Type: sqle.Text, Source: tableName, PrimaryKey: true},
		{Name: fileIdColumnName, Type: sqle.Text, Source: tableName, PrimaryKey: true},
		{Name: "type", Type: sqle.Text, Source: tableName, Nullable: true, Default: blankDefault},
		{Name: "targetBlob", Type: sqle.Text, Source: tableName, Nullable: true, Default: blankDefault},
		{Name: "targetSha256", Type: sqle.Text, Source: tableName, Nullable: true, Default: blankDefault},
		{Name: "targetLength", Type: sqle.Text, Source: tableName, Nullable: true, Default: blankDefault},
		{Name: "loaded", Type: sqle.Boolean, Source: tableName, Default: boolDefault},
		{Name: "changed", Type: sqle.Boolean, Source: tableName, Default: boolDefault},
		{Name: "lastModified", Type: sqle.Timestamp, Source: tableName, Default: timestampDefault},
	})
}

func initializeLazyLoadingForDownload(tfmC *flowcore.TrcFlowMachineContext, tfC *flowcore.TrcFlowContext) {
	go func(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) {
		for {
			select {
			case incomingValues := <-GetDownloadInsertAlertChan():
				argosIdentifier, docId := "", ""
				if _, eidOk := incomingValues[argosIdentifierColumnName]; eidOk {
					argosIdentifier = incomingValues[argosIdentifierColumnName]
				} else {
					continue
				}
				if _, didOK := incomingValues[fileIdColumnName]; didOK {
					docId = incomingValues[fileIdColumnName]
				} else {
					continue
				}
				tenantIndexPath, _ := coreopts.BuildOptions.GetDFSPathName()
				azureTokenInsertData := make(map[string]interface{}, 1)
				downloadData, _ := tfContext.GoMod.ReadData("super-secrets/Index/" + tenantIndexPath + "/" + argosIdentifierColumnName + "/" + argosIdentifier + "/downloads/" + fileIdColumnName + "/" + docId + "/Download")
				if downloadData == nil {
					incomingValuesInterface := make(map[string]interface{}, len(incomingValues))
					for k, v := range incomingValues {
						incomingValuesInterface[k] = v
					}
					incomingValuesInterface["loaded"] = "0"
					incomingValuesInterface["changed"] = "1"
					incomingValuesInterface["lastModified"] = time.Now().Format(tcopts.RFC_ISO_8601)
					azureTokenInsertData[GetAzureTokenPrimaryColumnName()] = tcopts.GetArgosIdByArgosIdentifier(tcopts.GetDatabaseName(), tcopts.GetArgosIdTableName(), downloadData[argosIdentifierColumnName].(string))
					azureTokenInsertData[GetAzureTokenSecondColumnName()] = incomingValuesInterface[GetAzureTokenSecondColumnName()]
					tfmContext.CallDBQuery(tfContext, flowsql.GetDownloadUpsert(incomingValuesInterface, tfContext.FlowSourceAlias, tfContext.Flow.TableName(), argosIdentifierColumnName, fileIdColumnName), nil, true, "INSERT", []flowcore.FlowNameType{flowcore.FlowNameType(tfContext.Flow.TableName())}, "")
				} else {
					//Uses vault data as the authority
					downloadData["loaded"] = "1"
					downloadData["changed"] = "0"
					if _, lmOk := downloadData["lastModified"]; !lmOk {
						downloadData["lastModified"] = time.Now().Format(tcopts.RFC_ISO_8601)
					}
					azureTokenInsertData[GetAzureTokenPrimaryColumnName()] = tcopts.GetArgosIdByArgosIdentifier(tcopts.GetDatabaseName(), tcopts.GetArgosIdTableName(), downloadData[argosIdentifierColumnName].(string))
					azureTokenInsertData[GetAzureTokenSecondColumnName()] = downloadData[GetAzureTokenSecondColumnName()]
					tfmContext.CallDBQuery(tfContext, flowsql.GetDownloadUpsert(downloadData, tfContext.FlowSourceAlias, tfContext.Flow.TableName(), argosIdentifierColumnName, fileIdColumnName), nil, false, "UPDATE", nil, "")
				}
				GetAzureTokenInsertAlert(tfmContext.Env) <- azureTokenInsertData
				GetDownloadQueryFinishedAlert() <- true
			}
		}
	}(tfmC, tfC)

	tfC.FlowLock.Lock()
	if tfC.Init { //Alert interface that the table is ready for permissions
		tfmC.PermissionChan <- flowcore.PermissionUpdate{TableName: tfC.Flow.TableName(), CurrentState: tfC.FlowState.State}
		tfC.Init = false
	}
	tfC.FlowLock.Unlock()
}

func downloadflowPushRemote(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) error {
	//TODO: pushonce == query to spectrum instance by that eid, pull images, creeate blobs by eid.
	return nil
}

func GetDownloadUpdateTrigger(databaseName string, tableName string, iden1 string, iden2 string) string {
	return `CREATE TRIGGER tcUpdateTrigger AFTER UPDATE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + iden1 + `,new.` + iden2 + `,current_timestamp());` +
		` END;`
}

func GetDownloadInsertTrigger(databaseName string, tableName string, iden1 string, iden2 string) string {
	return `CREATE TRIGGER tcInsertTrigger AFTER INSERT ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + iden1 + `,new.` + iden2 + `,current_timestamp());` +
		` END;`
}

func prepareDownloadChangeTable(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) {
	tfmContext.GetTableModifierLock().Lock()
	changeTableName := tfContext.Flow.TableName() + "_Changes"
	tfmContext.CallDBQuery(tfContext, map[string]interface{}{"TrcQuery": "DROP TABLE " + tfmContext.TierceronEngine.Database.Name() + "." + changeTableName}, nil, false, "DELETE", nil, "")
	changeTableErr := tfmContext.TierceronEngine.Database.CreateTable(tfmContext.TierceronEngine.Context, changeTableName, sqle.NewPrimaryKeySchema(sqle.Schema{
		{Name: argosIdentifierColumnName, Type: sqle.Text, Source: changeTableName, PrimaryKey: true},
		{Name: fileIdColumnName, Type: sqle.Text, Source: changeTableName, PrimaryKey: true},
		{Name: "updateTime", Type: sqle.Timestamp, Source: changeTableName},
	}),
		flowcore.TableCollationIdGen(changeTableName),
	)

	if changeTableErr != nil {
		tfmContext.Log("Error creating download change table", changeTableErr)
	}
	tfmContext.CreateCompositeTableTriggers(tfContext, argosIdentifierColumnName, fileIdColumnName, GetDownloadInsertTrigger, GetDownloadUpdateTrigger, nil)
	tfmContext.GetTableModifierLock().Unlock()
}

func ProcessDownloadConfigurations(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) error {
	tfmContext.AddTableSchema(getDownloadSchema(tfContext.Flow.TableName()), tfContext)
	prepareDownloadChangeTable(tfmContext, tfContext)
	tfmContext.CreateTableTriggers(tfContext, fileIdColumnName)
	tfContext.FlowLock.Lock()
	tfContext.FlowState = <-tfContext.RemoteDataSource["flowStateController"].(chan flowcorehelper.CurrentFlowState)
	tfContext.FlowState.SyncFilter = "N/A" //Overwrites any changes to syncFilter as this flow doesn't support it
	tfContext.FlowState.SyncMode = "N/A"
	if tfContext.FlowState.State != 1 && tfContext.FlowState.State != 2 {
		tfmContext.PermissionChan <- flowcore.PermissionUpdate{TableName: tfContext.Flow.TableName(), CurrentState: tfContext.FlowState.State}
		tfContext.Init = false
	}
	tfContext.FlowLock.Unlock()

	<-GetAzureTokenFlowReadyForState(tfmContext.Env) //Waiting for Azure Token Flow to be ready for stateUpdate
	azureTokenStateChan := GetAzureTokenController(tfmContext.Env)
	azureTokenStateChan <- tfContext.FlowState

	stateUpdateChannel := tfContext.RemoteDataSource["flowStateReceiver"].(chan flowcorehelper.FlowStateUpdate)

	go func(tfs flowcorehelper.CurrentFlowState, sL *sync.Mutex, aTC chan flowcorehelper.CurrentFlowState, sPC chan flowcorehelper.FlowStateUpdate) {
		sL.Lock()
		previousState := tfs
		sL.Unlock()
		for {
			select {
			case stateUpdate := <-tfContext.RemoteDataSource["flowStateController"].(chan flowcorehelper.CurrentFlowState):
				if previousState.State == stateUpdate.State && previousState.SyncMode == stateUpdate.SyncMode && previousState.SyncFilter == stateUpdate.SyncFilter && previousState.FlowAlias == stateUpdate.FlowAlias {
					continue
				} else if int(previousState.State) != coreopts.BuildOptions.PreviousStateCheck(int(stateUpdate.State)) && stateUpdate.State != previousState.State {
					sPC <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: strconv.Itoa(int(previousState.State)), SyncFilter: stateUpdate.SyncFilter, SyncMode: stateUpdate.SyncMode}
					aTC <- flowcorehelper.CurrentFlowState{State: previousState.State, SyncFilter: stateUpdate.SyncFilter, SyncMode: stateUpdate.SyncMode}
					continue
				}
				stateUpdate.SyncFilter = "N/A"
				stateUpdate.SyncMode = "N/A"
				previousState = stateUpdate
				sL.Lock()
				tfContext.FlowState = stateUpdate
				sL.Unlock()
				aTC <- flowcorehelper.CurrentFlowState{State: stateUpdate.State, SyncFilter: stateUpdate.SyncFilter, SyncMode: stateUpdate.SyncMode}
			}
		}
	}(tfContext.FlowState, tfContext.FlowLock, azureTokenStateChan, stateUpdateChannel)
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
				//Logic for start/stopping flow
				tfContext.FlowLock.Lock()
				if tfContext.FlowState.State == 3 {
					tfContext.FlowLock.Unlock()
					tfContext.Restart = false
					if tfContext.CancelContext != nil {
						tfContext.CancelContext() //This cancel also pushes any final changes to vault before closing sync cycle.
						var baseTableTemplate extract.TemplateResultData
						trcvutils.LoadBaseTemplate(tfmContext.Config, &baseTableTemplate, tfContext.GoMod, tfContext.FlowSource, tfContext.Flow.ServiceName(), tfContext.FlowPath)
						tfContext.FlowData = &baseTableTemplate
					}
					tfmContext.Log("Download flow is being stopped...", nil)
					stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "0", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: tfContext.FlowState.SyncMode}
					continue
				} else if tfContext.FlowState.State == 0 {
					tfContext.FlowLock.Unlock()
					tfmContext.Log("Download flow is currently offline...", nil)
					continue
				} else if tfContext.FlowState.State == 1 {
					tfContext.FlowLock.Unlock()
					tfmContext.Log("Download flow is restarting...", nil)
					syncInit = true
					tfmContext.CallDBQuery(tfContext, map[string]interface{}{"TrcQuery": "truncate " + tfContext.FlowSourceAlias + "." + tfContext.Flow.TableName()}, nil, false, "DELETE", nil, "")
					stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: tfContext.FlowState.SyncMode}
					continue
				} else if tfContext.FlowState.State == 2 {
					tfContext.FlowLock.Unlock()
					if syncInit { //init vault sync cycle
						tfContext.Restart = true //This is to prevent this table from attempting to pull from vault on start up - taken care of by lazy loading.
						go tfmContext.SyncTableCycle(tfContext, argosIdentifierColumnName, []string{getDownloadColumnName(), getDownloadSecondColumnName()}, nil, nil, false)
						initializeLazyLoadingForDownload(tfmContext, tfContext)
						syncInit = false
					}
				}

				tfContext.FlowLock.Lock()
				tfmContext.Log("Download flow is running and checking for changes"+flowcorehelper.SyncCheck(tfContext.FlowState.SyncMode)+".", nil)
				tfContext.FlowLock.Unlock()

			}
		}
	}

	tfContext.CancelContext()
	return nil
}

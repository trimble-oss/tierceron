package flows

import (
	"context"
	"errors"
	"sync"
	"time"

	flowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"
	"github.com/trimble-oss/tierceron/pkg/trcx/extract"

	flowcorehelper "github.com/trimble-oss/tierceron/atrium/trcflow/core/flowcorehelper"
	flowsql "github.com/trimble-oss/tierceron/atrium/vestibulum/trcflow/flows/flowsql"

	sqle "github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/expression"
)

const userIdColumnName = "userId"
const argosIdColumnName = "argosId"

var tokenControllerCache = make(map[string]chan flowcorehelper.CurrentFlowState, 0)
var azureTokenFlowLinkCache = make(map[string]chan bool, 0)
var downloadInsertAlertCache = make(map[string]chan map[string]interface{}, 0)

func GetAzureTokenPrimaryColumnName() string {
	return argosIdColumnName
}

func GetAzureTokenSecondColumnName() string {
	return userIdColumnName
}

func GetAzureTokenInsertAlert(env string) chan map[string]interface{} {
	if _, ok := downloadInsertAlertCache[env]; !ok {
		downloadInsertAlertCache[env] = make(chan map[string]interface{}, 1)
	}
	return downloadInsertAlertCache[env]
}

func GetAzureTokenFlowReadyForState(env string) chan bool {
	if _, ok := azureTokenFlowLinkCache[env]; !ok {
		azureTokenFlowLinkCache[env] = make(chan bool, 1)
	}
	return azureTokenFlowLinkCache[env]
}

func GetAzureTokenIndexedPathExt(engine interface{}, downloadDataMap map[string]interface{}, indexColumnNames interface{}, databaseName string, tableName string, dbCallBack func(interface{}, map[string]interface{}) (string, []string, [][]interface{}, error)) (string, error) {
	return "", nil
}

func getAzureTokenSchema(tableName string) sqle.PrimaryKeySchema {
	timestampDefault, _ := sqle.NewColumnDefaultValue(expression.NewLiteral(time.Now().UTC(), sqle.Timestamp), sqle.Timestamp, true, false, false)
	blankDefault, _ := sqle.NewColumnDefaultValue(expression.NewLiteral("", sqle.Text), sqle.Text, true, false, false)

	return sqle.NewPrimaryKeySchema(sqle.Schema{
		{Name: argosIdColumnName, Type: sqle.Text, Source: tableName, PrimaryKey: true},
		{Name: userIdColumnName, Type: sqle.Text, Source: tableName, PrimaryKey: true},
		{Name: "accessKey", Type: sqle.Text, Source: tableName, Nullable: true, Default: blankDefault},
		{Name: "permission", Type: sqle.Text, Source: tableName, Nullable: true, Default: blankDefault},
		{Name: "keyExpiration", Type: sqle.Text, Source: tableName, Nullable: true, Default: blankDefault},
		{Name: "lastModified", Type: sqle.Timestamp, Source: tableName, Default: timestampDefault},
	})
}

func prepareAzureTokenChangeTable(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) {
	tfmContext.GetTableModifierLock().Lock()
	changeTableName := tfContext.Flow.TableName() + "_Changes"
	tfmContext.CallDBQuery(tfContext, map[string]interface{}{"TrcQuery": "DROP TABLE " + tfmContext.TierceronEngine.Database.Name() + "." + changeTableName}, nil, false, "DELETE", nil, "")
	tfmContext.GetTableModifierLock().Unlock()
}

func GetAzureTokenController(env string) chan flowcorehelper.CurrentFlowState {
	return tokenControllerCache[env]
}

func ProcessAzureTokensConfigurations(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) error {
	tfmContext.AddTableSchema(getAzureTokenSchema(tfContext.Flow.TableName()), tfContext)
	prepareAzureTokenChangeTable(tfmContext, tfContext)
	tfContext.ReadOnly = true
	tfContext.Context, tfContext.CancelContext = context.WithCancel(context.Background()) //unique to this flow bc no sync
	tfContext.FlowLock.Lock()
	tokenControllerCache[tfmContext.Env] = tfContext.RemoteDataSource["flowStateController"].(chan flowcorehelper.CurrentFlowState)
	GetAzureTokenFlowReadyForState(tfmContext.Env) <- true

	tfContext.FlowState = <-tfContext.RemoteDataSource["flowStateController"].(chan flowcorehelper.CurrentFlowState)
	if tfContext.FlowState.State != 1 && tfContext.FlowState.State != 2 {
		tfmContext.PermissionChan <- flowcore.PermissionUpdate{TableName: tfContext.Flow.TableName(), CurrentState: tfContext.FlowState.State}
		tfContext.Init = false
	}
	tfContext.FlowLock.Unlock()

	go func(tfs flowcorehelper.CurrentFlowState, sL *sync.Mutex) {
		sL.Lock()
		previousState := tfs
		sL.Unlock()
		for {
			select {
			case stateUpdate := <-tfContext.RemoteDataSource["flowStateController"].(chan flowcorehelper.CurrentFlowState):
				if previousState.State == stateUpdate.State && previousState.SyncMode == stateUpdate.SyncMode && previousState.SyncFilter == stateUpdate.SyncFilter && previousState.FlowAlias == stateUpdate.FlowAlias {
					continue
				} else if int(previousState.State) != coreopts.BuildOptions.PreviousStateCheck(int(stateUpdate.State)) && stateUpdate.State != previousState.State {
					tfContext.RemoteDataSource["flowStateController"].(chan flowcorehelper.CurrentFlowState) <- flowcorehelper.CurrentFlowState{State: previousState.State, SyncFilter: stateUpdate.SyncFilter, SyncMode: stateUpdate.SyncMode}
					continue
				}
				stateUpdate.SyncFilter = "N/A"
				stateUpdate.SyncMode = "N/A"
				previousState = stateUpdate
				sL.Lock()
				tfContext.FlowState = stateUpdate
				sL.Unlock()
			}
		}
	}(tfContext.FlowState, tfContext.FlowLock)

	stateUpdateChannel := tfContext.RemoteDataSource["flowStateController"].(chan flowcorehelper.CurrentFlowState) //Flow is sent to go routine - not controller like other flows

	sqlIngestInterval := tfContext.RemoteDataSource["dbingestinterval"].(time.Duration)
	initInsertListener := false
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
						tfContext.CancelContext()
						var baseTableTemplate extract.TemplateResultData
						trcvutils.LoadBaseTemplate(tfmContext.Config, &baseTableTemplate, tfContext.GoMod, tfContext.FlowSource, tfContext.Flow.ServiceName(), tfContext.FlowPath)
						tfContext.FlowData = &baseTableTemplate
					}
					tfmContext.Log("Azure Token flow is being stopped...", nil)
					stateUpdateChannel <- flowcorehelper.CurrentFlowState{State: 0, SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: tfContext.FlowState.SyncMode}
					continue
				} else if tfContext.FlowState.State == 0 {
					tfContext.FlowLock.Unlock()
					tfmContext.Log("Azure Token flow is currently offline...", nil)
					continue
				} else if tfContext.FlowState.State == 1 {
					tfContext.FlowLock.Unlock()
					tfmContext.Log("Azure Token flow is restarting...", nil)
					tfmContext.CallDBQuery(tfContext, map[string]interface{}{"TrcQuery": "truncate " + tfContext.FlowSourceAlias + "." + tfContext.Flow.TableName()}, nil, false, "DELETE", nil, "")
					stateUpdateChannel <- flowcorehelper.CurrentFlowState{State: 2, SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: tfContext.FlowState.SyncMode}
					initInsertListener = true
					continue
				} else if tfContext.FlowState.State == 2 {
					if initInsertListener {
						go func(tfmC *flowcore.TrcFlowMachineContext, tfC *flowcore.TrcFlowContext) {
							for {
								select {
								case incomingValues := <-GetAzureTokenInsertAlert(tfmC.Env):
									var tenantId string
									var operatorCode string
									if val, ok := incomingValues[GetAzureTokenPrimaryColumnName()].(string); ok {
										tenantId = val
									}

									if val, ok := incomingValues[GetAzureTokenSecondColumnName()].(string); ok {
										operatorCode = val
									}

									if tenantId == "" {
										tfmContext.Log("Could not find tenantId for requested download", errors.New("Could not find tenantId for requested download"))
										continue
									}

									if operatorCode == "" {
										tfmContext.Log("Could not find operatorCode for requested download for "+tenantId, errors.New("Could not find operatorCode for requested download for "+tenantId))
										continue
									}

									tfmContext.CallDBQuery(tfContext, flowsql.GetAzureTokenUpsert(incomingValues, tfContext.FlowSourceAlias, tfContext.Flow.TableName(), GetAzureTokenPrimaryColumnName(), GetAzureTokenSecondColumnName()), nil, true, "INSERT", []flowcore.FlowNameType{flowcore.FlowNameType(tfContext.Flow.TableName())}, "")
								case <-tfC.Context.Done():
									return
								}
							}
						}(tfmContext, tfContext)
						initInsertListener = false
					}

					tfContext.FlowLock.Lock()
					if tfContext.Init { //This table is excluded on interface, but alert is needed for start up.
						tfmContext.PermissionChan <- flowcore.PermissionUpdate{TableName: tfContext.Flow.TableName(), CurrentState: tfContext.FlowState.State}
						tfContext.Init = false
					}
					tfContext.FlowLock.Unlock()
				}
			}

			tfmContext.Log("Azure Token flow is running.", nil)
		}
	}
	tfContext.CancelContext()
	return nil
}

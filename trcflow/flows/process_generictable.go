package flows

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	flowcore "tierceron/trcflow/core"
	trcvutils "tierceron/trcvault/util"
	"tierceron/trcx/extract"
	"time"

	flowcorehelper "tierceron/trcflow/core/flowcorehelper"

	"VaultConfig.TenantConfig/lib"
	sqle "github.com/dolthub/go-mysql-server/sql"
)

const TableIdColumnName = "jobId"

type TrcTable struct {
	DatabaseName  string
	TableName     string
	TableTemplate string
	TableSchema   sqle.Schema
}

func InitTable(databaseName string, tableName string, tableTemplate string) *TrcTable {
	return &TrcTable{DatabaseName: databaseName, TableName: tableName, TableTemplate: tableTemplate}
}
func CheckPrimaryColumn(m map[string]interface{}, eids [][]interface{}) (string, interface{}) {
	var idValue interface{}
	if len(eids) != 0 { //If spectrum enterprise
		if _, ok := m["tenantId"].(string); !ok { //tenant DNE -> Id = queried tenantId
			idValue = eids[0][0]
		}
	} else {
		if _, ok := m["tenantId"].(string); ok {
			idValue = m["tenantId"].(string) //else tenant configuration -> Id = in tenant already
		}
	}
	return tenantConfigTenantIdColumnName, idValue
}

func getIndexedPathExt(engine interface{}, rowDataMap map[string]interface{}, vaultIndexColumnName string, databaseName string, tableName string, dbCallBack func(interface{}, map[string]string) (string, []string, [][]interface{}, error)) (string, error) {
	var matrixRows [][]interface{}

	if _, iOk := rowDataMap[vaultIndexColumnName].(int64); iOk {
		var err error
		eid := fmt.Sprintf("%d", rowDataMap[vaultIndexColumnName])
		_, _, matrixRows, err = dbCallBack(engine, GetTenantIdByEnterpriseId(databaseName, coreopts.GetTenantDBName(), eid))

		if err != nil {
			return "", err
		}
	}

	indexName, idValue := CheckPrimaryColumn(rowDataMap, matrixRows)
	if len(matrixRows) == 0 && idValue == "" {
		return "", errors.New(fmt.Sprintf("tenantId not found for enterprise: %v", rowDataMap[vaultIndexColumnName]))
	}
	var idValueStr string
	if _, iOk := idValue.(int64); iOk {
		idValueStr = fmt.Sprintf("%d", idValue)
	} else if _, iOk := idValue.(int8); iOk {
		idValueStr = fmt.Sprintf("%d", idValue)
	} else if _, iOk := idValue.(int64); iOk {
		idValueStr = fmt.Sprintf("%d", idValue)
	} else if idValue == nil {
		return "", errors.New(fmt.Sprintf("tenantId not found for enterprise: %v", rowDataMap[vaultIndexColumnName]))
	} else {
		idValueStr = fmt.Sprintf("%s", idValue)
	}
	return "/" + indexName + "/" + idValueStr, nil
}

// True == equal, false = not equal
func CompareRows(a map[string]interface{}, b map[string]interface{}) bool {
	for key, value := range a {
		if b[key] != value {
			return false
		}
	}
	return true
}

func getTablePrimaryColumnName() string {
	return TableIdColumnName
}

func GetTableRowById(databaseName string, project string, Id string) map[string]string {
	return map[string]string{"TrcQuery": fmt.Sprintf("SELECT * FROM %s.%s WHERE %s = '%s'", databaseName, project, TableIdColumnName, Id)}
}

func GetTableIndexedPathExt(engine interface{}, rowDataMap map[string]interface{}, vaultIndexColumnName string, databaseName string, tableName string, dbCallBack func(interface{}, map[string]string) (string, []string, [][]interface{}, error)) (string, error) {
	indexName, idValue := "", ""
	if columnIdValue, ok := rowDataMap[vaultIndexColumnName].(string); ok {
		indexName = vaultIndexColumnName
		idValue = columnIdValue
	} else {
		return "", errors.New(fmt.Sprintf("%s not found for Table: %v", TableIdColumnName, rowDataMap[vaultIndexColumnName]))
	}
	return "/" + indexName + "/" + idValue, nil
}

func getTableSchema(tableName string) sqle.PrimaryKeySchema {
	return sqle.NewPrimaryKeySchema(sqle.Schema{
		{Name: TableIdColumnName, Type: sqle.Text, Source: tableName, PrimaryKey: true},
		{Name: "tenantId", Type: sqle.Text, Source: tableName},
		{Name: "operatorCode", Type: sqle.Text, Source: tableName},
		{Name: "companyCode", Type: sqle.Text, Source: tableName},
		{Name: "workOrderCode", Type: sqle.Text, Source: tableName},
		{Name: "submitTime", Type: sqle.Timestamp, Source: tableName},
		{Name: "startTime", Type: sqle.Timestamp, Source: tableName},
		{Name: "endTime", Type: sqle.Timestamp, Source: tableName},
		{Name: "reportSessionId", Type: sqle.Text, Source: tableName},
		{Name: "diTransactionId", Type: sqle.Text, Source: tableName},
		{Name: "errorMessage", Type: sqle.Text, Source: tableName},
	})
}

func TableFlowPullRemote(sqlConn *sql.DB) ([]map[string]interface{}, error) {
	// b. Retrieve tenant configurations
	var tableRowMapArr []map[string]interface{}
	if sqlConn == nil {
		return tableRowMapArr, nil
	}

	tableRows, err := lib.GetTable(sqlConn)
	if err != nil {
		return nil, err
	}

	for _, tableRow := range tableRows {
		tableRowMapArr = append(tableRowMapArr, lib.GetTableRowMap(tableRow))
	}

	return tableRowMapArr, nil
}

func TableFlowPushRemote(tfContext *flowcore.TrcFlowContext, trcRemoteDataSource map[string]interface{}, changedItem map[string]interface{}) error {
	sqlIngestInterval := trcRemoteDataSource["dbingestinterval"].(time.Duration)
	sqlConn := trcRemoteDataSource["connection"].(*sql.DB)
	tfContext.FlowLock.Lock()
	if sqlIngestInterval > 0 && (tfContext.FlowState.SyncMode == "push" || tfContext.FlowState.SyncMode == "pushonce") {
		tableRow := lib.GetTableRowFromMap(changedItem)
		if tfContext.FlowState.SyncFilter != "" {
			syncFilter := strings.Split(strings.ReplaceAll(tfContext.FlowState.SyncFilter, " ", ""), ",")
			tfContext.FlowLock.Unlock()
			for _, filter := range syncFilter {
				if filter == tableRow.JobID {
					err := tableRow.Upsert(sqle.NewEmptyContext(), sqlConn) //This is the writeback for spectrumEnterprise...for now?
					if err != nil {
						tfContext.RemoteDataSource["flowStateReceiver"].(chan flowcorehelper.FlowStateUpdate) <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: "pusherror"}
						return err
					}
				}
			}
		} else {
			tfContext.FlowLock.Unlock()
			err := tableRow.Upsert(sqle.NewEmptyContext(), sqlConn) //This is the writeback for spectrumEnterprise...for now?
			if err != nil {
				tfContext.RemoteDataSource["flowStateReceiver"].(chan flowcorehelper.FlowStateUpdate) <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: "pusherror"}
				return err
			}
		}
	}
	return nil
}

func ProcessTableConfigurations(tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) error {
	tfmContext.AddTableSchema(getTableSchema(tfContext.Flow.TableName()), tfContext)
	tfmContext.CreateTableTriggers(tfContext, TableIdColumnName)
	tfContext.FlowLock.Lock()
	tfContext.FlowState = <-tfContext.RemoteDataSource["flowStateController"].(chan flowcorehelper.CurrentFlowState)
	if tfContext.FlowState.State != 1 && tfContext.FlowState.State != 2 {
		tfmContext.InitConfigWG.Done()
	}
	tfContext.FlowLock.Unlock()
	go func(tfs flowcorehelper.CurrentFlowState, sL *sync.Mutex) {
		for {
			select {
			case stateUpdate := <-tfContext.RemoteDataSource["flowStateController"].(chan flowcorehelper.CurrentFlowState):
				sL.Lock()
				tfContext.FlowState = stateUpdate
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
				var sqlConn *sql.DB
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
					tfmContext.Log("Table flow is being stopped...", nil)
					stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "0", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: tfContext.FlowState.SyncMode}
					continue
				} else if tfContext.FlowState.State == 0 {
					tfContext.FlowLock.Unlock()
					tfmContext.Log("Table flow is currently offline...", nil)
					continue
				} else if tfContext.FlowState.State == 1 {
					tfContext.FlowLock.Unlock()
					tfmContext.Log("Table flow is restarting...", nil)
					syncInit = true
					tfmContext.CallDBQuery(tfContext, map[string]string{"TrcQuery": "truncate " + tfContext.FlowSourceAlias + "." + tfContext.Flow.TableName()}, nil, false, "DELETE", nil, "")
					stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: tfContext.FlowState.SyncMode}
					continue
				} else if tfContext.FlowState.State == 2 {
					tfContext.FlowLock.Unlock()
					if syncInit { //init vault sync cycle
						go tfmContext.SyncTableCycle(tfContext, TableIdColumnName, TableIdColumnName, "", getIndexedPathExt, TableFlowPushRemote, tfContext.FlowState.SyncMode == "push")
						syncInit = false
					}
				}

				tfContext.FlowLock.Lock()
				if tfContext.FlowState.State != 0 && (tfContext.FlowState.SyncMode == "pull" || tfContext.FlowState.SyncMode == "pullonce") {
					sqlConn = tfContext.RemoteDataSource["connection"].(*sql.DB)
				} else if tfContext.FlowState.State != 0 && (tfContext.FlowState.SyncMode == "push" || tfContext.FlowState.SyncMode == "pushonce") {
					sqlConn = nil
				} else {
					tfmContext.Log("Table is setup"+flowcorehelper.SyncCheck(tfContext.FlowState.SyncMode)+".", nil)
					tfContext.FlowLock.Unlock()
					continue
				}

				tfmContext.Log("Table is running and checking for changes"+flowcorehelper.SyncCheck(tfContext.FlowState.SyncMode)+".", nil)
				tfContext.FlowLock.Unlock()

				//Logic for push/pull once
				tfContext.FlowLock.Lock()
				if tfContext.FlowState.SyncMode == "pushonce" {
					tfContext.FlowLock.Unlock()
					rows := tfmContext.CallDBQuery(tfContext, map[string]string{"TrcQuery": "SELECT * FROM " + tfContext.FlowSourceAlias + "." + tfContext.Flow.TableName()}, nil, false, "SELECT", nil, "")
					if len(rows) == 0 {
						tfmContext.Log("Nothing in Table table to push out yet...", nil) //Table is not currently loaded.
						continue
					}
					for _, value := range rows {
						tenantMap := lib.GetTableRowMapFromArray(value)
						pushError := TableFlowPushRemote(tfContext, tfContext.RemoteDataSource, tenantMap)
						if pushError != nil {
							tfmContext.Log("Error pushing out Table", pushError)
							continue
						}
					}
					tfContext.FlowLock.Lock()
					tfContext.FlowState.SyncMode = "pushcomplete"
					stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: "pushcomplete"}
					tfContext.FlowLock.Unlock()
				} else {
					tfContext.FlowLock.Unlock()
				}

				// 3. Retrieve tenant configurations from mysql.
				Table, err := TableFlowPullRemote(sqlConn)
				if err != nil {
					tfmContext.Log("Error grabbing table data", err)
					stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: "pullerror"}
					continue
				}

				var filterTable []map[string]interface{}
				tfContext.FlowLock.Lock()
				if tfContext.FlowState.SyncFilter != "" {
					syncFilter := strings.Split(strings.ReplaceAll(tfContext.FlowState.SyncFilter, " ", ""), ",")
					for _, filter := range syncFilter {
						for _, tableRow := range Table {
							if filter == tableRow[TableIdColumnName].(string) {
								filterTable = append(filterTable, tableRow)
							}
						}
					}
					Table = filterTable
				}
				tfContext.FlowLock.Unlock()

				for _, tableRow := range Table {
					rows := tfmContext.CallDBQuery(tfContext, GetTableRowById(tfContext.FlowSourceAlias, tfContext.Flow.TableName(), tableRow[TableIdColumnName].(string)), nil, false, "SELECT", nil, "")
					if len(rows) == 0 {
						tfmContext.CallDBQuery(tfContext, lib.GetTableRowInsert(tableRow, tfContext.FlowSourceAlias, tfContext.Flow.TableName()), nil, true, "INSERT", []flowcore.FlowNameType{flowcore.FlowNameType(tfContext.Flow.TableName())}, "") //if DNE -> insert
					} else {
						for _, value := range rows {
							// tenantConfig is db, value is what's in vault...
							if CompareRows(tableRow, lib.GetTableRowMapFromArray(value)) { //If equal-> do nothing
								continue
							} else { //If not equal -> update
								tfmContext.CallDBQuery(tfContext, lib.GetTableRowUpdate(tableRow, tfContext.FlowSourceAlias, tfContext.Flow.TableName()), nil, true, "UPDATE", []flowcore.FlowNameType{flowcore.FlowNameType(tfContext.Flow.TableName())}, "")
							}
						}
					}
				}

				tfContext.FlowLock.Lock()
				if tfContext.FlowState.SyncMode == "pullonce" {
					tfContext.FlowState.SyncMode = "pullcomplete"
					go func(ftn string, sf string) {
						stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: ftn, StateUpdate: "2", SyncFilter: sf, SyncMode: "pullcomplete"}
					}(tfContext.Flow.TableName(), tfContext.FlowState.SyncFilter)
					// Now go to vault.
					//tfContext.Restart = true
					//tfContext.CancelContext() // Anti pattern...
				}
				tfContext.FlowLock.Unlock()
			}
		}
	}
	tfContext.CancelContext()
	return nil
}

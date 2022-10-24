package core

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"VaultConfig.TenantConfig/lib"

	"tierceron/trcflow/core/flowcorehelper"
	trcvutils "tierceron/trcvault/util"
	"tierceron/trcx/extract"

	sys "tierceron/vaulthelper/system"

	helperkv "tierceron/vaulthelper/kv"

	sqle "github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/expression"
	sqlee "github.com/dolthub/go-mysql-server/sql/expression"
	"github.com/dolthub/vitess/go/sqltypes"
)

type TrcTable struct {
	TableName     string
	TableTemplate string
}

type TrcFlowContext struct {
	RemoteDataSource map[string]interface{}
	GoMod            *helperkv.Modifier
	Vault            *sys.Vault

	// Recommended not to store contexts, but because we
	// are working with flows, this is a different pattern.
	// This just means some analytic tools won't be able to
	// perform analysis which are based on the Context.
	ContextNotifyChan chan bool
	Context           context.Context
	CancelContext     context.CancelFunc

	FlowSource      string       // The name of the flow source identified by project.
	FlowSourceAlias string       // May be a database name
	Flow            FlowNameType // May be a table name.
	ChangeIdKey     string       // Name of id column
	FlowPath        string
	FlowData        interface{}
	ChangeFlowName  string // Change flow table name.
	FlowState       flowcorehelper.CurrentFlowState
	FlowLock        *sync.Mutex //This is for sync concurrent changes to FlowState
	Restart         bool
	ReadOnly        bool
	TrcDbReady      bool // Flow has been loaded into a table in TrcDb.
}

func TableCollationIdGen(tableName string) sqle.CollationID {
	return sqle.CollationID(sqle.Collation_utf8mb4_unicode_ci)
}

// True == equal, false = not equal
func CompareLastModified(a map[string]interface{}, b map[string]interface{}) bool {
	//Check if a & b are time.time
	//Check if they match.
	var lastModifiedA time.Time
	var lastModifiedB time.Time
	var timeErr error
	if lastMA, ok := a["lastModified"].(time.Time); !ok {
		if lmA, ok := a["lastModified"].(string); ok {
			lastModifiedA, timeErr = time.Parse(lib.RFC_ISO_8601, lmA)
			if timeErr != nil {
				return false
			}
		}
	} else {
		lastModifiedA = lastMA
	}

	if lastMB, ok := b["lastModified"].(time.Time); !ok {
		if lmB, ok := b["lastModified"].(string); ok {
			lastModifiedB, timeErr = time.Parse(lib.RFC_ISO_8601, lmB)
			if timeErr != nil {
				return false
			}
		}
	} else {
		lastModifiedB = lastMB
	}

	if lastModifiedA != lastModifiedB {
		return false
	}

	return true
}

func (tfContext *TrcFlowContext) GetTableIdIndexedPathExt(engine interface{}, rowDataMap map[string]interface{}, vaultIndexColumnName string, databaseName string, tableName string, dbCallBack func(interface{}, string) (string, []string, [][]string, error)) (string, error) {
	indexName, idValue := "", ""
	if mysqlFilePath, ok := rowDataMap[vaultIndexColumnName].(string); ok {
		indexName = vaultIndexColumnName
		idValue = mysqlFilePath
	} else {
		return "", errors.New("mysqlFilePath not found for MysqlFile: " + rowDataMap[vaultIndexColumnName].(string))
	}
	return "/" + indexName + "/" + idValue, nil
}

func (tfContext *TrcFlowContext) GetSelectById(Id string) map[string]string {
	return map[string]string{"TrcQuery": "SELECT * FROM " + tfContext.FlowSourceAlias + "." + tfContext.Flow.TableName() + " WHERE MysqlFilePath = '" + Id + "'"}
}

func (tfContext *TrcFlowContext) GetInsertQueryWithBindings(te *TrcFlowMachineContext, mysqlFileConfigHandle map[string]interface{}) (map[string]string, map[string]sqle.Expression) {
	// if !strings.HasSuffix(string(mysqlFileConfigHandle["MysqlFileContent"].([]byte)), "==") {
	// 	mysqlFileConfigHandle["MysqlFileContent"] = base64.StdEncoding.EncodeToString(mysqlFileConfigHandle["MysqlFileContent"].([]byte))
	// }
	query := map[string]string{"TrcQuery": `INSERT IGNORE INTO ` + tfContext.FlowSourceAlias + `.` + tfContext.Flow.TableName() + `(MysqlFilePath, MysqlFileContent, lastModified) VALUES (:path,:content,:date)`, "TrcChangeId": mysqlFileConfigHandle["MysqlFilePath"].(string)}
	bindings := map[string]sqle.Expression{
		"path":    sqlee.NewLiteral(mysqlFileConfigHandle["MysqlFilePath"].(string), sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
		"content": sqlee.NewLiteral(mysqlFileConfigHandle["MysqlFileContent"].([]byte), sqle.MustCreateBinary(sqltypes.Blob, 16383)), //16383 -> Max length for a string in this DB
		"date":    sqlee.NewLiteral(mysqlFileConfigHandle["lastModified"].(string), sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
	}

	return query, bindings
}

func (tfContext *TrcFlowContext) GetUpdateQueryWithBindings(te *TrcFlowMachineContext, mysqlFileConfigHandle map[string]interface{}) (map[string]string, map[string]sqle.Expression) {
	query := map[string]string{
		"TrcQuery":    `UPDATE ` + tfContext.FlowSourceAlias + `.` + tfContext.Flow.TableName() + ` SET MysqlFileContent=(:content), lastModified=(:date) WHERE MysqlFilePath=(:path)`,
		"TrcChangeId": mysqlFileConfigHandle["MysqlFilePath"].(string),
	}
	bindings := map[string]sqle.Expression{
		"path":    sqlee.NewLiteral(mysqlFileConfigHandle["MysqlFilePath"].(string), sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
		"content": sqlee.NewLiteral(mysqlFileConfigHandle["MysqlFileContent"].([]byte), sqle.MustCreateBinary(sqltypes.Blob, 16383)), //16383 -> Max length for a string in this DB
		"date":    sqlee.NewLiteral(mysqlFileConfigHandle["lastModified"].(string), sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
	}

	return query, bindings
}

func (tfContext *TrcFlowContext) GetTableIndexedPathExt(engine interface{}, rowDataMap map[string]interface{}, vaultIndexColumnName string, databaseName string, tableName string, dbCallBack func(interface{}, map[string]string) (string, []string, [][]interface{}, error)) (string, error) {
	vaultPath := ""
	if MysqlFilePath, ok := rowDataMap[vaultIndexColumnName].(string); ok {
		vaultPath = MysqlFilePath
		vaultPath = strings.Replace(vaultPath, "store", "enterpriseId", -1)
		vaultPath = vaultPath[:strings.Index(vaultPath, ".")]
		vaultPath = vaultPath[:strings.LastIndex(vaultPath, "/")+1] + tableName + vaultPath[strings.LastIndex(vaultPath, "/"):]
		if strContent, stringOK := rowDataMap["MysqlFileContent"].(string); stringOK {
			if !strings.HasPrefix(rowDataMap["MysqlFileContent"].(string), "TierceronBase64") {
				rowDataMap["MysqlFileContent"] = "TierceronBase64" + base64.StdEncoding.EncodeToString([]byte(strContent))
			}
		} else if byteContent, byteOK := rowDataMap["MysqlFileContent"].([]uint8); byteOK {
			rowDataMap["MysqlFileContent"] = "TierceronBase64" + base64.StdEncoding.EncodeToString(byteContent)
		} else {
			return "", errors.New(fmt.Sprintf("Found an incompitable type for MysqlFileContent: %v", rowDataMap[vaultIndexColumnName]))
		}
	} else {
		return "", errors.New(fmt.Sprintf("MysqlFilePath not found for MysqlFile: %v", rowDataMap[vaultIndexColumnName]))
	}
	return vaultPath, nil
}

func (tfContext *TrcFlowContext) GetTableSchema() sqle.PrimaryKeySchema {
	timestampDefault, _ := sqle.NewColumnDefaultValue(expression.NewLiteral(time.Now().UTC(), sqle.Timestamp), sqle.Timestamp, true, false, false)
	return sqle.NewPrimaryKeySchema(sqle.Schema{
		{Name: tfContext.ChangeIdKey, Type: sqle.Text, Source: string(tfContext.Flow.TableName()), PrimaryKey: true},
		{Name: "MysqlFileContent", Type: sqle.MediumBlob, Source: string(tfContext.Flow.TableName())},
		{Name: "lastModified", Type: sqle.Timestamp, Source: string(tfContext.Flow.TableName()), Default: timestampDefault},
	},
	)
}

func (tfContext *TrcFlowContext) TableFlowPushRemote(changedItem map[string]interface{}) error {
	sqlIngestInterval := tfContext.RemoteDataSource["dbingestinterval"].(time.Duration)
	tfContext.FlowLock.Lock()
	if sqlIngestInterval > 0 && (tfContext.FlowState.SyncMode == "push" || tfContext.FlowState.SyncMode == "pushonce") {
		tfContext.FlowLock.Unlock()
		var err error
		mysqlFile := lib.GetMysqlFileMapFromMap(changedItem)
		tfContext.FlowLock.Lock()
		if tfContext.FlowState.SyncFilter != "" {
			syncFilter := strings.Split(strings.ReplaceAll(tfContext.FlowState.SyncFilter, " ", ""), ",")
			tfContext.FlowLock.Unlock()
			for _, filter := range syncFilter {
				if strings.Contains(mysqlFile.MysqlFilePath, filter) {
					tempContent, uncompressErr := lib.UncompressBytes(mysqlFile.MysqlFileContent) //uncompress content for writeback
					if uncompressErr != nil {
						if uncompressErr.Error() != "gzip: invalid header" {
							tfContext.RemoteDataSource["flowStateReceiver"].(chan flowcorehelper.FlowStateUpdate) <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: "pusherror"}
							return uncompressErr
						}
					} else {
						mysqlFile.MysqlFileContent = tempContent
					}

					if sqlConn, sqlConnOk := tfContext.RemoteDataSource["connection"].(lib.DB); sqlConnOk {
						err = mysqlFile.Upsert(sqle.NewEmptyContext(), sqlConn) //This is the writeback for spectrumEnterprise...for now?
						if err != nil {
							tfContext.RemoteDataSource["flowStateReceiver"].(chan flowcorehelper.FlowStateUpdate) <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: "pusherror"}
							return err
						}
					}
				}
			}
		} else {
			tfContext.FlowLock.Unlock()
			mysqlFile.MysqlFileContent, err = lib.UncompressBytes(mysqlFile.MysqlFileContent) //uncompress content for writeback
			if err != nil {
				tfContext.RemoteDataSource["flowStateReceiver"].(chan flowcorehelper.FlowStateUpdate) <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: "pusherror"}
				return err
			}
			if sqlConn, sqlConnOk := tfContext.RemoteDataSource["connection"].(lib.DB); sqlConnOk {
				err = mysqlFile.Upsert(sqle.NewEmptyContext(), sqlConn)
				tfContext.RemoteDataSource["flowStateReceiver"].(chan flowcorehelper.FlowStateUpdate) <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: "pusherror"}
				if err != nil {
					return err
				}
			}
		}

	}
	return nil
}

func (tfContext *TrcFlowContext) ProcessTable(tfmContext *TrcFlowMachineContext) error {
	tfmContext.AddTableSchema(tfContext.GetTableSchema(), tfContext)
	tfmContext.CreateTableTriggers(tfContext)

	tfContext.FlowLock.Lock()
	tfContext.FlowState = <-tfContext.RemoteDataSource["flowStateController"].(chan flowcorehelper.CurrentFlowState)
	tfContext.FlowLock.Unlock()
	go func(msfs flowcorehelper.CurrentFlowState, sL *sync.Mutex) {
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
				var sqlConn lib.DB
				afterTime = sqlIngestInterval
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
					tfmContext.Log("MysqlFile flow is being stopped...", nil)
					stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "0", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: tfContext.FlowState.SyncMode}
					continue
				} else if tfContext.FlowState.State == 0 {
					tfContext.FlowLock.Unlock()
					tfmContext.Log("MysqlFile flow is currently offline...", nil)
					continue
				} else if tfContext.FlowState.State == 1 {
					tfContext.FlowLock.Unlock()
					tfmContext.Log("MysqlFile flow is restarting...", nil)
					syncInit = true
					tfmContext.CallDBQuery(tfContext, map[string]string{"TrcQuery": "truncate " + tfContext.FlowSourceAlias + "." + tfContext.Flow.TableName()}, nil, false, "DELETE", nil, "")
					stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: tfContext.FlowState.SyncMode}
					continue
				} else if tfContext.FlowState.State == 2 {
					tfContext.FlowLock.Unlock()
					if syncInit { //init vault sync cycle
						go tfmContext.SyncTableCycle(tfContext, tfContext.ChangeIdKey, tfContext.ChangeIdKey, "", tfContext.FlowState.SyncMode == "push")
						syncInit = false
					}
				}

				tfContext.FlowLock.Lock()
				if tfContext.FlowState.State != 0 && (tfContext.FlowState.SyncMode == "pull" || tfContext.FlowState.SyncMode == "pullonce") {
					sqlConn = tfContext.RemoteDataSource["connection"].(lib.DB)
				} else if tfContext.FlowState.State != 0 && (tfContext.FlowState.SyncMode == "push" || tfContext.FlowState.SyncMode == "pushonce") {
					sqlConn = nil
				} else {
					tfmContext.Log("MysqlFile is setup"+SyncCheck(tfContext.FlowState.SyncMode)+".", nil)
					tfContext.FlowLock.Unlock()
					continue
				}

				tfmContext.Log("MysqlFile is running and checking for changes"+SyncCheck(tfContext.FlowState.SyncMode)+".", nil)
				tfContext.FlowLock.Unlock()

				//Logic for push/pull once
				tfContext.FlowLock.Lock()
				if tfContext.FlowState.SyncMode == "pushonce" {
					tfContext.FlowLock.Unlock()
					rows := tfmContext.CallDBQuery(tfContext, map[string]string{"TrcQuery": "SELECT * FROM " + tfContext.FlowSourceAlias + "." + tfContext.Flow.TableName()}, nil, false, "SELECT", nil, "")
					if len(rows) == 0 {
						tfmContext.Log("Nothing in MysqlFile table to push out yet...", nil) //Table is not currently loaded.
						continue
					}
					for _, value := range rows {
						mysqlFileMap := lib.GetMysqlFileMapFromArray(value)
						pushError := tfContext.TableFlowPushRemote(mysqlFileMap)
						if pushError != nil {
							tfContext.FlowLock.Lock()
							tfContext.FlowState.SyncMode = "pusherror"
							stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: "pusherror"}
							tfContext.FlowLock.Unlock()
							tfmContext.Log("Error pushing out MysqlFile", pushError)
							continue
						}
					}
					tfContext.FlowLock.Lock()
					tfContext.FlowState.SyncMode = "pushcomplete"
					stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: "pushcomplete"}
					tfContext.FlowLock.Unlock()
					continue
				} else {
					tfContext.FlowLock.Unlock()
				}
				// 3. Retrieve mysqlfile from local tables (data from vault) and check for changes in mysql.
				var changedEntries []string
				var existingEntries []string
				rows := tfmContext.CallDBQuery(tfContext, lib.GetLocalMysqlFiles(tfContext.FlowSourceAlias, tfContext.Flow.TableName()), nil, false, "SELECT", nil, "")
				if len(rows) > 0 {
					for _, mysqlFileArray := range rows {
						mysqlFile := lib.GetMysqlFileMapFromArray(mysqlFileArray)
						var mysqlChangeCheck int
						if sqlConn != nil {
							var changeError error
							mysqlChangeCheck, changeError = lib.CheckMysqlFileChanges(sqlConn.(lib.XODB), mysqlFile)
							if changeError != nil {
								tfmContext.Log("Error grabbing MysqlFiles", changeError)
								continue
							}
						}

						if mysqlChangeCheck > 0 {
							changedEntries = append(changedEntries, mysqlFile["MysqlFilePath"].(string))
							existingEntries = append(existingEntries, mysqlFile["MysqlFilePath"].(string))
						} else {
							existingEntries = append(existingEntries, mysqlFile["MysqlFilePath"].(string))
						}
					}
				}

				var mysqlFiles []*lib.MysqlFile
				var mysqlPullErr error
				if sqlConn != nil {
					mysqlFiles, mysqlPullErr = lib.GetMysqlFiles(sqlConn.(lib.XODB), changedEntries, existingEntries)
					if mysqlPullErr != nil {
						tfContext.RemoteDataSource["flowStateReceiver"].(chan flowcorehelper.FlowStateUpdate) <- flowcorehelper.FlowStateUpdate{FlowName: tfContext.Flow.TableName(), StateUpdate: "2", SyncFilter: tfContext.FlowState.SyncFilter, SyncMode: "pullerror"}
						tfmContext.Log("Error grabbing MysqlFiles", mysqlPullErr)
						continue
					}
					var filterMysqlFiles []*lib.MysqlFile
					tfContext.FlowLock.Lock()
					if tfContext.FlowState.SyncFilter != "" {
						syncFilter := strings.Split(strings.ReplaceAll(tfContext.FlowState.SyncFilter, " ", ""), ",")
						for _, filter := range syncFilter {
							for _, mysqlFile := range mysqlFiles {
								if strings.Contains(mysqlFile.MysqlFilePath, filter) {
									filterMysqlFiles = append(filterMysqlFiles, mysqlFile)
								}
							}
						}
						mysqlFiles = filterMysqlFiles
					}
					tfContext.FlowLock.Unlock()
				}

				for _, mysqlFile := range mysqlFiles {
					rows := tfmContext.CallDBQuery(tfContext, tfContext.GetSelectById(mysqlFile.MysqlFilePath), nil, false, "SELECT", nil, "")
					if len(rows) == 0 {
						tfmContext.Log("Inserting new mysqlfile", nil)

						insertQuery, bindings := tfContext.GetInsertQueryWithBindings(tfmContext, lib.GetMysqlFileMap(mysqlFile))
						tfmContext.CallDBQuery(tfContext, insertQuery, bindings, true, "INSERT", []FlowNameType{FlowNameType(tfContext.Flow.TableName())}, "") //Query to alert change channel
					} else {
						for _, value := range rows {
							if CompareLastModified(lib.GetMysqlFileMap(mysqlFile), lib.GetMysqlFileMapFromArray(value)) { //If equal-> do nothing
								continue
							} else { //If not equal -> update
								tfmContext.Log("Updating mysqlfile", nil)
								updateQuery, bindings := tfContext.GetUpdateQueryWithBindings(tfmContext, lib.GetMysqlFileMap(mysqlFile))
								tfmContext.CallDBQuery(tfContext, updateQuery, bindings, true, "UPDATE", []FlowNameType{FlowNameType(tfContext.Flow.TableName())}, "") //Query to alert change channel
							}
						}
					}
				}

				tfContext.FlowLock.Lock()
				if tfContext.FlowState.SyncMode == "pullonce" {
					tfContext.FlowState.SyncMode = "pullcomplete"
					tfmContext.Log(fmt.Sprintf("Pull complete: %s", tfContext.Flow), nil)
					go func(ftn string, sf string) {
						stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: ftn, StateUpdate: "2", SyncFilter: sf, SyncMode: "pullcomplete"}
					}(tfContext.Flow.TableName(), tfContext.FlowState.SyncFilter)
					// Now go to vault.
					// This is bad...
					// tfContext.Restart = true
					// tfContext.CancelContext() // Anti pattern...
				}
				tfContext.FlowLock.Unlock()

			}
		}
	}
	tfContext.Restart = false
	tfContext.CancelContext()
	return nil
}

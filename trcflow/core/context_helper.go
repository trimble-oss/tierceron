package core

import (
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"tierceron/buildopts/coreopts"
	trcvutils "tierceron/trcvault/util"
	"tierceron/trcx/extract"

	trcdb "tierceron/trcx/db"
	eUtils "tierceron/utils"
)

var changesLock sync.Mutex

func getChangeIdQuery(databaseName string, changeTable string) string {
	return fmt.Sprintf("SELECT id FROM %s.%s", databaseName, changeTable)
}

func getChangedByIdQuery(databaseName string, changeTable string, identityColumnName string, id interface{}) string {
	if _, iOk := id.(int64); iOk {
		return fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%d'", databaseName, changeTable, identityColumnName, id)
	} else {
		return fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%s'", databaseName, changeTable, identityColumnName, id)
	}
}

func getDeleteChangeQuery(databaseName string, changeTable string, id interface{}) string {
	if _, iOk := id.(int64); iOk {
		return fmt.Sprintf("DELETE FROM %s.%s WHERE id = '%d'", databaseName, changeTable, id)
	} else {
		return fmt.Sprintf("DELETE FROM %s.%s WHERE id = '%s'", databaseName, changeTable, id)
	}
}

func getInsertChangeQuery(databaseName string, changeTable string, id interface{}) string {
	if _, iOk := id.(int64); iOk {
		return fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES (%d, current_timestamp())", databaseName, changeTable, id)
	} else {
		return fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES (%s, current_timestamp())", databaseName, changeTable, id)
	}
}

// removeChangedTableEntries -- gets and removes any changed table entries.
func (tfmContext *TrcFlowMachineContext) removeChangedTableEntries(tfContext *TrcFlowContext) ([][]interface{}, error) {
	var changedEntriesQuery string

	changesLock.Lock()

	/*if isInit {
		changedEntriesQuery = `SELECT ` + identityColumnName + ` FROM ` + databaseName + `.` + tableName
	} else { */
	changedEntriesQuery = getChangeIdQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName)
	//}

	_, _, matrixChangedEntries, err := trcdb.Query(tfmContext.TierceronEngine, changedEntriesQuery)
	if err != nil {
		eUtils.LogErrorObject(tfmContext.Config, err, false)
		return nil, err
	}
	for _, changedEntry := range matrixChangedEntries {
		changedId := changedEntry[0]
		_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getDeleteChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, changedId))
		if err != nil {
			eUtils.LogErrorObject(tfmContext.Config, err, false)
			return nil, err
		}
	}
	changesLock.Unlock()
	return matrixChangedEntries, nil
}

func getStatisticChangeIdQuery(databaseName string, changeTable string, idCol string, indexCol string, secIndexCol string) string {
	return fmt.Sprintf("SELECT %s, %s, %s FROM %s.%s", idCol, indexCol, secIndexCol, databaseName, changeTable)
}

func getStatisticDeleteChangeQuery(databaseName string, changeTable string, idColVal interface{}, indexColVal interface{}, secIndexColVal interface{}, idCol string, indexCol string, secIndexCol string) string {
	if first, second, third := idColVal.(string), indexColVal.(string), secIndexColVal.(string); first != "" && second != "" && third != "" {
		return fmt.Sprintf("DELETE FROM %s.%s WHERE %s='%s' AND %s='%s' AND %s='%s'", databaseName, changeTable, idCol, idColVal, indexCol, indexColVal, secIndexCol, secIndexColVal)
	}
	return ""
}

func getStatisticChangedByIdQuery(databaseName string, changeTable string, idColVal interface{}, indexColVal interface{}, secIndexColVal interface{}, idCol string, indexCol string, secIndexCol string) string {
	if first, second, third := idColVal.(string), indexColVal.(string), secIndexColVal.(string); first != "" && second != "" && third != "" {
		return fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%s' AND %s='%s' AND %s='%s'", databaseName, changeTable, idCol, idColVal, indexCol, indexColVal, secIndexCol, secIndexColVal)
	}
	return ""
}

func getStatisticInsertChangeQuery(databaseName string, changeTable string, idColVal interface{}, indexColVal interface{}, secIndexColVal interface{}) string {
	if first, second, third := idColVal.(string), indexColVal.(string), secIndexColVal.(string); first != "" && second != "" && third != "" {
		return fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES ('%s', '%s', '%s', current_timestamp())", databaseName, changeTable, idColVal, indexColVal, secIndexColVal)
	}
	return ""
}

// removeChangedTableEntries -- gets and removes any changed table entries.
func (tfmContext *TrcFlowMachineContext) removeStatisticChangedTableEntries(tfContext *TrcFlowContext, idCol string, indexCol string, secIndexCol string) ([][]interface{}, error) {
	var changedEntriesQuery string

	changesLock.Lock()
	changedEntriesQuery = getStatisticChangeIdQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, idCol, indexCol, secIndexCol)

	_, _, matrixChangedEntries, err := trcdb.Query(tfmContext.TierceronEngine, changedEntriesQuery)
	if err != nil {
		eUtils.LogErrorObject(tfmContext.Config, err, false)
		return nil, err
	}
	for _, changedEntry := range matrixChangedEntries {
		idColVal := changedEntry[0]
		indexColVal := changedEntry[1]
		secIndexColVal := changedEntry[2]
		_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getStatisticDeleteChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, idColVal, indexColVal, secIndexColVal, idCol, indexCol, secIndexCol))
		if err != nil {
			eUtils.LogErrorObject(tfmContext.Config, err, false)
			return nil, err
		}
	}
	changesLock.Unlock()
	return matrixChangedEntries, nil
}

// vaultPersistPushRemoteChanges - Persists any local mysql changes to vault and pushed any changes to a remote data source.
func (tfmContext *TrcFlowMachineContext) vaultPersistPushRemoteChanges(
	tfContext *TrcFlowContext,
	identityColumnName string,
	vaultIndexColumnName string,
	vaultSecondIndexColumnName string,
	syncMysql bool,
	getIndexedPathExt func(engine interface{}, rowDataMap map[string]interface{}, vaultIndexColumnName string, databaseName string, tableName string, dbCallBack func(interface{}, string) (string, []string, [][]interface{}, error)) (string, error),
	flowPushRemote func(map[string]interface{}, map[string]interface{}) error) error {

	var matrixChangedEntries [][]interface{}
	var removeErr error
	if vaultSecondIndexColumnName != "" {
		matrixChangedEntries, removeErr = tfmContext.removeStatisticChangedTableEntries(tfContext, identityColumnName, vaultIndexColumnName, vaultSecondIndexColumnName)
		if removeErr != nil {
			tfmContext.Log("Failure to scrub table entries.", removeErr)
			return removeErr
		}
	} else {
		var removeErr error
		matrixChangedEntries, removeErr = tfmContext.removeChangedTableEntries(tfContext)
		if removeErr != nil {
			tfmContext.Log("Failure to scrub table entries.", removeErr)
			return removeErr
		}
	}

	for _, changedEntry := range matrixChangedEntries {
		var changedTableQuery string
		var changedId interface{}
		if len(changedEntry) == 3 {
			changedTableQuery = getStatisticChangedByIdQuery(tfContext.FlowSourceAlias, tfContext.Flow.TableName(), changedEntry[0], changedEntry[1], changedEntry[2], identityColumnName, vaultIndexColumnName, vaultSecondIndexColumnName)
		} else {
			changedId := changedEntry[0]
			changedTableQuery = getChangedByIdQuery(tfContext.FlowSourceAlias, tfContext.Flow.TableName(), identityColumnName, changedId)
		}
		_, changedTableColumns, changedTableRowData, err := trcdb.Query(tfmContext.TierceronEngine, changedTableQuery)
		if err != nil {
			eUtils.LogErrorObject(tfmContext.Config, err, false)
			continue
		}

		if len(changedTableRowData) == 0 && err == nil && len(changedEntry) != 3 { //This change was a delete
			for _, syncedTable := range coreopts.GetSyncedTables() {
				if tfContext.Flow.TableName() != syncedTable { //TODO: Add delete functionality for other tables? - logic is in SEC push remote
					continue
				}
			}
			//Check if it exists in trcdb
			//Writeback to mysql to delete that
			rowDataMap := map[string]interface{}{}
			rowDataMap["Deleted"] = "true"
			rowDataMap["changedId"] = changedId
			for _, column := range changedTableColumns {
				rowDataMap[column] = ""
			}
			pushError := flowPushRemote(tfContext.RemoteDataSource, rowDataMap)
			if pushError != nil {
				eUtils.LogErrorObject(tfmContext.Config, err, false)
			}
			continue
		}

		rowDataMap := map[string]interface{}{}
		for i, column := range changedTableColumns {
			rowDataMap[column] = changedTableRowData[0][i]
		}
		// Convert matrix/slice to table map
		// Columns are keys, values in tenantData

		//Use trigger to make another table
		indexPath, indexPathErr := getIndexedPathExt(tfmContext.TierceronEngine, rowDataMap, vaultIndexColumnName, tfContext.FlowSourceAlias, tfContext.Flow.TableName(), func(engine interface{}, query string) (string, []string, [][]interface{}, error) {
			return trcdb.Query(engine.(*trcdb.TierceronEngine), query)
		})
		if indexPathErr != nil { //TODO: Reinject unsuccessful changes for DataStatistics
			eUtils.LogErrorObject(tfmContext.Config, indexPathErr, false)
			// Re-inject into changes because it might not be here yet...
			if !strings.Contains(indexPath, "PublicIndex") {
				_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getInsertChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, changedId))
				if err != nil {
					eUtils.LogErrorObject(tfmContext.Config, err, false)
				}
			} else {
				if len(changedEntry) == 3 {
					_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getStatisticInsertChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, changedEntry[0], changedEntry[1], changedEntry[2]))
					if err != nil {
						eUtils.LogErrorObject(tfmContext.Config, err, false)
					}
				}
			}
			continue
		}

		if indexPath == "" && indexPathErr == nil {
			continue //This case is for when SEC row can't find a matching tenant
		}

		if identityColumnName == "MysqlFilePath" {
			if !strings.HasSuffix(rowDataMap["MysqlFileContent"].(string), "==") {
				rowDataMap["MysqlFileContent"] = base64.StdEncoding.EncodeToString([]byte(rowDataMap["MysqlFileContent"].(string)))
			}
		}

		seedError := trcvutils.SeedVaultById(tfmContext.Config, tfContext.GoMod, tfContext.Flow.ServiceName(), tfmContext.Config.VaultAddress, tfContext.Vault.GetToken(), tfContext.FlowData.(*extract.TemplateResultData), rowDataMap, indexPath, tfContext.FlowSource)
		if seedError != nil {
			eUtils.LogErrorObject(tfmContext.Config, seedError, false)
			// Re-inject into changes because it might not be here yet...
			_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getInsertChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, changedId.(string)))
			if err != nil {
				eUtils.LogErrorObject(tfmContext.Config, err, false)
			}
			continue
		}

		// Push this change to the flow for delivery to remote data source.
		if syncMysql {
			pushError := flowPushRemote(tfContext.RemoteDataSource, rowDataMap)
			if pushError != nil {
				eUtils.LogErrorObject(tfmContext.Config, err, false)
			}
		}

	}

	return nil
}

// seedTrcDbFromChanges - seeds Trc DB with changes from vault
func (tfmContext *TrcFlowMachineContext) seedTrcDbFromChanges(
	tfContext *TrcFlowContext,
	identityColumnName string,
	vaultIndexColumnName string,
	isInit bool,
	getIndexedPathExt func(engine interface{}, rowDataMap map[string]interface{}, vaultIndexColumnName string, databaseName string, tableName string, dbCallBack func(interface{}, string) (string, []string, [][]interface{}, error)) (string, error),
	flowPushRemote func(map[string]interface{}, map[string]interface{}) error) error {
	trcdb.TransformConfig(tfContext.GoMod,
		tfmContext.TierceronEngine,
		tfmContext.Env,
		"0",
		tfContext.FlowSource,
		tfContext.FlowSourceAlias,
		string(tfContext.Flow),
		tfmContext.Config)

	return nil
}

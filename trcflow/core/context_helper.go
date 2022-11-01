package core

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"tierceron/buildopts/coreopts"
	trcvutils "tierceron/trcvault/util"
	"tierceron/trcx/extract"

	trcdb "tierceron/trcx/db"
	trcengine "tierceron/trcx/engine"
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
		return fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES ('%s', current_timestamp())", databaseName, changeTable, id)
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

	_, _, matrixChangedEntries, err := trcdb.Query(tfmContext.TierceronEngine, changedEntriesQuery, tfContext.FlowLock)
	if err != nil {
		eUtils.LogErrorObject(tfmContext.Config, err, false)
		return nil, err
	}
	for _, changedEntry := range matrixChangedEntries {
		changedId := changedEntry[0]
		_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getDeleteChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, changedId), tfContext.FlowLock)
		if err != nil {
			eUtils.LogErrorObject(tfmContext.Config, err, false)
			return nil, err
		}
	}
	changesLock.Unlock()
	return matrixChangedEntries, nil
}

func getStatisticChangeIdQuery(databaseName string, changeTable string, indexColumnNames interface{}) string {
	return fmt.Sprintf("SELECT %s, %s, %s FROM %s.%s", indexColumnNames.([]string)[0], indexColumnNames.([]string)[1], indexColumnNames.([]string)[2], databaseName, changeTable)
}

func getStatisticDeleteChangeQuery(databaseName string, changeTable string, indexColumnNames interface{}, indexColumnValues interface{}) string {
	if first, second, third := indexColumnValues.([]string)[0], indexColumnValues.([]string)[1], indexColumnValues.([]string)[2]; first != "" && second != "" && third != "" {
		return fmt.Sprintf("DELETE FROM %s.%s WHERE %s='%s' AND %s='%s' AND %s='%s'", databaseName, changeTable, indexColumnNames.([]string)[0], indexColumnValues.([]string)[0], indexColumnNames.([]string)[1], indexColumnValues.([]string)[1], indexColumnNames.([]string)[2], indexColumnValues.([]string)[2])
	}
	return ""
}

func getStatisticChangedByIdQuery(databaseName string,
	changeTable string,
	indexColumnNames interface{},
	indexColumnValues interface{}) (string, error) {
	if indexColumnNamesSlice, iOk := indexColumnNames.([]string); iOk {
		if indexColumnValuesSlice, iOk := indexColumnValues.([]string); iOk {
			query := fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%s'", databaseName, changeTable, indexColumnNames.([]string)[0], indexColumnValuesSlice[0])

			if len(indexColumnValuesSlice) > 0 {
				for i := 0; i < len(indexColumnValuesSlice); i++ {
					query = fmt.Sprintf("%s AND %s='%s'", query, indexColumnNamesSlice[i], indexColumnValuesSlice[i])
				}
			}
			return query, nil
		} else {
			return "", errors.New("Invalid index value data for statistic data")
		}
	} else {
		return "", errors.New("Invalid index name data for statistic data")
	}
}

func getStatisticInsertChangeQuery(databaseName string, changeTable string, idColVal interface{}, indexColVal interface{}, secIndexColVal interface{}) string {
	if first, second, third := idColVal.(string), indexColVal.(string), secIndexColVal.(string); first != "" && second != "" && third != "" {
		return fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES ('%s', '%s', '%s', current_timestamp())", databaseName, changeTable, idColVal, indexColVal, secIndexColVal)
	}
	return ""
}

// removeChangedTableEntries -- gets and removes any changed table entries.
func (tfmContext *TrcFlowMachineContext) removeStatisticChangedTableEntries(tfContext *TrcFlowContext, indexColumnNames interface{}) ([][]interface{}, error) {
	var changedEntriesQuery string

	changesLock.Lock()
	changedEntriesQuery = getStatisticChangeIdQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, indexColumnNames)

	_, _, matrixChangedEntries, err := trcdb.Query(tfmContext.TierceronEngine, changedEntriesQuery, tfContext.FlowLock)
	if err != nil {
		eUtils.LogErrorObject(tfmContext.Config, err, false)
		return nil, err
	}
	for _, changedEntry := range matrixChangedEntries {
		indexColumnValues := []string{}
		indexColumnValues = append(indexColumnValues, changedEntry[0].(string))
		indexColumnValues = append(indexColumnValues, changedEntry[1].(string))
		indexColumnValues = append(indexColumnValues, changedEntry[2].(string))
		_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getStatisticDeleteChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, indexColumnNames, indexColumnValues), tfContext.FlowLock)
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
	indexColumnNames interface{},
	mysqlPushEnabled bool,
	getIndexedPathExt func(engine interface{}, rowDataMap map[string]interface{}, indexColumnNames interface{}, databaseName string, tableName string, dbCallBack func(interface{}, map[string]interface{}) (string, []string, [][]interface{}, error)) (string, error),
	flowPushRemote func(*TrcFlowContext, map[string]interface{}, map[string]interface{}) error) error {

	var matrixChangedEntries [][]interface{}
	var removeErr error
	if indexColumnNames != "" { // TODO: Coercion???
		matrixChangedEntries, removeErr = tfmContext.removeStatisticChangedTableEntries(tfContext, indexColumnNames)
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
		var changeTableError error
		changedTableQuery, changeTableError = getStatisticChangedByIdQuery(tfContext.FlowSourceAlias, tfContext.Flow.TableName(), indexColumnNames, changedEntry)
		if changeTableError != nil {
			eUtils.LogErrorObject(tfmContext.Config, changeTableError, false)
			continue
		}

		_, changedTableColumns, changedTableRowData, err := trcdb.Query(tfmContext.TierceronEngine, changedTableQuery, tfContext.FlowLock)
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
			pushError := flowPushRemote(tfContext, tfContext.RemoteDataSource, rowDataMap)
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
		indexPath, indexPathErr := getIndexedPathExt(tfmContext.TierceronEngine, rowDataMap, indexColumnNames, tfContext.FlowSourceAlias, tfContext.Flow.TableName(), func(engine interface{}, query map[string]interface{}) (string, []string, [][]interface{}, error) {
			return trcdb.Query(engine.(*trcengine.TierceronEngine), query["TrcQuery"].(string), tfContext.FlowLock)
		})
		if indexPathErr != nil {
			eUtils.LogErrorObject(tfmContext.Config, indexPathErr, false)
			// Re-inject into changes because it might not be here yet...
			if !strings.Contains(indexPath, "PublicIndex") {
				_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getInsertChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, changedId), tfContext.FlowLock)
				if err != nil {
					eUtils.LogErrorObject(tfmContext.Config, err, false)
				}
			} else {
				if len(changedEntry) == 3 { //Maybe there is a better way to do this, but this works for now.
					_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getStatisticInsertChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, changedEntry[0], changedEntry[1], changedEntry[2]), tfContext.FlowLock)
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

		if identityColumn, idOk := indexColumnNames.(string); idOk {
			if identityColumn == "flowName" {
				if alert, ok := rowDataMap[indexColumnNames.([]string)[0]].(string); ok {
					if tfmContext.FlowControllerUpdateAlert != nil {
						tfmContext.FlowControllerUpdateAlert <- alert
					}
				}
			}
		}

		if !tfContext.ReadOnly {
			seedError := trcvutils.SeedVaultById(tfmContext.Config, tfContext.GoMod, tfContext.Flow.ServiceName(), tfmContext.Config.VaultAddress, tfContext.Vault.GetToken(), tfContext.FlowData.(*extract.TemplateResultData), rowDataMap, indexPath, tfContext.FlowSource)
			if seedError != nil {
				eUtils.LogErrorObject(tfmContext.Config, seedError, false)
				// Re-inject into changes because it might not be here yet...
				_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getInsertChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, changedId.(string)), tfContext.FlowLock)
				if err != nil {
					eUtils.LogErrorObject(tfmContext.Config, err, false)
				}
				continue
			}
		}

		// Push this change to the flow for delivery to remote data source.
		if mysqlPushEnabled && flowPushRemote != nil {
			pushError := flowPushRemote(tfContext, tfContext.RemoteDataSource, rowDataMap)
			if pushError != nil {
				eUtils.LogErrorObject(tfmContext.Config, err, false)
			}
		}

	}

	return nil
}

/*
// seedTrcDbFromChanges - seeds Trc DB with changes from vault
func (tfmContext *TrcFlowMachineContext) seedTrcDbFromChanges(
	tfContext *TrcFlowContext,
	isInit bool,
	getIndexedPathExt func(engine interface{}, rowDataMap map[string]interface{}, vaultIndexColumnName string, databaseName string, tableName string, dbCallBack func(interface{}, map[string]string) (string, []string, [][]interface{}, error)) (string, error),
	flowPushRemote func(*TrcFlowContext, map[string]interface{}, map[string]interface{}) error,
	tableLock *sync.Mutex) error {
	trcseed.TransformConfig(tfContext.GoMod,
		tfmContext.TierceronEngine,
		tfmContext.Env,
		"0",
		tfContext.FlowSource,
		tfContext.FlowSourceAlias,
		string(tfContext.Flow),
		tfmContext.Config,
		tableLock)

	return nil
}
*/
// seedTrcDbFromVault - optimized implementation of seedTrcDbFromChanges
func (tfmContext *TrcFlowMachineContext) seedTrcDbFromVault(
	tfContext *TrcFlowContext) error {
	var indexValues []string = []string{}
	var secondaryIndexes []string
	var err error

	if tfContext.CustomSeedTrcDb != nil {
		customSeedErr := tfContext.CustomSeedTrcDb(tfmContext, tfContext)
		return customSeedErr
	}
	if tfContext.GoMod != nil {
		tfContext.GoMod.Env = tfmContext.Env
		tfContext.GoMod.Version = "0"

		index, secondaryI, indexErr := coreopts.FindIndexForService(tfContext.FlowSource, tfContext.Flow.ServiceName())
		if indexErr == nil && index != "" {
			tfContext.GoMod.SectionName = index
			secondaryIndexes = secondaryI
		}
		if tfContext.GoMod.SectionName != "" {
			indexValues, err = tfContext.GoMod.ListSubsection("/Index/", tfContext.FlowSource, tfContext.GoMod.SectionName, tfmContext.Config.Log)
			if err != nil {
				eUtils.LogErrorObject(tfmContext.Config, err, false)
				return err
			}
		}
	}

	for _, indexValue := range indexValues {
		if indexValue != "" {
			tfContext.GoMod.SectionKey = "/Index/"
			tfContext.GoMod.SectionPath = "super-secrets/Index/" + tfContext.FlowSource + "/" + tfContext.GoMod.SectionName + "/" + indexValue + "/" + tfContext.Flow.ServiceName()
			if len(secondaryIndexes) > 0 {
				for _, secondaryIndex := range secondaryIndexes {
					tfContext.GoMod.SectionPath = "super-secrets/Index/" + tfContext.FlowSource + "/" + tfContext.GoMod.SectionName + "/" + indexValue + "/" + tfContext.Flow.ServiceName() + "/" + secondaryIndex
					rowErr := tfmContext.PathToTableRowHelper(tfContext)
					if rowErr != nil {
						return rowErr
					}
				}
			} else {
				rowErr := tfmContext.PathToTableRowHelper(tfContext)
				if rowErr != nil {
					return rowErr
				}
			}
		} else {
			rowErr := tfmContext.PathToTableRowHelper(tfContext)
			if rowErr != nil {
				return rowErr
			}
		}
	}

	return nil
}

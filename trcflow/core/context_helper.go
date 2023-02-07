package core

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	trcvutils "github.com/trimble-oss/tierceron/trcvault/util"
	"github.com/trimble-oss/tierceron/trcx/extract"

	trcdb "github.com/trimble-oss/tierceron/trcx/db"
	trcengine "github.com/trimble-oss/tierceron/trcx/engine"
	eUtils "github.com/trimble-oss/tierceron/utils"

	sqlememory "github.com/dolthub/go-mysql-server/memory"
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

func getCompositeChangeIdQuery(databaseName string, changeTable string, indexColumnNames interface{}) string {
	return fmt.Sprintf("SELECT %s, %s FROM %s.%s", indexColumnNames.([]string)[0], indexColumnNames.([]string)[1], databaseName, changeTable)
}

func getCompositeDeleteChangeQuery(databaseName string, changeTable string, indexColumnNames interface{}, indexColumnValues interface{}) string {
	if first, second, third, fourth := indexColumnNames.([]string)[0], indexColumnValues.([]string)[0], indexColumnNames.([]string)[1], indexColumnValues.([]string)[1]; first != "" && second != "" && third != "" && fourth != "" {
		return fmt.Sprintf("DELETE FROM %s.%s WHERE %s='%s' AND %s='%s'", databaseName, changeTable, indexColumnNames.([]string)[0], indexColumnValues.([]string)[0], indexColumnNames.([]string)[1], indexColumnValues.([]string)[1])
	}
	return ""
}

// removeChangedTableEntries -- gets and removes any changed table entries.
func (tfmContext *TrcFlowMachineContext) removeCompositeKeyChangedTableEntries(tfContext *TrcFlowContext, idCol string, indexColumnNames interface{}) ([][]interface{}, error) {
	var changedEntriesQuery string

	changesLock.Lock()
	changedEntriesQuery = getCompositeChangeIdQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, indexColumnNames)

	_, _, matrixChangedEntries, err := trcdb.Query(tfmContext.TierceronEngine, changedEntriesQuery, tfContext.FlowLock)
	if err != nil {
		eUtils.LogErrorObject(tfmContext.Config, err, false)
		return nil, err
	}
	for _, changedEntry := range matrixChangedEntries {
		indexColumnValues := []string{}
		indexColumnValues = append(indexColumnValues, changedEntry[0].(string))
		indexColumnValues = append(indexColumnValues, changedEntry[1].(string))
		_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getCompositeDeleteChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, indexColumnNames, indexColumnValues), tfContext.FlowLock)
		if err != nil {
			eUtils.LogErrorObject(tfmContext.Config, err, false)
			return nil, err
		}
	}
	changesLock.Unlock()
	return matrixChangedEntries, nil
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

func getStatisticChangeIdQuery(databaseName string, changeTable string, idCol string, indexColumnNames interface{}) string {
	return fmt.Sprintf("SELECT %s, %s, %s FROM %s.%s", idCol, indexColumnNames.([]string)[0], indexColumnNames.([]string)[1], databaseName, changeTable)
}

func getStatisticDeleteChangeQuery(databaseName string, changeTable string, idCol string, idColVal interface{}, indexColumnNames interface{}, indexColumnValues interface{}) string {
	if first, second, third := idColVal.(string), indexColumnValues.([]string)[0], indexColumnValues.([]string)[1]; first != "" && second != "" && third != "" {
		return fmt.Sprintf("DELETE FROM %s.%s WHERE %s='%s' AND %s='%s' AND %s='%s'", databaseName, changeTable, idCol, idColVal, indexColumnNames.([]string)[0], indexColumnValues.([]string)[0], indexColumnNames.([]string)[1], indexColumnValues.([]string)[1])
	}
	return ""
}

func getStatisticChangedByIdQuery(databaseName string, changeTable string, idColumn string, indexColumnNames interface{}, indexColumnValues interface{}) (string, error) {
	if indexColumnNamesSlice, iOk := indexColumnNames.([]string); iOk {
		if indexColumnValuesSlice, iOk := indexColumnValues.([]interface{}); iOk {
			var query string
			if valueStr, sOk := indexColumnValuesSlice[0].(string); sOk {
				query = fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%s'", databaseName, changeTable, idColumn, valueStr)
			} else if valueInt, viOK := indexColumnValuesSlice[0].(int64); viOK {
				query = fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%d'", databaseName, changeTable, idColumn, valueInt)
			} else if valueInt, vIntOK := indexColumnValuesSlice[0].(int); vIntOK {
				query = fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%d'", databaseName, changeTable, idColumn, valueInt)
			} else {
				panic("Error - Unsupported type for index column - add support for new type.")
			}

			if len(indexColumnValuesSlice) > 1 {
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
func (tfmContext *TrcFlowMachineContext) removeStatisticChangedTableEntries(tfContext *TrcFlowContext, idCol string, indexColumnNames interface{}) ([][]interface{}, error) {
	var changedEntriesQuery string

	changesLock.Lock()
	changedEntriesQuery = getStatisticChangeIdQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, idCol, indexColumnNames)

	_, _, matrixChangedEntries, err := trcdb.Query(tfmContext.TierceronEngine, changedEntriesQuery, tfContext.FlowLock)
	if err != nil {
		eUtils.LogErrorObject(tfmContext.Config, err, false)
		return nil, err
	}
	for _, changedEntry := range matrixChangedEntries {
		idColVal := changedEntry[0]
		indexColumnValues := []string{}
		indexColumnValues = append(indexColumnValues, changedEntry[1].(string))
		indexColumnValues = append(indexColumnValues, changedEntry[2].(string))
		_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getStatisticDeleteChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, idCol, idColVal, indexColumnNames, indexColumnValues), tfContext.FlowLock)
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
	indexColumnNames interface{},
	mysqlPushEnabled bool,
	getIndexedPathExt func(engine interface{}, rowDataMap map[string]interface{}, indexColumnNames interface{}, databaseName string, tableName string, dbCallBack func(interface{}, map[string]interface{}) (string, []string, [][]interface{}, error)) (string, error),
	flowPushRemote func(*TrcFlowContext, map[string]interface{}, map[string]interface{}) error) error {

	var matrixChangedEntries [][]interface{}
	var removeErr error

	if indexColumnNamesSlice, colOK := indexColumnNames.([]string); colOK {
		if len(indexColumnNamesSlice) == 3 { // TODO: Coercion???
			matrixChangedEntries, removeErr = tfmContext.removeStatisticChangedTableEntries(tfContext, identityColumnName, indexColumnNames)
			if removeErr != nil {
				tfmContext.Log("Failure to scrub table entries.", removeErr)
				return removeErr
			}
		} else if len(indexColumnNamesSlice) == 2 { // TODO: Coercion???
			matrixChangedEntries, removeErr = tfmContext.removeCompositeKeyChangedTableEntries(tfContext, identityColumnName, indexColumnNames)
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
	}

	for _, changedEntry := range matrixChangedEntries {
		var changedTableQuery string
		var changedId interface{}
		var changeTableError error
		changedTableQuery, changeTableError = getStatisticChangedByIdQuery(tfContext.FlowSourceAlias, tfContext.Flow.TableName(), identityColumnName, indexColumnNames, changedEntry)
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
			if flowPushRemote != nil {
				pushError := flowPushRemote(tfContext, tfContext.RemoteDataSource, rowDataMap)
				if pushError != nil {
					eUtils.LogErrorObject(tfmContext.Config, err, false)
				}
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

		if identityColumnName == "flowName" {
			if alert, ok := rowDataMap[identityColumnName].(string); ok {
				if tfmContext.FlowControllerUpdateAlert != nil {
					tfmContext.FlowControllerUpdateAlert <- alert
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
	identityColumnName string,
	vaultIndexColumnName string,
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

		index, secondaryI, indexExt, indexErr := coreopts.FindIndexForService(tfContext.FlowSource, tfContext.Flow.ServiceName())
		if indexErr == nil && index != "" {
			tfContext.GoMod.SectionName = index
			secondaryIndexes = secondaryI
			tfContext.GoMod.SubSectionName = indexExt
		}
		if tfContext.GoMod.SectionName != "" {
			indexValues, err = tfContext.GoMod.ListSubsection("/Index/", tfContext.FlowSource, tfContext.GoMod.SectionName, tfmContext.Config.Log)
			if err != nil {
				eUtils.LogErrorObject(tfmContext.Config, err, false)
				return err
			}
		}
	}

	rows := make([][]interface{}, 0)
	for _, indexValue := range indexValues {
		if indexValue != "" {
			tfContext.GoMod.SectionKey = "/Index/"
			var subSection string
			if tfContext.GoMod.SubSectionName != "" {
				subSection = tfContext.GoMod.SubSectionName
			} else {
				subSection = tfContext.Flow.ServiceName()
			}
			tfContext.GoMod.SectionPath = "super-secrets/Index/" + tfContext.FlowSource + "/" + tfContext.GoMod.SectionName + "/" + indexValue + "/" + subSection
			if len(secondaryIndexes) > 0 {
				for _, secondaryIndex := range secondaryIndexes {
					tfContext.GoMod.SectionPath = "super-secrets/Index/" + tfContext.FlowSource + "/" + tfContext.GoMod.SectionName + "/" + indexValue + "/" + subSection + "/" + secondaryIndex
					if subSection != "" {
						subIndexValues, _ := tfContext.GoMod.ListSubsection("/Index/"+tfContext.FlowSource+"/"+tfContext.GoMod.SectionName+"/"+indexValue+"/", subSection, secondaryIndex, tfmContext.Config.Log)
						if subIndexValues != nil {
							for _, subIndexValue := range subIndexValues {
								tfContext.GoMod.SectionPath = "super-secrets/Index/" + tfContext.FlowSource + "/" + tfContext.GoMod.SectionName + "/" + indexValue + "/" + subSection + "/" + secondaryIndex + "/" + subIndexValue + "/" + tfContext.Flow.ServiceName()
								row, rowErr := tfmContext.PathToTableRowHelper(tfContext)
								if rowErr != nil {
									eUtils.LogErrorObject(tfmContext.Config, rowErr, false)
									continue
								}
								rows = append(rows, row)
							}
						} else {
							row, rowErr := tfmContext.PathToTableRowHelper(tfContext)
							if rowErr != nil {
								eUtils.LogErrorObject(tfmContext.Config, rowErr, false)
								continue
							}
							rows = append(rows, row)
						}
					} else {
						row, rowErr := tfmContext.PathToTableRowHelper(tfContext)
						if rowErr != nil {
							eUtils.LogErrorObject(tfmContext.Config, rowErr, false)
							continue
						}
						rows = append(rows, row)
					}
				}
			} else {
				row, rowErr := tfmContext.PathToTableRowHelper(tfContext)
				if rowErr != nil {
					eUtils.LogErrorObject(tfmContext.Config, rowErr, false)
					continue
				}
				rows = append(rows, row)
			}
		} else {
			row, rowErr := tfmContext.PathToTableRowHelper(tfContext)
			if rowErr != nil {
				eUtils.LogErrorObject(tfmContext.Config, rowErr, false)
				continue
			}
			rows = append(rows, row)
		}
	}

	//Writes accumlated rows to the table.
	if tfContext.Inserter == nil {
		tableSql, tableOk, _ := tfmContext.TierceronEngine.Database.GetTableInsensitive(nil, tfContext.Flow.TableName())
		if tableOk {
			tfContext.Inserter = tableSql.(*sqlememory.Table).Inserter(tfmContext.TierceronEngine.Context)
		} else {
			insertErr := errors.New("Unable to insert rows into:" + tfContext.Flow.TableName())
			eUtils.LogErrorObject(tfmContext.Config, insertErr, false)
			return insertErr
		}
	}
	for _, row := range rows {
		if row == nil {
			continue
		}
		if err := tfContext.Inserter.Insert(tfmContext.TierceronEngine.Context, row); err != nil {
			eUtils.LogErrorObject(tfmContext.Config, err, false)
			continue
		}
	}
	tfContext.Inserter.Close(tfmContext.TierceronEngine.Context)
	tfContext.Inserter = nil

	return nil
}

package core

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	tcflow "github.com/trimble-oss/tierceron-core/v2/flow"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"
	"github.com/trimble-oss/tierceron/pkg/trcx/extract"

	trcdb "github.com/trimble-oss/tierceron/atrium/trcdb"
	trcengine "github.com/trimble-oss/tierceron/atrium/trcdb/engine"
	"github.com/trimble-oss/tierceron/atrium/trcflow/core/flowcorehelper"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"

	sqlememory "github.com/dolthub/go-mysql-server/memory"
	"github.com/dolthub/go-mysql-server/sql"
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
func (tfmContext *TrcFlowMachineContext) removeCompositeKeyChangedTableEntries(tfContext *TrcFlowContext, idCols []string, indexColumnNames interface{}) ([][]interface{}, error) {
	var changedEntriesQuery string

	changesLock.Lock()
	changedEntriesQuery = getCompositeChangeIdQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, indexColumnNames)

	_, _, matrixChangedEntries, err := trcdb.Query(tfmContext.TierceronEngine, changedEntriesQuery, tfContext.QueryLock)
	if err != nil {
		eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
		return nil, err
	}
	for _, changedEntry := range matrixChangedEntries {
		indexColumnValues := []string{}
		indexColumnValues = append(indexColumnValues, changedEntry[0].(string))
		indexColumnValues = append(indexColumnValues, changedEntry[1].(string))
		_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getCompositeDeleteChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, indexColumnNames, indexColumnValues), tfContext.QueryLock)
		if err != nil {
			eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
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

	_, _, matrixChangedEntries, err := trcdb.Query(tfmContext.TierceronEngine, changedEntriesQuery, tfContext.QueryLock)
	if err != nil {
		eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
		return nil, err
	}
	for _, changedEntry := range matrixChangedEntries {
		changedId := changedEntry[0]
		_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getDeleteChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, changedId), tfContext.QueryLock)
		if err != nil {
			eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
			return nil, err
		}
	}
	changesLock.Unlock()
	return matrixChangedEntries, nil
}

func getStatisticChangeIdQuery(databaseName string, changeTable string, idCols []string, indexColumnNames interface{}) string {
	if len(idCols) == 1 {
		return fmt.Sprintf("SELECT %s, %s, %s FROM %s.%s", idCols[0], indexColumnNames.([]string)[0], indexColumnNames.([]string)[1], databaseName, changeTable)
	} else if len(idCols) == 2 {
		return fmt.Sprintf("SELECT %s, %s FROM %s.%s", idCols[0], idCols[1], databaseName, changeTable)
	} else if len(idCols) == 3 {
		return fmt.Sprintf("SELECT %s, %s, %s FROM %s.%s", idCols[0], idCols[1], idCols[2], databaseName, changeTable)
	} else {
		return fmt.Sprintf("SELECT %s FROM %s.%s", idCols[0], databaseName, changeTable)
	}
}

func getStatisticDeleteChangeQuery(databaseName string, changeTable string, idCols []string, idColVal interface{}, indexColumnNames interface{}, indexColumnValues interface{}) string {
	if first, second, third := idColVal.(string), indexColumnValues.([]string)[0], indexColumnValues.([]string)[1]; first != "" && second != "" && third != "" {
		return fmt.Sprintf("DELETE FROM %s.%s WHERE %s='%s' AND %s='%s' AND %s='%s'", databaseName, changeTable, idCols[0], idColVal, indexColumnNames.([]string)[0], indexColumnValues.([]string)[0], indexColumnNames.([]string)[1], indexColumnValues.([]string)[1])
	}
	return ""
}

func removeElementFromSlice(slice []string, ss []string) ([]string, string) {
	for k, v := range slice {
		for _, s := range ss {
			if slice[k] == s {
				removedVal := slice[k]
				return append(slice[:k], slice[k+1:]...), removedVal
			}
			slice[k] = v
		}
	}
	return slice, ""
}

func removeElementFromSliceInterface(slice []interface{}, ss []string) ([]interface{}, interface{}) {
	indexFound := -1
	for _, v := range slice {
		indexFound = 0
		for _, s := range ss {
			if valueStr, sOk := v.(string); !sOk || valueStr != s {
				indexFound = -1
				break
			}
		}
		if indexFound != -1 {
			break
		}
	}

	if indexFound == -1 {
		return slice, nil
	}

	var value interface{} = &slice

	// Now do the removal:
	sp := value.(*[]interface{})
	removedVal := (*sp)[indexFound]
	*sp = append((*sp)[:indexFound], (*sp)[indexFound+1:]...)
	return *sp, removedVal
}

func getStatisticChangedByIdQuery(databaseName string, changeTable string, idColumns []string, indexColumnNames interface{}, indexColumnValues interface{}) (string, error) {
	if indexColumnNamesSlice, iOk := indexColumnNames.([]string); iOk {
		if indexColumnValuesSlice, iOk := indexColumnValues.([]interface{}); iOk {
			var query string
			var removedVal interface{}
			var removedValName string
			if valueSliceStr, sOk := indexColumnValuesSlice[0].([]string); sOk {
				if len(idColumns) == 1 {
					query = fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%s'", databaseName, changeTable, idColumns[0], valueSliceStr[0])
				} else if len(idColumns) == 2 {
					query = fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%s' AND %s='%s'", databaseName, changeTable, idColumns[0], valueSliceStr[0], idColumns[1], valueSliceStr[1])
				} else if len(idColumns) == 3 {
					// TODO: test...
					query = fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%s' AND %s='%s' AND %s='%s'", databaseName, changeTable, idColumns[0], valueSliceStr[0], idColumns[1], valueSliceStr[1], idColumns[2], valueSliceStr[2])
				}
				if indexColumnValuesSlice, removedVal = removeElementFromSliceInterface(indexColumnValuesSlice, valueSliceStr); removedVal != nil { //this logic is for dfs...names & values appear out of order in slices at this point but is needed for previous step.

					indexColumnNamesSlice, removedValName = removeElementFromSlice(indexColumnNamesSlice, idColumns) //							 may need to revist if a table has 3 identifiying column names (none currently).
				}
			} else if valueInt, viOK := indexColumnValuesSlice[0].(int64); viOK {
				if len(idColumns) == 1 {
					query = fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%d'", databaseName, changeTable, idColumns[0], valueInt)
				} else if len(idColumns) == 2 {
					query = fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%d' AND %s='%d'", databaseName, changeTable, idColumns[0], valueInt, idColumns[1], indexColumnValuesSlice[1])
				} else if len(idColumns) == 3 {
					query = fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%d' AND %s='%d' AND %s='%d'", databaseName, changeTable, idColumns[0], valueInt, idColumns[1], indexColumnValuesSlice[1], idColumns[2], indexColumnValuesSlice[2])
				}
			} else if valueInt, vIntOK := indexColumnValuesSlice[0].(int); vIntOK {
				if len(idColumns) == 1 {
					query = fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%d'", databaseName, changeTable, idColumns[0], valueInt)
				} else if len(idColumns) == 2 {
					query = fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%d' AND %s='%d'", databaseName, changeTable, idColumns[0], valueInt, idColumns[1], indexColumnValuesSlice[1])
				} else if len(idColumns) == 3 {
					query = fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%d' AND %s='%d' AND %s='%d'", databaseName, changeTable, idColumns[0], valueInt, idColumns[1], indexColumnValuesSlice[1], idColumns[2], indexColumnValuesSlice[2])
				}
			} else if valueStr, vStrOK := indexColumnValuesSlice[0].(string); vStrOK {
				if len(idColumns) == 1 {
					query = fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%s'", databaseName, changeTable, idColumns[0], valueStr)
				} else if len(idColumns) == 2 {
					query = fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%s' AND %s='%s'", databaseName, changeTable, idColumns[0], valueStr, idColumns[1], indexColumnValuesSlice[1])
				} else if len(idColumns) == 3 {
					query = fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%s' AND %s='%s' AND %s='%s'", databaseName, changeTable, idColumns[0], valueStr, idColumns[1], indexColumnValuesSlice[1], idColumns[2], indexColumnValuesSlice[2])
				}
			} else {
				return "", errors.New("Error - Unsupported type for index column - add support for new type.")
			}

			if len(indexColumnNamesSlice) > 1 {
				for i := 0; i < len(indexColumnNamesSlice); i++ {
					query = fmt.Sprintf("%s AND %s='%s'", query, indexColumnNamesSlice[i], indexColumnValuesSlice[i])
				}
			}

			if removedValName != "" { //Adding back in ordered name & val for dfs for next steps...
				indexColumnValuesSlice = append(indexColumnValuesSlice, removedVal)
				indexColumnNamesSlice = append(indexColumnNamesSlice, removedValName)
			}

			return query, nil
		} else {
			return "", errors.New("invalid index value data for statistic data")
		}
	} else {
		return "", errors.New("invalid index name data for statistic data")
	}
}

func getStatisticInsertChangeQuery(databaseName string, changeTable string, idColVal interface{}, indexColVal interface{}, secIndexColVal interface{}) string {
	if first, second, third := idColVal.(string), indexColVal.(string), secIndexColVal.(string); first != "" && second != "" && third != "" {
		return fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES ('%s', '%s', '%s', current_timestamp())", databaseName, changeTable, idColVal, indexColVal, secIndexColVal)
	}
	return ""
}

// removeChangedTableEntries -- gets and removes any changed table entries.
func (tfmContext *TrcFlowMachineContext) removeStatisticChangedTableEntries(tcflowContext tcflow.FlowContext, idCols []string, indexColumnNames interface{}) ([][]interface{}, error) {
	var changedEntriesQuery string
	tfContext := tcflowContext.(*TrcFlowContext)

	changesLock.Lock()
	changedEntriesQuery = getStatisticChangeIdQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, idCols, indexColumnNames)

	_, _, matrixChangedEntries, err := trcdb.Query(tfmContext.TierceronEngine, changedEntriesQuery, tfContext.QueryLock)
	if err != nil {
		eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
		return nil, err
	}
	for _, changedEntry := range matrixChangedEntries {
		idColVal := changedEntry[0]
		indexColumnValues := []string{}
		indexColumnValues = append(indexColumnValues, changedEntry[1].(string))
		indexColumnValues = append(indexColumnValues, changedEntry[2].(string))
		_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getStatisticDeleteChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, idCols, idColVal, indexColumnNames, indexColumnValues), tfContext.QueryLock)
		if err != nil {
			eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
			return nil, err
		}
	}
	changesLock.Unlock()
	return matrixChangedEntries, nil
}

// vaultPersistPushRemoteChanges - Persists any local mysql changes to vault and pushed any changes to a remote data source.
func (tfmContext *TrcFlowMachineContext) vaultPersistPushRemoteChanges(
	tcflowContext tcflow.FlowContext,
	identityColumnNames []string,
	indexColumnNames interface{},
	mysqlPushEnabled bool,
	getIndexedPathExt func(engine interface{}, rowDataMap map[string]interface{}, indexColumnNames interface{}, databaseName string, tableName string, dbCallBack func(interface{}, map[string]interface{}) (string, []string, [][]interface{}, error)) (string, error),
	flowPushRemote func(tcflow.FlowContext, map[string]interface{}) error) error {
	tfContext := tcflowContext.(*TrcFlowContext)

	var matrixChangedEntries [][]interface{}
	var removeErr error

	if indexColumnNamesSlice, colOK := indexColumnNames.([]string); colOK {
		if len(indexColumnNamesSlice) == 3 { // TODO: Coercion???
			matrixChangedEntries, removeErr = tfmContext.removeStatisticChangedTableEntries(tfContext, identityColumnNames, indexColumnNames)
			if removeErr != nil {
				tfmContext.Log("Failure to scrub table entries.", removeErr)
				return removeErr
			}
		} else if len(indexColumnNamesSlice) == 2 { // TODO: Coercion???
			matrixChangedEntries, removeErr = tfmContext.removeCompositeKeyChangedTableEntries(tfContext, identityColumnNames, indexColumnNames)
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
		changedTableQuery, changeTableError = getStatisticChangedByIdQuery(tfContext.FlowSourceAlias, tfContext.Flow.TableName(), identityColumnNames, indexColumnNames, changedEntry)
		if changeTableError != nil {
			eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, changeTableError, false)
			continue
		}

		_, changedTableColumns, changedTableRowData, err := trcdb.Query(tfmContext.TierceronEngine, changedTableQuery, tfContext.QueryLock)
		if err != nil {
			eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
			continue
		}

		if len(changedTableRowData) == 0 && err == nil && len(changedEntry) != 3 { //This change was a delete
			syncDelete := false
			for _, syncedTable := range coreopts.BuildOptions.GetSyncedTables() {
				if tfContext.Flow.TableName() == syncedTable {
					syncDelete = true
				}
			}

			if !syncDelete {
				continue
			}

			if tfContext.FlowState.State != 0 && (tfContext.FlowState.SyncMode == "push" || tfContext.FlowState.SyncMode == "pushonce") && flowPushRemote != nil {
				//Check if it exists in trcdb
				//Writeback to mysql to delete that
				rowDataMap := map[string]interface{}{}
				rowDataMap["Deleted"] = "true"
				rowDataMap["changedId"] = changedId
				for _, column := range changedTableColumns {
					rowDataMap[column] = ""
				}

				pushError := flowPushRemote(tfContext, rowDataMap)
				if pushError != nil {
					eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, changeTableError, false)
				}
			}

			rowDataMap := map[string]interface{}{}
			for index, column := range indexColumnNames.([]string) {
				if _, strOk := changedEntry[index].(string); strOk && len(changedEntry[index].(string)) == 0 {
					// Invalid string index...  Skip these.
					continue
				}
				rowDataMap[column] = changedEntry[index]
			}

			indexPath, indexPathErr := getIndexedPathExt(tfmContext.TierceronEngine, rowDataMap, indexColumnNames, tfContext.FlowSourceAlias, tfContext.Flow.TableName(), func(engine interface{}, query map[string]interface{}) (string, []string, [][]interface{}, error) {
				return trcdb.Query(engine.(*trcengine.TierceronEngine), query["TrcQuery"].(string), tfContext.QueryLock)
			})
			if indexPathErr != nil {
				eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, indexPathErr, false)
				continue
			}

			if !tfContext.ReadOnly {
				if !strings.Contains(indexPath, "/PublicIndex/") {
					indexPath = "Index/" + tfContext.FlowSource + indexPath
					if !strings.HasSuffix(indexPath, tfContext.Flow.TableName()) {
						indexPath = indexPath + "/" + tfContext.Flow.TableName()
					}
				}

				deleteMap, deleteErr := tfContext.GoMod.SoftDelete(indexPath, tfContext.Logger)
				if deleteErr != nil || deleteMap != nil {
					eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, errors.New("Unable to process a delete query for "+tfContext.Flow.TableName()), false)
				}
			}
			continue
		}

		rowDataMap := map[string]interface{}{}
		if len(changedTableRowData) == 0 {
			continue
		}
		for i, column := range changedTableColumns {
			rowDataMap[column] = changedTableRowData[0][i]
		}
		// Convert matrix/slice to table map
		// Columns are keys, values in tenantData

		//Use trigger to make another table
		indexPath, indexPathErr := getIndexedPathExt(tfmContext.TierceronEngine, rowDataMap, indexColumnNames, tfContext.FlowSourceAlias, tfContext.Flow.TableName(), func(engine interface{}, query map[string]interface{}) (string, []string, [][]interface{}, error) {
			return trcdb.Query(engine.(*trcengine.TierceronEngine), query["TrcQuery"].(string), tfContext.QueryLock)
		})
		if indexPathErr != nil {
			eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
			// Re-inject into changes because it might not be here yet...
			if !strings.Contains(indexPath, "PublicIndex") {
				_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getInsertChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, changedId), tfContext.QueryLock)
				if err != nil {
					eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
				}
			} else {
				if len(changedEntry) == 3 { //Maybe there is a better way to do this, but this works for now.
					_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getStatisticInsertChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, changedEntry[0], changedEntry[1], changedEntry[2]), tfContext.QueryLock)
					if err != nil {
						eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
					}
				}
			}
			continue
		}

		if indexPath == "" && indexPathErr == nil {
			continue //This case is for when SEC row can't find a matching tenant
		}

		if len(identityColumnNames) > 0 && identityColumnNames[0] == "flowName" {
			if alert, ok := rowDataMap[identityColumnNames[0]].(string); ok {
				if tfmContext.FlowControllerUpdateAlert != nil {
					tfmContext.FlowControllerUpdateAlert <- alert
				}
			}
		}

		if !tfContext.ReadOnly {
			seedError := trcvutils.SeedVaultById(tfmContext.DriverConfig, tfContext.GoMod, tfContext.Flow.ServiceName(), tfmContext.DriverConfig.CoreConfig.TokenCache.VaultAddressPtr, tfContext.Vault.GetToken(), tfContext.FlowData.(*extract.TemplateResultData), rowDataMap, indexPath, tfContext.FlowSource)
			if seedError != nil {
				eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, seedError, false)
				// Re-inject into changes because it might not be here yet...
				_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getInsertChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, changedId.(string)), tfContext.QueryLock)
				if err != nil {
					eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
				}
				continue
			}
		}

		// Push this change to the flow for delivery to remote data source.
		if mysqlPushEnabled && flowPushRemote != nil {
			pushError := flowPushRemote(tfContext, rowDataMap)
			if pushError != nil {
				eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, pushError, false)
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
		tfmContext.DriverConfig.CoreConfig,
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

		index, secondaryI, indexExt, indexErr := func(tfCtx *TrcFlowContext) (string, []string, string, error) {
			if tfCtx.FlowSource == flowcorehelper.TierceronFlowDB {
				if tfCtx.Flow.ServiceName() == flowcorehelper.TierceronFlowConfigurationTableName {
					return "flowName", nil, "", nil
				} else {
					return "", nil, "", errors.New("not implemented")
				}
			} else {
				if flowDefinitionContext := tfCtx.GetFlowDefinitionContext(); flowDefinitionContext != nil && flowDefinitionContext.GetFlowIndexComplex != nil {
					return flowDefinitionContext.GetFlowIndexComplex()
				} else {
					return coreopts.BuildOptions.FindIndexForService(tfCtx.FlowSource, tfCtx.Flow.ServiceName())
				}
			}
		}(tfContext)

		if indexErr == nil && index != "" {
			tfContext.GoMod.SectionName = index
			secondaryIndexes = secondaryI
			tfContext.GoMod.SubSectionName = indexExt
		}
		if tfContext.GoMod.SectionName != "" {
			indexValues, err = tfContext.GoMod.ListSubsection("/Index/", tfContext.FlowSource, tfContext.GoMod.SectionName, tfmContext.DriverConfig.CoreConfig.Log)
			if err != nil {
				eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
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
						subIndexValues, _ := tfContext.GoMod.ListSubsection("/Index/"+tfContext.FlowSource+"/"+tfContext.GoMod.SectionName+"/"+indexValue+"/", subSection, secondaryIndex, tfmContext.DriverConfig.CoreConfig.Log)
						if subIndexValues != nil {
							for _, subIndexValue := range subIndexValues {
								tfContext.GoMod.SectionPath = "super-secrets/Index/" + tfContext.FlowSource + "/" + tfContext.GoMod.SectionName + "/" + indexValue + "/" + subSection + "/" + secondaryIndex + "/" + subIndexValue + "/" + tfContext.Flow.ServiceName()
								row, rowErr := tfmContext.PathToTableRowHelper(tfContext)
								if rowErr != nil {
									eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, rowErr, false)
									continue
								}
								rows = append(rows, row)
							}
						} else {
							row, rowErr := tfmContext.PathToTableRowHelper(tfContext)
							if rowErr != nil {
								eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, rowErr, false)
								continue
							}
							rows = append(rows, row)
						}
					} else {
						row, rowErr := tfmContext.PathToTableRowHelper(tfContext)
						if rowErr != nil {
							eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, rowErr, false)
							continue
						}
						rows = append(rows, row)
					}
				}
			} else {
				row, rowErr := tfmContext.PathToTableRowHelper(tfContext)
				if rowErr != nil {
					eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, rowErr, false)
					continue
				}
				rows = append(rows, row)
			}
		} else {
			row, rowErr := tfmContext.PathToTableRowHelper(tfContext)
			if rowErr != nil {
				eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, rowErr, false)
				continue
			}
			rows = append(rows, row)
		}
	}

	var inserter sql.RowInserter
	//Writes accumlated rows to the table.
	tableSql, tableOk, _ := tfmContext.TierceronEngine.Database.GetTableInsensitive(nil, tfContext.Flow.TableName())
	if tableOk {
		inserter = tableSql.(*sqlememory.Table).Inserter(tfmContext.TierceronEngine.Context)
	} else {
		insertErr := errors.New("Unable to insert rows into:" + tfContext.Flow.TableName())
		eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, insertErr, false)
		return insertErr
	}
	for _, row := range rows {
		if row == nil {
			continue
		}
		if err := inserter.Insert(tfmContext.TierceronEngine.Context, row); err != nil {
			if !strings.Contains(err.Error(), "duplicate primary key") && !strings.Contains(err.Error(), "invalid type") {
				eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
			}
			continue
		}
	}
	inserter.Close(tfmContext.TierceronEngine.Context)

	inserter = nil

	return nil
}

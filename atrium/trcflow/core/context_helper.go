package core

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	flowcorehelper "github.com/trimble-oss/tierceron/atrium/trcflow/core/flowcorehelper"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/kernelopts"
	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"
	"github.com/trimble-oss/tierceron/pkg/trcx/extract"

	trcdb "github.com/trimble-oss/tierceron/atrium/trcdb"
	trcengine "github.com/trimble-oss/tierceron/atrium/trcdb/engine"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

var changesLock sync.Mutex

func getChangeIDQuery(databaseName string, changeTable string) string {
	return fmt.Sprintf("SELECT id FROM %s.%s", databaseName, changeTable)
}

// func getChangedByIDQuery(databaseName string, changeTable string, identityColumnName string, id any) string {
// 	if _, iOk := id.(int64); iOk {
// 		return fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%d'", databaseName, changeTable, identityColumnName, id)
// 	} else {
// 		return fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%s'", databaseName, changeTable, identityColumnName, id)
// 	}
// }

func getDeleteChangeQuery(databaseName string, changeTable string, id any) string {
	if _, iOk := id.(int64); iOk {
		return fmt.Sprintf("DELETE FROM %s.%s WHERE id = '%d'", databaseName, changeTable, id)
	} else {
		return fmt.Sprintf("DELETE FROM %s.%s WHERE id = '%s'", databaseName, changeTable, id)
	}
}

func getInsertChangeQuery(databaseName string, changeTable string, id any) string {
	if _, iOk := id.(int64); iOk {
		return fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES (%d, current_timestamp())", databaseName, changeTable, id)
	} else {
		return fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES ('%s', current_timestamp())", databaseName, changeTable, id)
	}
}

func getCompositeChangeIDQuery(databaseName string, changeTable string, indexColumnNames any) string {
	return fmt.Sprintf("SELECT %s, %s FROM %s.%s", indexColumnNames.([]string)[0], indexColumnNames.([]string)[1], databaseName, changeTable)
}

func getCompositeDeleteChangeQuery(databaseName string, changeTable string, indexColumnNames any, indexColumnValues any) string {
	if first, second, third, fourth := indexColumnNames.([]string)[0], indexColumnValues.([]string)[0], indexColumnNames.([]string)[1], indexColumnValues.([]string)[1]; first != "" && second != "" && third != "" && fourth != "" {
		return fmt.Sprintf("DELETE FROM %s.%s WHERE %s='%s' AND %s='%s'", databaseName, changeTable, indexColumnNames.([]string)[0], indexColumnValues.([]string)[0], indexColumnNames.([]string)[1], indexColumnValues.([]string)[1])
	}
	return ""
}

func (tfmContext *TrcFlowMachineContext) NotifyFlowComponentLoaded(tableName string) {
	switch tableName {
	case flowcore.DataFlowStatConfigurationsFlow.TableName():
		fallthrough
	case flowcore.TierceronControllerFlow.TableName():
		tfmContext.FlowMapLock.RLock()
		if tfFlowContext, refOk := tfmContext.FlowMap[flowcore.FlowNameType(tableName)]; refOk {
			tfmContext.FlowMapLock.RUnlock()
			tfFlowContext.NotifyFlowComponentLoaded()
		} else {
			tfmContext.FlowMapLock.RUnlock()
		}
	}
}

// removeChangedTableEntries -- gets and removes any changed table entries.
func (tfmContext *TrcFlowMachineContext) removeCompositeKeyChangedTableEntries(tfContext *TrcFlowContext, _ []string, indexColumnNames any) ([][]any, error) {
	var changedEntriesQuery string

	changesLock.Lock()
	changedEntriesQuery = getCompositeChangeIDQuery(tfContext.FlowHeader.SourceAlias, tfContext.ChangeFlowName, indexColumnNames)

	_, _, matrixChangedEntries, err := trcdb.Query(tfmContext.TierceronEngine, changedEntriesQuery, tfContext.QueryLock)
	if err != nil {
		eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
		return nil, err
	}
	for _, changedEntry := range matrixChangedEntries {
		indexColumnValues := []string{}
		indexColumnValues = append(indexColumnValues, changedEntry[0].(string))
		indexColumnValues = append(indexColumnValues, changedEntry[1].(string))
		_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getCompositeDeleteChangeQuery(tfContext.FlowHeader.SourceAlias, tfContext.ChangeFlowName, indexColumnNames, indexColumnValues), tfContext.QueryLock)
		if err != nil {
			eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
			return nil, err
		}
	}
	changesLock.Unlock()
	return matrixChangedEntries, nil
}

// removeChangedTableEntries -- gets and removes any changed table entries.
func (tfmContext *TrcFlowMachineContext) removeChangedTableEntries(tfContext *TrcFlowContext) ([][]any, error) {
	var changedEntriesQuery string

	changesLock.Lock()
	defer changesLock.Unlock()

	/*if isInit {
		changedEntriesQuery = `SELECT ` + identityColumnName + ` FROM ` + databaseName + `.` + tableName
	} else { */
	changedEntriesQuery = getChangeIDQuery(tfContext.FlowHeader.SourceAlias, tfContext.ChangeFlowName)
	//}

	_, _, matrixChangedEntries, err := trcdb.Query(tfmContext.TierceronEngine, changedEntriesQuery, tfContext.QueryLock)
	if err != nil {
		eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
		return nil, err
	}
	for _, changedEntry := range matrixChangedEntries {
		changedID := changedEntry[0]
		_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getDeleteChangeQuery(tfContext.FlowHeader.SourceAlias, tfContext.ChangeFlowName, changedID), tfContext.QueryLock)
		if err != nil {
			eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
			return nil, err
		}
	}
	return matrixChangedEntries, nil
}

func getStatisticChangeIDQuery(databaseName string, changeTable string, idCols []string, indexColumnNames any) string {
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

func getStatisticDeleteChangeQuery(databaseName string, changeTable string, idCols []string, idColVal any, indexColumnNames any, indexColumnValues any) string {
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

func removeElementFromSliceInterface(slice []any, ss []string) ([]any, any) {
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

	var value any = &slice

	// Now do the removal:
	sp := value.(*[]any)
	removedVal := (*sp)[indexFound]
	*sp = append((*sp)[:indexFound], (*sp)[indexFound+1:]...)
	return *sp, removedVal
}

func getStatisticChangedByIDQuery(databaseName string, changeTable string, idColumns []string, indexColumnNames any, indexColumnValues any) (string, error) {
	if indexColumnNamesSlice, iOk := indexColumnNames.([]string); iOk {
		if indexColumnValuesSlice, iOk := indexColumnValues.([]any); iOk {
			var query string
			var removedVal any
			//			var removedValName string
			if valueSliceStr, sOk := indexColumnValuesSlice[0].([]string); sOk {
				if len(idColumns) == 1 {
					query = fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%s'", databaseName, changeTable, idColumns[0], valueSliceStr[0])
				} else if len(idColumns) == 2 {
					query = fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%s' AND %s='%s'", databaseName, changeTable, idColumns[0], valueSliceStr[0], idColumns[1], valueSliceStr[1])
				} else if len(idColumns) == 3 {
					// TODO: test...
					query = fmt.Sprintf("SELECT * FROM %s.%s WHERE %s='%s' AND %s='%s' AND %s='%s'", databaseName, changeTable, idColumns[0], valueSliceStr[0], idColumns[1], valueSliceStr[1], idColumns[2], valueSliceStr[2])
				}
				if indexColumnValuesSlice, removedVal = removeElementFromSliceInterface(indexColumnValuesSlice, valueSliceStr); removedVal != nil { // this logic is for dfs...names & values appear out of order in slices at this point but is needed for previous step.

					indexColumnNamesSlice, _ = removeElementFromSlice(indexColumnNamesSlice, idColumns) //							 may need to revist if a table has 3 identifiying column names (none currently).
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
				return "", errors.New("error - unsupported type for index column - add support for new type")
			}

			if len(indexColumnNamesSlice) > 1 {
				for i := 0; i < len(indexColumnNamesSlice); i++ {
					query = fmt.Sprintf("%s AND %s='%s'", query, indexColumnNamesSlice[i], indexColumnValuesSlice[i])
				}
			}

			// if removedValName != "" { // Adding back in ordered name & val for dfs for next steps...
			//   indexColumnValuesSlice = append(indexColumnValuesSlice, removedVal)
			//   indexColumnNamesSlice = append(indexColumnNamesSlice, removedValName)
			// }

			return query, nil
		} else {
			return "", errors.New("invalid index value data for statistic data")
		}
	} else {
		return "", errors.New("invalid index name data for statistic data")
	}
}

func getStatisticInsertChangeQuery(databaseName string, changeTable string, idColVal any, indexColVal any, secIndexColVal any) string {
	if first, second, third := idColVal.(string), indexColVal.(string), secIndexColVal.(string); first != "" && second != "" && third != "" {
		return fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES ('%s', '%s', '%s', current_timestamp())", databaseName, changeTable, idColVal, indexColVal, secIndexColVal)
	}
	return ""
}

// removeChangedTableEntries -- gets and removes any changed table entries.
func (tfmContext *TrcFlowMachineContext) removeStatisticChangedTableEntries(tcflowContext flowcore.FlowContext, idCols []string, indexColumnNames any) ([][]any, error) {
	var changedEntriesQuery string
	tfContext := tcflowContext.(*TrcFlowContext)

	changesLock.Lock()
	changedEntriesQuery = getStatisticChangeIDQuery(tfContext.FlowHeader.SourceAlias, tfContext.ChangeFlowName, idCols, indexColumnNames)

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
		_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getStatisticDeleteChangeQuery(tfContext.FlowHeader.SourceAlias, tfContext.ChangeFlowName, idCols, idColVal, indexColumnNames, indexColumnValues), tfContext.QueryLock)
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
	tcflowContext flowcore.FlowContext,
	identityColumnNames []string,
	indexColumnNames any,
	mysqlPushEnabled bool,
	getIndexedPathExt func(engine any, rowDataMap map[string]any, indexColumnNames any, databaseName string, tableName string, dbCallBack func(any, map[string]any) (string, []string, [][]any, error)) (string, error),
	flowPushRemote func(flowcore.FlowContext, map[string]any) error,
) error {
	tfContext := tcflowContext.(*TrcFlowContext)

	var matrixChangedEntries [][]any
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
		var changedID any
		var changeTableError error
		changedTableQuery, changeTableError = getStatisticChangedByIDQuery(tfContext.FlowHeader.SourceAlias, tfContext.FlowHeader.TableName(), identityColumnNames, indexColumnNames, changedEntry)
		if changeTableError != nil {
			eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, changeTableError, false)
			continue
		}

		_, changedTableColumns, changedTableRowData, err := trcdb.Query(tfmContext.TierceronEngine, changedTableQuery, tfContext.QueryLock)
		if err != nil {
			eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
			continue
		}

		if len(changedTableRowData) == 0 && len(changedEntry) != 3 { // This change was a delete
			syncDelete := false
			for _, syncedTable := range coreopts.BuildOptions.GetSyncedTables() {
				if tfContext.FlowHeader.TableName() == syncedTable {
					syncDelete = true
				}
			}

			if !syncDelete {
				continue
			}

			if tfContext.FlowState.State != 0 && (tfContext.FlowState.SyncMode == "push" || tfContext.FlowState.SyncMode == "pushonce") && flowPushRemote != nil {
				// Check if it exists in trcdb
				// Writeback to mysql to delete that
				rowDataMap := map[string]any{}
				rowDataMap["Deleted"] = "true"
				rowDataMap["changedId"] = changedID
				for _, column := range changedTableColumns {
					rowDataMap[column] = ""
				}

				pushError := flowPushRemote(tfContext, rowDataMap)
				if pushError != nil {
					eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, changeTableError, false)
				}
			}

			rowDataMap := map[string]any{}
			for index, column := range indexColumnNames.([]string) {
				if _, strOk := changedEntry[index].(string); strOk && len(changedEntry[index].(string)) == 0 {
					// Invalid string index...  Skip these.
					continue
				}
				rowDataMap[column] = changedEntry[index]
			}

			indexPath, indexPathErr := getIndexedPathExt(tfmContext.TierceronEngine, rowDataMap, indexColumnNames, tfContext.FlowHeader.SourceAlias, tfContext.FlowHeader.TableName(), func(engine any, query map[string]any) (string, []string, [][]any, error) {
				return trcdb.Query(engine.(*trcengine.TierceronEngine), query["TrcQuery"].(string), tfContext.QueryLock)
			})
			if indexPathErr != nil {
				eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, indexPathErr, false)
				continue
			}

			if !tfContext.ReadOnly {
				if !strings.Contains(indexPath, "/PublicIndex/") {
					indexPath = "Index/" + tfContext.FlowHeader.Source + indexPath
					if !strings.HasSuffix(indexPath, tfContext.FlowHeader.TableName()) {
						indexPath = indexPath + "/" + tfContext.FlowHeader.TableName()
					}
				}

				deleteMap, deleteErr := tfContext.GoMod.SoftDelete(indexPath, tfContext.Logger)
				if deleteErr != nil || deleteMap != nil {
					eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, errors.New("Unable to process a delete query for "+tfContext.FlowHeader.TableName()), false)
				}
			}
			continue
		} else {
			// If this change concerns the Tierceron controller flow, update
			// the TrcFlowContext state for the flow indicated in the changed row data.
			if tfContext.FlowHeader != nil && tfContext.FlowHeader.FlowName() == flowcore.TierceronControllerFlow.FlowName() {
				if len(changedTableRowData) > 0 {
					row := changedTableRowData[0]

					// The first column must be a string flow name. If not, skip.
					targetFlowName, ok := row[0].(string)
					if !ok {
						tfmContext.Log("Controller change row did not contain a string flow name", nil)
						continue
					}

					tfmContext.FlowMapLock.RLock()
					targetTfContext, refOk := tfmContext.FlowMap[flowcore.FlowNameType(targetFlowName)]
					tfmContext.FlowMapLock.RUnlock()

					if !refOk || targetTfContext == nil {
						tfmContext.Log("Could not find flow for controller change: "+targetFlowName, nil)
						continue
					}

					// Safely copy current state then update fields found in the changed row
					curState := targetTfContext.GetFlowState().(flowcorehelper.CurrentFlowState)
					newState := curState

					for i, col := range changedTableColumns {
						val := row[i]
						switch col {
						case "state":
							switch v := val.(type) {
							case int64:
								newState.State = v
							case int:
								newState.State = int64(v)
							case string:
								if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
									newState.State = parsed
								}
							}
						case "syncMode":
							if s, ok := val.(string); ok {
								newState.SyncMode = s
							}
						case "syncFilter":
							if s, ok := val.(string); ok {
								newState.SyncFilter = s
							}
						case "flowAlias":
							if s, ok := val.(string); ok {
								newState.FlowAlias = s
							}
						}
					}

					if targetTfContext.Logger != nil {
						targetTfContext.Logger.Printf("Applying TierceronFlow change -> SetFlowState: flow=%s to=%+v", targetTfContext.FlowHeader.FlowName(), newState)
					}

					targetTfContext.SetFlowState(newState)
				}
			}
		}

		rowDataMap := map[string]any{}
		if len(changedTableRowData) == 0 {
			continue
		}
		for i, column := range changedTableColumns {
			rowDataMap[column] = changedTableRowData[0][i]
		}
		// Convert matrix/slice to table map
		// Columns are keys, values in tenantData

		// Use trigger to make another table
		indexPath, indexPathErr := getIndexedPathExt(tfmContext.TierceronEngine, rowDataMap, indexColumnNames, tfContext.FlowHeader.SourceAlias, tfContext.FlowHeader.TableName(), func(engine any, query map[string]any) (string, []string, [][]any, error) {
			return trcdb.Query(engine.(*trcengine.TierceronEngine), query["TrcQuery"].(string), tfContext.QueryLock)
		})
		if indexPathErr != nil {
			eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
			// Re-inject into changes because it might not be here yet...
			if !strings.Contains(indexPath, "PublicIndex") {
				_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getInsertChangeQuery(tfContext.FlowHeader.SourceAlias, tfContext.ChangeFlowName, changedID), tfContext.QueryLock)
				if err != nil {
					eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
				}
			} else {
				if len(changedEntry) == 3 { // Maybe there is a better way to do this, but this works for now.
					_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getStatisticInsertChangeQuery(tfContext.FlowHeader.SourceAlias, tfContext.ChangeFlowName, changedEntry[0], changedEntry[1], changedEntry[2]), tfContext.QueryLock)
					if err != nil {
						eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
					}
				}
			}
			continue
		}

		if indexPath == "" {
			continue // This case is for when SEC row can't find a matching tenant
		}

		if len(identityColumnNames) > 0 && identityColumnNames[0] == "flowName" {
			if alert, ok := rowDataMap[identityColumnNames[0]].(string); ok {
				if tfmContext.FlowControllerUpdateAlert != nil {
					tfmContext.FlowControllerUpdateAlert <- alert
				}
			}
		}

		if !tfContext.ReadOnly {
			seedError := trcvutils.SeedVaultById(tfmContext.DriverConfig, tfContext.GoMod, tfContext.FlowHeader.ServiceName(), tfmContext.DriverConfig.CoreConfig.TokenCache.VaultAddressPtr, tfContext.Vault.GetToken(), tfContext.FlowData.(*extract.TemplateResultData), rowDataMap, indexPath, tfContext.FlowHeader.Source)
			if seedError != nil {
				eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, seedError, false)
				// Re-inject into changes because it might not be here yet...
				_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getInsertChangeQuery(tfContext.FlowHeader.SourceAlias, tfContext.ChangeFlowName, changedID), tfContext.QueryLock)
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
	getIndexedPathExt func(engine any, rowDataMap map[string]any, vaultIndexColumnName string, databaseName string, tableName string, dbCallBack func(any, map[string]string) (string, []string, [][]any, error)) (string, error),
	flowPushRemote func(*TrcFlowContext, map[string]any, map[string]any) error,
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

// seedTrcDBFromVault - This loads all data from vault into TrcDB
func (tfmContext *TrcFlowMachineContext) seedTrcDBFromVault(
	tfContext *TrcFlowContext,
	filteredIndexProvidedValues []string,
) error {
	var indexValues []string = []string{}
	var secondaryIndexes []string
	var err error

	kernelID := tfmContext.GetKernelId()

	if (kernelID > 0) &&
		kernelopts.BuildOptions.IsKernel() &&
		tfContext.FlowHeader.FlowName() != flowcore.TierceronControllerFlow.FlowName() {
		// If a filtered list is provide, it'll be ok to continue even in the hive...
		// This is because we're planning to have support for "sparse" TrcDb in the hive kernel
		// for id's > 0.
		if filteredIndexProvidedValues == nil {
			// What still needs to be done here
			if tfContext.Inserter != nil {
				tfContext.Inserter.Close(tfmContext.TierceronEngine.Context)
				tfContext.Inserter = nil
			}
			return nil
		} else {
			indexValues = filteredIndexProvidedValues
		}
	}

	if tfContext.CustomSeedTrcDb != nil {
		customSeedErr := tfContext.CustomSeedTrcDb(tfmContext, tfContext)
		return customSeedErr
	}
	if tfContext.GoMod != nil {
		tfContext.GoMod.Env = tfmContext.Env
		tfContext.GoMod.Version = "0"

		index, secondaryI, indexExt, indexErr := func(tfCtx *TrcFlowContext) (string, []string, string, error) {
			if tfCtx.FlowHeader.Source == tfmContext.GetDatabaseName(tfmContext.GetFlumeDbType()) {
				if tfCtx.FlowHeader.ServiceName() == flowcore.TierceronControllerFlow.FlowName() {
					return "flowName", nil, "", nil
				} else {
					return "", nil, "", errors.New("not implemented")
				}
			} else {
				if flowDefinitionContext := tfCtx.GetFlowLibraryContext(); flowDefinitionContext != nil && flowDefinitionContext.GetFlowIndexComplex != nil {
					return flowDefinitionContext.GetFlowIndexComplex()
				} else {
					return coreopts.BuildOptions.FindIndexForService(tfmContext, tfCtx.FlowHeader.Source, tfCtx.FlowHeader.ServiceName())
				}
			}
		}(tfContext)

		if indexErr == nil && index != "" {
			tfContext.GoMod.SectionName = index
			secondaryIndexes = secondaryI
			tfContext.GoMod.SubSectionName = indexExt
		}
		if tfContext.GoMod.SectionName != "" && filteredIndexProvidedValues == nil {
			indexValues, err = tfContext.GoMod.ListSubsection("/Index/", tfContext.FlowHeader.Source, tfContext.GoMod.SectionName, tfmContext.DriverConfig.CoreConfig.Log)
			if err != nil {
				eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
				return err
			}
		}
	}

	var filteredIndexValues []string = []string{}

	if tfContext.FlowHeader.FlowName() == flowcore.TierceronControllerFlow.FlowName() {
		for _, indexValue := range indexValues {
			if !tfmContext.IsSupportedFlow(indexValue) {
				if !tfmContext.DriverConfig.CoreConfig.IsEditor {
					eUtils.LogInfo(tfmContext.DriverConfig.CoreConfig, "Skip seeding of unsupported flow: "+indexValue)
				}
				continue
			} else {
				filteredIndexValues = append(filteredIndexValues, indexValue)
			}
		}
	} else {
		filteredIndexValues = indexValues
	}

	var subSection string
	tfContext.GoMod.SectionKey = "/Index/"
	if tfContext.GoMod.SubSectionName != "" {
		subSection = tfContext.GoMod.SubSectionName
	} else {
		subSection = tfContext.FlowHeader.ServiceName()
	}
	pathTemplate := "super-secrets/Index/" + tfContext.FlowHeader.Source + "/" + tfContext.GoMod.SectionName + "/%s/" + subSection

	for _, indexValue := range filteredIndexValues {
		if indexValue != "" {
			// Loading the tables now from vault.
			tfContext.GoMod.SectionPath = fmt.Sprintf(pathTemplate, indexValue)
			if len(secondaryIndexes) > 0 {
				for _, secondaryIndex := range secondaryIndexes {
					tfContext.GoMod.SectionPath = "super-secrets/Index/" + tfContext.FlowHeader.Source + "/" + tfContext.GoMod.SectionName + "/" + indexValue + "/" + subSection + "/" + secondaryIndex
					if subSection != "" {
						subIndexValues, _ := tfContext.GoMod.ListSubsection("/Index/"+tfContext.FlowHeader.Source+"/"+tfContext.GoMod.SectionName+"/"+indexValue+"/", subSection, secondaryIndex, tfmContext.DriverConfig.CoreConfig.Log)
						if subIndexValues != nil {
							for _, subIndexValue := range subIndexValues {
								tfContext.GoMod.SectionPath = "super-secrets/Index/" + tfContext.FlowHeader.Source + "/" + tfContext.GoMod.SectionName + "/" + indexValue + "/" + subSection + "/" + secondaryIndex + "/" + subIndexValue + "/" + tfContext.FlowHeader.ServiceName()
								_, rowErr := tfmContext.PathToTableRowHelper(tfContext)
								if rowErr != nil {
									eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, rowErr, false)
									continue
								}
							}
						} else {
							_, rowErr := tfmContext.PathToTableRowHelper(tfContext)
							if rowErr != nil {
								eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, rowErr, false)
								continue
							}
						}
					} else {
						_, rowErr := tfmContext.PathToTableRowHelper(tfContext)
						if rowErr != nil {
							eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, rowErr, false)
							continue
						}
					}
				}
			} else {
				_, rowErr := tfmContext.PathToTableRowHelper(tfContext)
				if rowErr != nil {
					eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, rowErr, false)
					continue
				}
			}
		} else {
			_, rowErr := tfmContext.PathToTableRowHelper(tfContext)
			if rowErr != nil {
				eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, rowErr, false)
				continue
			}
		}
	}

	if tfContext.Inserter != nil {
		tfContext.Inserter.Close(tfmContext.TierceronEngine.Context)
		tfContext.Inserter = nil
	}

	return nil
}

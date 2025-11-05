package flumen

import (
	"errors"
	"sync"

	trcdb "github.com/trimble-oss/tierceron/atrium/trcdb"
	trcengine "github.com/trimble-oss/tierceron/atrium/trcdb/engine"
	trcflowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"
	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"
	"github.com/trimble-oss/tierceron/pkg/trcx/extract"

	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"

	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

var TierceronControllerFlow flowcore.FlowDefinition = flowcore.FlowDefinition{FlowHeader: flowcore.TierceronControllerFlow}

var changesLock sync.Mutex

func getChangeIdQuery(databaseName string, changeTable string) string {
	return "SELECT id FROM " + databaseName + `.` + changeTable
}

func getDeleteChangeQuery(databaseName string, changeTable string, id string) string {
	return "DELETE FROM " + databaseName + `.` + changeTable + " WHERE id = '" + id + "'"
}

func getInsertChangeQuery(databaseName string, changeTable string, id string) string {
	return `INSERT IGNORE INTO ` + databaseName + `.` + changeTable + `VALUES (` + id + `, current_timestamp());`
}

func FlumenProcessFlowController(tfmContext flowcore.FlowMachineContext, tfContext flowcore.FlowContext) error {
	trcTfmContext := tfmContext.(*trcflowcore.TrcFlowMachineContext)
	trcFlowContext := tfContext.(*trcflowcore.TrcFlowContext)

	switch trcFlowContext.FlowHeader.FlowNameType() {
	case TierceronControllerFlow.FlowHeader.FlowNameType():
		return ProcessTierceronFlows(trcTfmContext, trcFlowContext)
	}

	return errors.New("table not implemented")
}

func seedVaultFromChanges(tfmContext *trcflowcore.TrcFlowMachineContext,
	tfContext *trcflowcore.TrcFlowContext,
	vaultAddressPtr *string,
	identityColumnName string,
	vaultIndexColumnName string,
	isInit bool,
	getIndexedPathExt func(engine any, rowDataMap map[string]any, vaultIndexColumnName string, databaseName string, tableName string, dbCallBack func(any, string) (string, []string, [][]any, error)) (string, error),
	flowPushRemote func(map[string]any, map[string]any) error,
) error {
	var matrixChangedEntries [][]any
	var changedEntriesQuery string

	changesLock.Lock()

	/*if isInit {
		changedEntriesQuery = `SELECT ` + identityColumnName + ` FROM ` + databaseName + `.` + tableName
	} else { */
	changedEntriesQuery = getChangeIdQuery(tfContext.FlowHeader.SourceAlias, tfContext.ChangeFlowName)
	//}

	_, _, matrixChangedEntries, err := trcdb.Query(tfmContext.TierceronEngine, changedEntriesQuery, tfContext.QueryLock)
	if err != nil {
		eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
	}
	for _, changedEntry := range matrixChangedEntries {
		changedID := changedEntry[0]
		_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getDeleteChangeQuery(tfContext.FlowHeader.SourceAlias, tfContext.ChangeFlowName, changedID.(string)), tfContext.QueryLock)
		if err != nil {
			eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
		}
	}
	changesLock.Unlock()

	for _, changedEntry := range matrixChangedEntries {
		changedID := changedEntry[0]

		changedTableQuery := `SELECT * FROM ` + tfContext.FlowHeader.SourceAlias + `.` + tfContext.FlowHeader.TableName() + ` WHERE ` + identityColumnName + `='` + changedID.(string) + `'` // TODO: Implement query using changedID

		_, changedTableColumns, changedTableRowData, err := trcdb.Query(tfmContext.TierceronEngine, changedTableQuery, tfContext.QueryLock)
		if err != nil {
			eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
			continue
		}

		rowDataMap := map[string]any{}
		for i, column := range changedTableColumns {
			rowDataMap[column] = changedTableRowData[0][i]
		}
		// Convert matrix/slice to table map
		// Columns are keys, values in tenantData

		// Use trigger to make another table

		indexPath, indexPathErr := getIndexedPathExt(tfmContext.TierceronEngine, rowDataMap, vaultIndexColumnName, tfContext.FlowHeader.SourceAlias, tfContext.FlowHeader.TableName(), func(engine any, query string) (string, []string, [][]any, error) {
			return trcdb.Query(engine.(*trcengine.TierceronEngine), query, tfContext.QueryLock)
		})
		if indexPathErr != nil {
			eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, indexPathErr, false)
			// Re-inject into changes because it might not be here yet...
			_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getInsertChangeQuery(tfContext.FlowHeader.SourceAlias, tfContext.ChangeFlowName, changedID.(string)), tfContext.QueryLock)
			if err != nil {
				eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
			}
			continue
		}

		if refreshErr := tfContext.Vault.RefreshClient(); refreshErr != nil {
			// Panic situation...  Can't connect to vault... Wait until next cycle to try again.
			eUtils.LogErrorMessage(tfmContext.DriverConfig.CoreConfig, "Failure to connect to vault.  It may be down...", false)
			eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, refreshErr, false)
			continue
		}

		eUtils.LogInfo(tfmContext.DriverConfig.CoreConfig, "Attempting to seed:"+indexPath)
		seedError := trcvutils.SeedVaultById(tfmContext.DriverConfig, tfContext.GoMod, tfContext.FlowHeader.ServiceName(), vaultAddressPtr, tfmContext.Vault.GetToken(), tfContext.FlowData.(*extract.TemplateResultData), rowDataMap, indexPath, tfContext.FlowHeader.Source)
		if seedError != nil {
			eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, seedError, false)
			// Re-inject into changes because it might not be here yet...
			_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getInsertChangeQuery(tfContext.FlowHeader.SourceAlias, tfContext.ChangeFlowName, changedID.(string)), tfContext.QueryLock)
			if err != nil {
				eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
			}
			continue
		}

		// Push this change to the flow for delivery to remote data source.
		if !isInit {
			pushError := flowPushRemote(tfContext.RemoteDataSource, rowDataMap)
			if pushError != nil {
				eUtils.LogErrorObject(tfmContext.DriverConfig.CoreConfig, err, false)
			}
		}

	}

	return nil
}

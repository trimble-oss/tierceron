package flumen

import (
	"errors"
	"sync"

	trcdb "github.com/trimble-oss/tierceron/atrium/trcdb"
	trcengine "github.com/trimble-oss/tierceron/atrium/trcdb/engine"
	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"
	"github.com/trimble-oss/tierceron/pkg/trcx/extract"

	flowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"

	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

const (
	TierceronControllerFlow flowcore.FlowNameType = "TierceronFlow"
)

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

func FlumenProcessFlowController(tfmContext *flowcore.TrcFlowMachineContext, trcFlowContext *flowcore.TrcFlowContext) error {

	switch trcFlowContext.Flow {
	case TierceronControllerFlow:
		return ProcessTierceronFlows(tfmContext, trcFlowContext)
	}

	return errors.New("Table not implemented.")
}

func seedVaultFromChanges(tfmContext *flowcore.TrcFlowMachineContext,
	tfContext *flowcore.TrcFlowContext,
	vaultAddress string,
	identityColumnName string,
	vaultIndexColumnName string,
	isInit bool,
	getIndexedPathExt func(engine interface{}, rowDataMap map[string]interface{}, vaultIndexColumnName string, databaseName string, tableName string, dbCallBack func(interface{}, string) (string, []string, [][]interface{}, error)) (string, error),
	flowPushRemote func(map[string]interface{}, map[string]interface{}) error) error {

	var matrixChangedEntries [][]interface{}
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
	}
	for _, changedEntry := range matrixChangedEntries {
		changedId := changedEntry[0]
		_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getDeleteChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, changedId.(string)), tfContext.FlowLock)
		if err != nil {
			eUtils.LogErrorObject(tfmContext.Config, err, false)
		}
	}
	changesLock.Unlock()

	for _, changedEntry := range matrixChangedEntries {
		changedId := changedEntry[0]

		changedTableQuery := `SELECT * FROM ` + tfContext.FlowSourceAlias + `.` + tfContext.Flow.TableName() + ` WHERE ` + identityColumnName + `='` + changedId.(string) + `'` // TODO: Implement query using changedId

		_, changedTableColumns, changedTableRowData, err := trcdb.Query(tfmContext.TierceronEngine, changedTableQuery, tfContext.FlowLock)
		if err != nil {
			eUtils.LogErrorObject(tfmContext.Config, err, false)
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
			return trcdb.Query(engine.(*trcengine.TierceronEngine), query, tfContext.FlowLock)
		})
		if indexPathErr != nil {
			eUtils.LogErrorObject(tfmContext.Config, indexPathErr, false)
			// Re-inject into changes because it might not be here yet...
			_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getInsertChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, changedId.(string)), tfContext.FlowLock)
			if err != nil {
				eUtils.LogErrorObject(tfmContext.Config, err, false)
			}
			continue
		}

		if refreshErr := tfContext.Vault.RefreshClient(); refreshErr != nil {
			// Panic situation...  Can't connect to vault... Wait until next cycle to try again.
			eUtils.LogErrorMessage(tfmContext.Config, "Failure to connect to vault.  It may be down...", false)
			eUtils.LogErrorObject(tfmContext.Config, refreshErr, false)
			continue
		}

		eUtils.LogInfo(tfmContext.Config, "Attempting to seed:"+indexPath)
		seedError := trcvutils.SeedVaultById(tfmContext.Config, tfContext.GoMod, tfContext.Flow.ServiceName(), vaultAddress, tfmContext.Vault.GetToken(), tfContext.FlowData.(*extract.TemplateResultData), rowDataMap, indexPath, tfContext.FlowSource)
		if seedError != nil {
			eUtils.LogErrorObject(tfmContext.Config, seedError, false)
			// Re-inject into changes because it might not be here yet...
			_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getInsertChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, changedId.(string)), tfContext.FlowLock)
			if err != nil {
				eUtils.LogErrorObject(tfmContext.Config, err, false)
			}
			continue
		}

		// Push this change to the flow for delivery to remote data source.
		if !isInit {
			tfContext.FlowLock.Lock()
			pushError := flowPushRemote(tfContext.RemoteDataSource, rowDataMap)
			if pushError != nil {
				eUtils.LogErrorObject(tfmContext.Config, err, false)
			}
			tfContext.FlowLock.Unlock()
		}

	}

	return nil
}

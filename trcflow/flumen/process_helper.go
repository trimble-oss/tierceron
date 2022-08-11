package flumen

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"tierceron/trcvault/opts/insecure"
	"tierceron/trcvault/opts/prod"
	trcvutils "tierceron/trcvault/util"
	trcdb "tierceron/trcx/db"
	"tierceron/trcx/extract"
	helperkv "tierceron/vaulthelper/kv"

	flowcore "tierceron/trcflow/core"

	eUtils "tierceron/utils"
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

	_, _, matrixChangedEntries, err := trcdb.Query(tfmContext.TierceronEngine, changedEntriesQuery)
	if err != nil {
		eUtils.LogErrorObject(tfmContext.Config, err, false)
	}
	for _, changedEntry := range matrixChangedEntries {
		changedId := changedEntry[0]
		_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getDeleteChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, changedId.(string)))
		if err != nil {
			eUtils.LogErrorObject(tfmContext.Config, err, false)
		}
	}
	changesLock.Unlock()

	for _, changedEntry := range matrixChangedEntries {
		changedId := changedEntry[0]

		changedTableQuery := `SELECT * FROM ` + tfContext.FlowSourceAlias + `.` + tfContext.Flow.TableName() + ` WHERE ` + identityColumnName + `='` + changedId.(string) + `'` // TODO: Implement query using changedId

		_, changedTableColumns, changedTableRowData, err := trcdb.Query(tfmContext.TierceronEngine, changedTableQuery)
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
			return trcdb.Query(engine.(*trcdb.TierceronEngine), query)
		})
		if indexPathErr != nil {
			eUtils.LogErrorObject(tfmContext.Config, indexPathErr, false)
			// Re-inject into changes because it might not be here yet...
			_, _, _, err = trcdb.Query(tfmContext.TierceronEngine, getInsertChangeQuery(tfContext.FlowSourceAlias, tfContext.ChangeFlowName, changedId.(string)))
			if err != nil {
				eUtils.LogErrorObject(tfmContext.Config, err, false)
			}
			continue
		}

		seedError := trcvutils.SeedVaultById(tfmContext.Config, tfContext.GoMod, tfContext.Flow.ServiceName(), vaultAddress, tfmContext.Vault.GetToken(), tfContext.FlowData.(*extract.TemplateResultData), rowDataMap, indexPath, tfContext.FlowSource)
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
		if !isInit {
			pushError := flowPushRemote(tfContext.RemoteDataSource, rowDataMap)
			if pushError != nil {
				eUtils.LogErrorObject(tfmContext.Config, err, false)
			}
		}

	}

	return nil
}

//Updated deployed to true for any plugin
func PluginDeployedUpdate(mod *helperkv.Modifier, pluginNameList []string, logger *log.Logger) error {
	logger.Println("PluginDeployedUpdate start.")

	for _, pluginName := range pluginNameList {
		pluginData, err := mod.ReadData("super-secrets/Index/TrcVault/trcplugin/" + pluginName + "/Certify")
		if err != nil {
			return err
		}
		if pluginData == nil {
			if !prod.IsProd() && insecure.IsInsecure() {
				pluginData = make(map[string]interface{})
				pluginData["trcplugin"] = pluginName
				logger.Println("Checking file.")
				if imageFile, err := os.Open("/etc/opt/vault/plugins/" + pluginName); err == nil {
					sha256 := sha256.New()

					defer imageFile.Close()
					if _, err := io.Copy(sha256, imageFile); err != nil {
						continue
					}

					filesystemsha256 := fmt.Sprintf("%x", sha256.Sum(nil))
					pluginData["trcsha256"] = filesystemsha256
					pluginData["copied"] = true
				}
			} else {
				return errors.New("Plugin not certified.")
			}
		}

		if copied, okCopied := pluginData["copied"]; !okCopied || !copied.(bool) {
			logger.Println("Cannot certify plugin.  Plugin not copied: " + pluginName)
			continue
		}

		if deployed, okDeployed := pluginData["deployed"]; !okDeployed || deployed.(bool) {
			continue
		}

		writeMap := make(map[string]interface{})
		writeMap["trcplugin"] = pluginData["trcplugin"]
		writeMap["trcsha256"] = pluginData["trcsha256"]
		writeMap["copied"] = pluginData["copied"]
		writeMap["deployed"] = true

		_, err = mod.Write("super-secrets/Index/TrcVault/trcplugin/"+pluginName+"/Certify", writeMap)
		if err != nil {
			return err
		}
	}
	logger.Println("PluginDeployedUpdate complete.")
	return nil
}

func FlumenProcessFlowController(tfmContext *flowcore.TrcFlowMachineContext, trcFlowContext *flowcore.TrcFlowContext) error {

	switch trcFlowContext.Flow {
	case TierceronControllerFlow:
		return ProcessTierceronFlows(tfmContext, trcFlowContext)
	}

	return errors.New("Table not implemented.")
}

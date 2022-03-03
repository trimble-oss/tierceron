package flumen

import (
	"database/sql"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"tierceron/trcvault/util"
	"tierceron/trcx/db"
	extract "tierceron/trcx/extract"

	flowcore "tierceron/trcflow/core"
	helperkv "tierceron/vaulthelper/kv"

	flowimpl "VaultConfig.TenantConfig/util"
	testflowimpl "VaultConfig.Test/util"

	eUtils "tierceron/utils"

	sys "tierceron/vaulthelper/system"

	configcore "VaultConfig.Bootstrap/configcore"
	sqle "github.com/dolthub/go-mysql-server/sql"
)

var tableCreationLock sync.Mutex
var flowInitLock sync.Mutex
var changesLock sync.Mutex

var channelMap map[string]chan bool

func getChangeIdQuery(databaseName string, changeTable string) string {
	return "SELECT id FROM " + databaseName + `.` + changeTable
}

func getDeleteChangeQuery(databaseName string, changeTable string, id string) string {
	return "DELETE FROM " + databaseName + `.` + changeTable + " WHERE id = '" + id + "'"
}

func getInsertChangeQuery(databaseName string, changeTable string, id string) string {
	return `INSERT IGNORE INTO ` + databaseName + `.` + changeTable + `VALUES (` + id + `, current_timestamp());`
}

func getUpdateTrigger(databaseName string, tableName string, idColumnName string) string {
	return `CREATE TRIGGER tcUpdateTrigger AFTER UPDATE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + idColumnName + `, current_timestamp());` +
		` END;`
}

func getInsertTrigger(databaseName string, tableName string, idColumnName string) string {
	return `CREATE TRIGGER tcInsertTrigger AFTER INSERT ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + idColumnName + `, current_timestamp());` +
		` END;`
}

func seedVaultFromChanges(trcFlowMachineContext *flowcore.TrcFlowMachineContext,
	trcFlowContext *flowcore.TrcFlowContext,
	config *eUtils.DriverConfig,
	vaultAddress string,
	v *sys.Vault,
	identityColumnName string,
	vaultIndexColumnName string,
	isInit bool,
	flowPushRemote func(map[string]interface{}, map[string]interface{}) error) error {

	var matrixChangedEntries [][]string
	var changedEntriesQuery string

	changesLock.Lock()

	/*if isInit {
		changedEntriesQuery = `SELECT ` + identityColumnName + ` FROM ` + databaseName + `.` + tableName
	} else { */
	changedEntriesQuery = getChangeIdQuery(trcFlowContext.FlowSourceAlias, trcFlowContext.ChangeFlowName)
	//}

	_, _, matrixChangedEntries, err := db.Query(trcFlowMachineContext.TierceronEngine, changedEntriesQuery)
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
	}
	for _, changedEntry := range matrixChangedEntries {
		changedId := changedEntry[0]
		_, _, _, err = db.Query(trcFlowMachineContext.TierceronEngine, getDeleteChangeQuery(trcFlowContext.FlowSourceAlias, trcFlowContext.ChangeFlowName, changedId))
		if err != nil {
			eUtils.LogErrorObject(config, err, false)
		}
	}
	changesLock.Unlock()

	for _, changedEntry := range matrixChangedEntries {
		changedId := changedEntry[0]

		changedTableQuery := `SELECT * FROM ` + trcFlowContext.FlowSourceAlias + `.` + trcFlowContext.FlowName + ` WHERE ` + identityColumnName + `='` + changedId + `'` // TODO: Implement query using changedId

		_, changedTableColumns, changedTableRowData, err := db.Query(trcFlowMachineContext.TierceronEngine, changedTableQuery)
		if err != nil {
			eUtils.LogErrorObject(config, err, false)
			continue
		}

		rowDataMap := map[string]interface{}{}
		for i, column := range changedTableColumns {
			rowDataMap[column] = changedTableRowData[0][i]
		}
		// Convert matrix/slice to tenantConfiguration map
		// Columns are keys, values in tenantData

		//Use trigger to make another table
		//Check for tenantId

		// TODO: This should be simplified to lib.GetIndexedPathExt() -- replace below
		indexPath, indexPathErr := flowimpl.GetIndexedPathExt(trcFlowMachineContext.TierceronEngine, rowDataMap, vaultIndexColumnName, trcFlowContext.FlowSourceAlias, trcFlowContext.FlowName, func(engine interface{}, query string) (string, []string, [][]string, error) {
			return db.Query(engine.(*db.TierceronEngine), query)
		})
		if indexPathErr != nil {
			eUtils.LogErrorObject(config, indexPathErr, false)
			// Re-inject into changes because it might not be here yet...
			_, _, _, err = db.Query(trcFlowMachineContext.TierceronEngine, getInsertChangeQuery(trcFlowContext.FlowSourceAlias, trcFlowContext.ChangeFlowName, changedId))
			if err != nil {
				eUtils.LogErrorObject(config, err, false)
			}
			continue
		}

		// TODO: This should be simplified to lib.GetIndexedPathExt() -- replace above
		seedError := util.SeedVaultById(config, trcFlowContext.GoMod, trcFlowContext.FlowService, vaultAddress, v.GetToken(), trcFlowContext.FlowData.(*extract.TemplateResultData), rowDataMap, indexPath, trcFlowContext.FlowSource)
		if seedError != nil {
			eUtils.LogErrorObject(config, seedError, false)
			// Re-inject into changes because it might not be here yet...
			_, _, _, err = db.Query(trcFlowMachineContext.TierceronEngine, getInsertChangeQuery(trcFlowContext.FlowSourceAlias, trcFlowContext.ChangeFlowName, changedId))
			if err != nil {
				eUtils.LogErrorObject(config, err, false)
			}
			continue
		}

		// Push this change to the flow for delivery to remote data source.
		if !isInit {
			pushError := flowPushRemote(trcFlowContext.RemoteDataSource, rowDataMap)
			if pushError != nil {
				eUtils.LogErrorObject(config, err, false)
			}
		}

	}

	return nil
}

func ProcessFlow(trcFlowMachineContext *flowcore.TrcFlowMachineContext,
	trcFlowContext *flowcore.TrcFlowContext,
	config *eUtils.DriverConfig,
	vaultDatabaseConfig map[string]interface{}, // TODO: actually use this to set up a mysql facade.
	vaultAddress string,
	sourceDatabaseConnectionMap map[string]interface{},
	vault *sys.Vault,
	flow string,
	flowType flowcore.FlowType,
	changedChannel chan bool,
	signalChannel chan os.Signal) error {

	// 	i. Init engine
	//     a. Get project, service, and table config template name.
	if flowType == flowcore.TableSyncFlow {
		trcFlowContext.FlowSource, trcFlowContext.FlowService, trcFlowContext.FlowPath = eUtils.GetProjectService(flow)

		trcFlowContext.FlowName = eUtils.GetTemplateFileName(trcFlowContext.FlowPath, trcFlowContext.FlowService)
		trcFlowContext.ChangeFlowName = trcFlowContext.FlowName + "_Changes"

		flowInitLock.Lock()
		trcFlowMachineContext.FlowMap[trcFlowContext.FlowService] = trcFlowContext
		flowInitLock.Unlock()

		// Set up schema callback for table to track.
		trcFlowMachineContext.CallAddTableSchema = func(tableSchema sqle.PrimaryKeySchema, tableName string) {
			// Create table if necessary.
			tableCreationLock.Lock()
			if _, ok, _ := trcFlowMachineContext.TierceronEngine.Database.GetTableInsensitive(trcFlowMachineContext.TierceronEngine.Context, tableName); !ok {
				//	ii. Init database and tables in local mysql engine instance.
				err := trcFlowMachineContext.TierceronEngine.Database.CreateTable(trcFlowMachineContext.TierceronEngine.Context, tableName, tableSchema)
				if err != nil {
					eUtils.LogErrorObject(config, err, false)
				}
			}
			tableCreationLock.Unlock()
		}

		// Set up call back to enable a trigger to track
		// whenever a row in a table changes...
		trcFlowMachineContext.CallCreateTableTriggers = func(trcfc *flowcore.TrcFlowContext, identityColumnName string) {
			//Create triggers
			var updTrigger sqle.TriggerDefinition
			var insTrigger sqle.TriggerDefinition
			insTrigger.Name = "tcInsertTrigger_" + trcfc.FlowName
			updTrigger.Name = "tcUpdateTrigger_" + trcfc.FlowName
			//Prevent duplicate triggers from existing
			existingTriggers, err := trcFlowMachineContext.TierceronEngine.Database.GetTriggers(trcFlowMachineContext.TierceronEngine.Context)
			if err != nil {
				eUtils.CheckError(config, err, false)
			}

			triggerExist := false
			for _, trigger := range existingTriggers {
				if trigger.Name == insTrigger.Name || trigger.Name == updTrigger.Name {
					triggerExist = true
				}
			}
			if !triggerExist {
				updTrigger.CreateStatement = getUpdateTrigger(trcFlowMachineContext.TierceronEngine.Database.Name(), trcfc.FlowName, identityColumnName)
				insTrigger.CreateStatement = getInsertTrigger(trcFlowMachineContext.TierceronEngine.Database.Name(), trcfc.FlowName, identityColumnName)
				trcFlowMachineContext.TierceronEngine.Database.CreateTrigger(trcFlowMachineContext.TierceronEngine.Context, updTrigger)
				trcFlowMachineContext.TierceronEngine.Database.CreateTrigger(trcFlowMachineContext.TierceronEngine.Context, insTrigger)
			}
		}

		// 3. Create a base seed template for use in vault seed process.
		var baseTableTemplate extract.TemplateResultData
		util.LoadBaseTemplate(config, &baseTableTemplate, trcFlowContext.GoMod, trcFlowContext.FlowSource, trcFlowContext.FlowService, trcFlowContext.FlowPath)
		trcFlowContext.FlowData = &baseTableTemplate

		// When called sets up an infinite loop listening for changes on either
		// the changedChannel or checks itself every 3 minutes for changes to
		// its own tables.
		trcFlowMachineContext.CallSyncTableCycle = func(trcfc *flowcore.TrcFlowContext, identityColumnName string, vaultIndexColumnName string, flowPushRemote func(map[string]interface{}, map[string]interface{}) error) {
			afterTime := time.Duration(time.Second * 20)
			isInit := true
			for {
				select {
				case <-signalChannel:
					eUtils.LogErrorMessage(config, "Receiving shutdown presumably from vault.", true)
					os.Exit(0)
				case <-changedChannel:
					seedVaultFromChanges(trcFlowMachineContext,
						trcfc,
						config,
						vaultAddress,
						vault,
						identityColumnName,
						vaultIndexColumnName,
						false,
						flowPushRemote)
				case <-time.After(afterTime):
					afterTime = time.Minute * 3
					eUtils.LogInfo(config, "3 minutes... checking local mysql for changes.")
					seedVaultFromChanges(trcFlowMachineContext,
						trcfc,
						config,
						vaultAddress,
						vault,
						identityColumnName,
						vaultIndexColumnName,
						isInit,
						flowPushRemote)
					isInit = false
				}
			}
		}
	} else {
		// Use the flow name directly.
		trcFlowContext.FlowName = flow
		trcFlowContext.FlowSource = flow
	}

	trcFlowMachineContext.CallGetFlowConfiguration = func(trcfc *flowcore.TrcFlowContext, flowTemplatePath string) (map[string]interface{}, bool) {
		flowProject, flowService, flowConfigTemplatePath := eUtils.GetProjectService(flowTemplatePath)
		flowConfigTemplateName := eUtils.GetTemplateFileName(flowConfigTemplatePath, flowService)
		trcfc.GoMod.SectionKey = "/Restricted/"
		trcfc.GoMod.SectionName = flowService
		trcfc.GoMod.SubSectionValue = flowService
		properties, err := util.NewProperties(config, vault, trcfc.GoMod, trcFlowMachineContext.Env, flowProject, flowService)
		if err != nil {
			return nil, false
		}
		return properties.GetConfigValues(flowService, flowConfigTemplateName)
	}

	// Make a call on Call back to insert or update using the provided query.
	// If this is expected to result in a change to an existing table, thern trigger
	// something to the changed channel.
	trcFlowMachineContext.CallDBQuery = func(trcfc *flowcore.TrcFlowContext, query string, changed bool, operation string, flowNotifications []string) [][]string {
		if operation == "INSERT" {
			_, _, matrix, err := db.Query(trcFlowMachineContext.TierceronEngine, query)
			if err != nil {
				eUtils.LogErrorObject(config, err, false)
			}
			if changed && len(matrix) > 0 {
				if changedChannel != nil {
					changedChannel <- true
				}
				if len(flowNotifications) > 0 {
					// look up channels and notify them too.
					for _, flowNotification := range flowNotifications {
						if notifFlowCxt, ok := trcFlowMachineContext.FlowMap[flowNotification]; ok {
							if notificationFlowChannel, ok := channelMap[notifFlowCxt.(*flowcore.TrcFlowContext).FlowPath]; ok {
								notificationFlowChannel <- true
							}
						}
					}
				}
			}
		} else if operation == "UPDATE" {
			tableName, _, matrix, err := db.Query(trcFlowMachineContext.TierceronEngine, query)
			if err != nil {
				eUtils.LogErrorObject(config, err, false)
			}
			if changed && (len(matrix) > 0 || tableName != "") {
				if changedChannel != nil {
					changedChannel <- true
				}
				if len(flowNotifications) > 0 {
					// look up channels and notify them too.
					for _, flowNotification := range flowNotifications {
						if notifFlowCxt, ok := trcFlowMachineContext.FlowMap[flowNotification]; ok {
							if notificationFlowChannel, ok := channelMap[notifFlowCxt.(*flowcore.TrcFlowContext).FlowPath]; ok {
								notificationFlowChannel <- true
							}
						}
					}
				}
			}
		} else if operation == "SELECT" {
			_, _, matrixChangedEntries, err := db.Query(trcFlowMachineContext.TierceronEngine, query)
			if err != nil {
				eUtils.LogErrorObject(config, err, false)
			}
			return matrixChangedEntries
		}
		return nil
	}

	// Open a database connection to the provided source using provided
	// source configurations.
	trcFlowMachineContext.CallGetDbConn = func(dbUrl string, username string, sourceDBConfig map[string]interface{}) (*sql.DB, error) {
		return util.OpenDirectConnection(config, dbUrl,
			username,
			configcore.DecryptSecretConfig(sourceDBConfig, sourceDatabaseConnectionMap))
	}

	// Utilizing provided api auth headers, endpoint, and body data
	// this CB makes a call on behalf of the caller and returns a map
	// representation of json data provided by the endpoint.
	trcFlowMachineContext.CallAPI = func(apiAuthHeaders map[string]string, host string, apiEndpoint string, bodyData io.Reader, getOrPost bool) (map[string]interface{}, error) {
		httpClient, err := helperkv.CreateHTTPClient(false, host, trcFlowMachineContext.Env, false)
		if err != nil {
			return nil, err
		}
		if getOrPost {
			return util.GetJSONFromClientByGet(config, httpClient, apiAuthHeaders, apiEndpoint, bodyData)
		}
		return util.GetJSONFromClientByPost(config, httpClient, apiAuthHeaders, apiEndpoint, bodyData)
	}

	// Create remote data source with only what is needed.
	trcFlowContext.RemoteDataSource["dbsourceregion"] = sourceDatabaseConnectionMap["dbsourceregion"]
	trcFlowContext.RemoteDataSource["dbingestinterval"] = sourceDatabaseConnectionMap["dbingestinterval"]

	dbsourceConn, err := util.OpenDirectConnection(config, sourceDatabaseConnectionMap["dbsourceurl"].(string), sourceDatabaseConnectionMap["dbsourceuser"].(string), sourceDatabaseConnectionMap["dbsourcepassword"].(string))

	if err != nil {
		eUtils.LogErrorMessage(config, "Couldn't get dedicated database connection.", false)
		return err
	}
	defer dbsourceConn.Close()

	trcFlowContext.RemoteDataSource["connection"] = dbsourceConn

	trcFlowMachineContext.CallLog = func(msg string, err error) {
		if err != nil {
			eUtils.LogErrorObject(config, err, false)
		} else {
			eUtils.LogInfo(config, msg)
		}
	}

	if flowType == flowcore.TableSyncFlow || flowType == flowcore.TableEnrichFlow {
		flowError := flowimpl.ProcessFlowController(trcFlowMachineContext, trcFlowContext)
		if flowError != nil {
			eUtils.LogErrorObject(config, flowError, true)
		}
	} else if flowType == flowcore.TableTestFlow {
		flowError := testflowimpl.ProcessTestFlowController(trcFlowMachineContext, trcFlowContext)
		if flowError != nil {
			eUtils.LogErrorObject(config, flowError, true)
		}
	}

	return nil
}

func ProcessFlows(pluginConfig map[string]interface{}, logger *log.Logger) error {
	// 1. Get Plugin configurations.
	trcFlowMachineContext := flowcore.TrcFlowMachineContext{}
	var config *eUtils.DriverConfig
	var vault *sys.Vault
	var goMod *helperkv.Modifier
	var err error

	trcFlowMachineContext.Env = pluginConfig["env"].(string)
	config, goMod, vault, err = eUtils.InitVaultModForPlugin(pluginConfig, logger)
	if err != nil {
		eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start.", false)
		return err
	}
	projects, services, _ := eUtils.GetProjectServices(pluginConfig["connectionPath"].([]string))
	var sourceDatabaseConfigs []map[string]interface{}
	var vaultDatabaseConfig map[string]interface{}
	var trcIdentityConfig map[string]interface{}

	for i := 0; i < len(projects); i++ {

		var indexValues []string

		if services[i] == "Database" {
			// TODO: This could be an api call vault list - to list what's available with rid's.
			// East and west...
			goMod.SectionName = "regionId"
			goMod.SectionKey = "/Index/"
			regions, err := goMod.ListSubsection("/Index/", projects[i], goMod.SectionName)
			if err != nil {
				eUtils.LogErrorObject(config, err, false)
				return err
			}
			indexValues = regions
		} else {
			indexValues = []string{""}
		}

		for _, indexValue := range indexValues {
			goMod.SubSectionValue = indexValue
			ok := false
			properties, err := util.NewProperties(config, vault, goMod, pluginConfig["env"].(string), projects[i], services[i])
			if err != nil {
				eUtils.LogErrorObject(config, err, false)
				return err
			}

			switch services[i] {
			case "Database":
				var sourceDatabaseConfig map[string]interface{}
				sourceDatabaseConfig, ok = properties.GetConfigValues(services[i], "config")
				if !ok || len(sourceDatabaseConfig) == 0 {
					// Just ignore this one and go to the next one.
					eUtils.LogWarningMessage(config, "Expected database configuration does not exist: "+indexValue, false)
					continue
				}
				// Chewbacca -- remove if check.
				if sourceDatabaseConfig["dbsourceregion"] == "west" {
					sourceDatabaseConfigs = append(sourceDatabaseConfigs, sourceDatabaseConfig)
				}

			case "Identity":
				trcIdentityConfig, ok = properties.GetConfigValues(services[i], "config")
				if !ok {
					eUtils.LogErrorMessage(config, "Couldn't get config values.", false)
					return err
				}
			case "VaultDatabase":
				vaultDatabaseConfig, ok = properties.GetConfigValues(services[i], "config")
				if !ok {
					eUtils.LogErrorMessage(config, "Couldn't get config values.", false)
					return err
				}
			}
		}

	}
	sourceDatabaseConnectionsMap := map[string]map[string]interface{}{}

	// 4. Create config for vault for queries to vault.
	emptySlice := []string{""}

	configBasis := eUtils.DriverConfig{
		Regions:      emptySlice,
		Insecure:     pluginConfig["insecure"].(bool),
		Token:        pluginConfig["token"].(string),
		VaultAddress: pluginConfig["address"].(string),
		Env:          pluginConfig["env"].(string),
		Log:          logger,
	}

	tableList := pluginConfig["templatePath"].([]string)

	for _, table := range tableList {
		_, service, tableTemplateName := eUtils.GetProjectService(table)
		tableName := eUtils.GetTemplateFileName(tableTemplateName, service)
		configBasis.VersionFilter = append(configBasis.VersionFilter, tableName)
	}

	trcFlowMachineContext.TierceronEngine, err = db.CreateEngine(&configBasis, tableList, pluginConfig["env"].(string), flowimpl.GetDatabaseName())
	if err != nil {
		eUtils.LogErrorMessage(config, "Couldn't build engine.", false)
		return err
	}

	// 2. Establish mysql connection to remote mysql instance.
	for _, sourceDatabaseConfig := range sourceDatabaseConfigs {
		dbSourceConnBundle := map[string]interface{}{}
		dbSourceConnBundle["dbsourceurl"] = sourceDatabaseConfig["dbsourceurl"].(string)
		dbSourceConnBundle["dbsourceuser"] = sourceDatabaseConfig["dbsourceuser"].(string)
		dbSourceConnBundle["dbsourcepassword"] = sourceDatabaseConfig["dbsourcepassword"].(string)
		dbSourceConnBundle["dbsourceregion"] = sourceDatabaseConfig["dbsourceregion"].(string)

		dbSourceConnBundle["encryptionSecret"] = sourceDatabaseConfig["dbencryptionSecret"].(string)
		if dbIngestInterval, ok := sourceDatabaseConfig["dbingestinterval"]; ok {
			ingestInterval, err := strconv.ParseInt(dbIngestInterval.(string), 10, 64)
			if err == nil {
				eUtils.LogInfo(config, "Ingest interval: "+dbIngestInterval.(string))
				dbSourceConnBundle["dbingestinterval"] = time.Duration(ingestInterval)
			}
		} else {
			eUtils.LogErrorMessage(config, "Ingest interval invalid - Defaulting to 60 minutes.", false)
			dbSourceConnBundle["dbingestinterval"] = time.Duration(60000)
		}

		sourceDatabaseConnectionsMap[sourceDatabaseConfig["dbsourceregion"].(string)] = dbSourceConnBundle
	}

	// Http query resources include:
	// 1. Auth -- Auth is provided by the external library tcutil.
	// 2. Get json by Api call.
	extensionAuthComponents := flowimpl.GetExtensionAuthComponents(trcIdentityConfig)
	httpClient, err := helperkv.CreateHTTPClient(false, extensionAuthComponents["authDomain"].(string), pluginConfig["env"].(string), false)
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return err
	}

	trcFlowMachineContext.ExtensionAuthData, err = util.GetJSONFromClientByPost(config, httpClient, extensionAuthComponents["authHeaders"].(map[string]string), extensionAuthComponents["authUrl"].(string), extensionAuthComponents["bodyData"].(io.Reader))
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return err
	}

	// 2. Initialize Engine and create changes table.
	trcFlowMachineContext.TierceronEngine.Context = sqle.NewEmptyContext()
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	var wg sync.WaitGroup

	for _, tableName := range configBasis.VersionFilter {
		changeTableName := tableName + "_Changes"

		if _, ok, _ := trcFlowMachineContext.TierceronEngine.Database.GetTableInsensitive(trcFlowMachineContext.TierceronEngine.Context, changeTableName); !ok {
			eUtils.LogInfo(config, "Creating tierceron sql table: "+changeTableName)
			tableCreationLock.Lock()
			err := trcFlowMachineContext.TierceronEngine.Database.CreateTable(trcFlowMachineContext.TierceronEngine.Context, changeTableName, sqle.NewPrimaryKeySchema(sqle.Schema{
				{Name: "id", Type: sqle.Text, Source: changeTableName, PrimaryKey: true},
				{Name: "updateTime", Type: sqle.Timestamp, Source: changeTableName},
			}))
			tableCreationLock.Unlock()
			if err != nil {
				eUtils.LogErrorObject(config, err, false)
				return err
			}
		}
	}
	eUtils.LogInfo(config, "Tables creation completed.")

	channelMap = make(map[string]chan bool)
	for _, table := range tableList {
		channelMap[table] = make(chan bool, 5)
	}

	for _, flowName := range flowimpl.GetAdditionalFlows() {
		channelMap[flowName] = make(chan bool, 5)
	}

	trcFlowMachineContext.FlowMap = make(map[string]interface{})
	for _, sourceDatabaseConnectionMap := range sourceDatabaseConnectionsMap {
		for _, table := range tableList {
			wg.Add(1)
			go func(t string) {
				eUtils.LogInfo(config, "Beginning flow: "+t)
				defer wg.Done()
				trcFlowContext := flowcore.TrcFlowContext{RemoteDataSource: map[string]interface{}{}}
				var flowVault *sys.Vault
				config, trcFlowContext.GoMod, flowVault, err = eUtils.InitVaultModForPlugin(pluginConfig, logger)
				if err != nil {
					eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start flow.", false)
					return
				}
				trcFlowContext.FlowSourceAlias = flowimpl.GetDatabaseName()

				ProcessFlow(&trcFlowMachineContext,
					&trcFlowContext,
					config,
					vaultDatabaseConfig,
					pluginConfig["address"].(string),
					sourceDatabaseConnectionMap,
					flowVault,
					t,
					flowcore.TableSyncFlow,
					channelMap[t], // tableChangedChannel
					signalChannel,
				)
			}(table)
		}
		for _, flowName := range flowimpl.GetAdditionalFlows() {
			wg.Add(1)
			go func(f string) {
				eUtils.LogInfo(config, "Beginning flow: "+f)
				defer wg.Done()
				trcFlowContext := flowcore.TrcFlowContext{RemoteDataSource: map[string]interface{}{}}
				var flowVault *sys.Vault
				config, trcFlowContext.GoMod, flowVault, err = eUtils.InitVaultModForPlugin(pluginConfig, logger)
				if err != nil {
					eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start flow.", false)
					return
				}

				ProcessFlow(&trcFlowMachineContext,
					&trcFlowContext,
					config,
					vaultDatabaseConfig,
					pluginConfig["address"].(string),
					sourceDatabaseConnectionMap,
					flowVault,
					f,
					flowcore.TableEnrichFlow,
					channelMap[f], // tableChangedChannel
					signalChannel,
				)
			}(flowName)
		}

		for _, flowName := range testflowimpl.GetAdditionalFlows() {
			wg.Add(1)
			go func(f string) {
				eUtils.LogInfo(config, "Beginning flow: "+f)
				defer wg.Done()
				trcFlowContext := flowcore.TrcFlowContext{RemoteDataSource: map[string]interface{}{}}
				var flowVault *sys.Vault
				config, trcFlowContext.GoMod, flowVault, err = eUtils.InitVaultModForPlugin(pluginConfig, logger)
				if err != nil {
					eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start flow.", false)
					return
				}

				ProcessFlow(&trcFlowMachineContext,
					&trcFlowContext,
					config,
					vaultDatabaseConfig,
					pluginConfig["address"].(string),
					sourceDatabaseConnectionMap,
					flowVault,
					f,
					flowcore.TableTestFlow,
					channelMap[f], // tableChangedChannel
					signalChannel,
				)
			}(flowName)
		}
	}

	wg.Wait()

	// 5. Implement write backs to vault from our TierceronEngine ....  if an enterpriseId appears... then write it to vault...
	//    somehow you need to track if something is a new entry...  like a rowChangedSlice...

	// :AutoRegistration
	//    -- Query Spectrum to find an administrator...  Also figure out an EnterpriseName?  EnterpriseId? Other stuff....
	//       -- Get auth token to be able to call AutoRegistration some how...
	//       -- Call AutoRegistration...
	//
	// Other things we can do:
	//     I. Write config files for rest of tables in mysql:
	//        KafkaTableConfiguration, MysqlFile, ReportJobs, SpectrumEnterpriseConfig, TenantConfiguration (done*), Tokens
	//        In order of priority: TenantConfiguration, SpectrumEnterpriseConfig, Mysqlfile, KafkaTableConfiguration (vault feature needed?), ReportJobs, Tokens?
	//     II. Open up mysql port and performance test queries...
	//         -- create a mysql client runner... I bet there are go libraries that let you do this...
	//     I don't wanna do this...
	//     d. Optionally update fieldtech TenantConfiguration back to mysql.
	//
	return nil
}

package util

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

	"tierceron/trcx/db"
	extract "tierceron/trcx/extract"

	helperkv "tierceron/vaulthelper/kv"

	eUtils "tierceron/utils"

	tcutil "VaultConfig.TenantConfig/util"

	sys "tierceron/vaulthelper/system"

	configcore "VaultConfig.Bootstrap/configcore"
	sqle "github.com/dolthub/go-mysql-server/sql"
)

type FlowType int64

var m sync.Mutex

var channelMap map[string]chan bool

const (
	TableSyncFlow FlowType = iota
	TableEnrichFlow
)

func getChangeIdQuery(databaseName string, changeTable string) string {
	return "SELECT id FROM " + databaseName + `.` + changeTable
}

func getDeleteChangeQuery(databaseName string, changeTable string, id string) string {
	return "DELETE FROM " + databaseName + `.` + changeTable + " WHERE id = '" + id + "'"
}

func getUpdateTrigger(databaseName string, tableName string, idColumnName string) string {
	return `CREATE TRIGGER tcUpdateTrigger BEFORE UPDATE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` UPDATE ` + databaseName + `.` + tableName + `_Changes SET id=new.tenantId, updateTime=current_timestamp() WHERE EXISTS (select id from ` + databaseName + `.` + tableName + `_Changes where id=new.` + idColumnName + `);` +
		` END;`
}

func getInsertTrigger(databaseName string, tableName string, idColumnName string) string {
	return `CREATE TRIGGER tcInsertTrigger BEFORE INSERT ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + idColumnName + `, current_timestamp());` +
		` END;`
}

func seedVaultFromChanges(tierceronEngine *db.TierceronEngine,
	goMod *helperkv.Modifier,
	vaultAddress string,
	baseTableTemplate *extract.TemplateResultData,
	service string,
	v *sys.Vault,
	databaseName string,
	tableName string,
	identityColumnName string,
	changeTable string,
	vaultIndexColumnName string,
	isInit bool,
	remoteDataSource map[string]interface{},
	flowPushRemote func(map[string]interface{}, map[string]interface{}) error,
	logger *log.Logger,
	flowSource string) error {

	var matrixChangedEntries [][]string
	var changedEntriesQuery string

	if isInit {
		changedEntriesQuery = `SELECT ` + identityColumnName + ` FROM ` + databaseName + `.` + tableName
	} else {
		changedEntriesQuery = getChangeIdQuery(databaseName, changeTable)
	}

	_, _, matrixChangedEntries, err := db.Query(tierceronEngine, changedEntriesQuery)
	if err != nil {
		eUtils.LogErrorObject(err, logger, false)
	}

	for _, changedEntry := range matrixChangedEntries {
		changedId := changedEntry[0]

		changedTableQuery := `SELECT * FROM ` + databaseName + `.` + tableName + ` WHERE ` + identityColumnName + `='` + changedId + `'` // TODO: Implement query using changedId

		_, changedTableColumns, changedTableRowData, err := db.Query(tierceronEngine, changedTableQuery)
		if err != nil {
			eUtils.LogErrorObject(err, logger, false)
			return err
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
		indexPath, indexPathErr := tcutil.GetIndexedPathExt(tierceronEngine, rowDataMap, vaultIndexColumnName, databaseName, tableName, func(engine interface{}, query string) (string, []string, [][]string, error) {
			return db.Query(engine.(*db.TierceronEngine), query)
		})
		if indexPathErr != nil {
			eUtils.LogErrorObject(indexPathErr, logger, false)
			return indexPathErr
		}

		// TODO: This should be simplified to lib.GetIndexedPathExt() -- replace above
		seedError := SeedVaultById(goMod, service, vaultAddress, v.GetToken(), baseTableTemplate, rowDataMap, indexPath, logger, flowSource)
		if seedError != nil {
			eUtils.LogErrorObject(seedError, logger, false)
			return seedError
		}

		// Push this change to the flow for delivery to remote data source.
		if !isInit {
			pushError := flowPushRemote(remoteDataSource, rowDataMap)
			if pushError != nil {
				eUtils.LogErrorObject(err, logger, false)
			}

			_, _, _, err = db.Query(tierceronEngine, getDeleteChangeQuery(databaseName, changeTable, changedId))
			if err != nil {
				eUtils.LogErrorObject(err, logger, false)
			}
		}

	}

	return nil
}

func ProcessFlow(tierceronEngine *db.TierceronEngine,
	identityConfig map[string]interface{},
	vaultDatabaseConfig map[string]interface{},
	vaultAddress string,
	goMod *helperkv.Modifier,
	sourceDatabaseConnectionMap map[string]interface{},
	vault *sys.Vault,
	authData map[string]interface{},
	env string,
	flow string,
	flowType FlowType,
	changedChannel chan bool,
	signalChannel chan os.Signal,
	logger *log.Logger) error {

	var flowSource string
	var flowName string
	var initTableSchemaCB func(tableSchema sqle.PrimaryKeySchema, tableName string)
	var createTableTriggersCB func(identityColumnName string)
	var applyDBQueryCB func(query string, changed bool, operation string, flowNotifications []string) [][]string
	var getFlowConfiguration func(flowTemplatePath string) (map[string]interface{}, bool)
	var initSeedVaultListenerCB func(remoteDataSource map[string]interface{}, identityColumnName string, vaultIndexColumnName string, flowPushRemote func(map[string]interface{}, map[string]interface{}) error)

	// 	i. Init engine
	//     a. Get project, service, and table config template name.
	if flowType == TableSyncFlow {
		flowSource, service, tableTemplateName := eUtils.GetProjectService(flow)
		flowName = eUtils.GetTemplateFileName(tableTemplateName, service)
		changeFlowName := flowName + "_Changes"

		// Set up schema callback for table to track.
		initTableSchemaCB = func(tableSchema sqle.PrimaryKeySchema, tableName string) {
			// Create table if necessary.
			m.Lock()
			if _, ok, _ := tierceronEngine.Database.GetTableInsensitive(tierceronEngine.Context, tableName); !ok {
				//	ii. Init database and tables in local mysql engine instance.
				err := tierceronEngine.Database.CreateTable(tierceronEngine.Context, tableName, tableSchema)
				if err != nil {
					eUtils.LogErrorObject(err, logger, false)
				}
			}
			m.Unlock()
		}

		// Set up call back to enable a trigger to track
		// whenever a row in a table changes...
		createTableTriggersCB = func(identityColumnName string) {
			//Create triggers
			var updTrigger sqle.TriggerDefinition
			var insTrigger sqle.TriggerDefinition
			insTrigger.Name = "tcInsertTrigger"
			updTrigger.Name = "tcUpdateTrigger"
			//Prevent duplicate triggers from existing
			existingTriggers, err := tierceronEngine.Database.GetTriggers(tierceronEngine.Context)
			if err != nil {
				eUtils.CheckError(err, false)
			}

			triggerExist := false
			for _, trigger := range existingTriggers {
				if trigger.Name == insTrigger.Name || trigger.Name == updTrigger.Name {
					triggerExist = true
				}
			}
			if !triggerExist {
				updTrigger.CreateStatement = getUpdateTrigger(tierceronEngine.Database.Name(), flowName, identityColumnName)
				insTrigger.CreateStatement = getInsertTrigger(tierceronEngine.Database.Name(), flowName, identityColumnName)
				tierceronEngine.Database.CreateTrigger(tierceronEngine.Context, updTrigger)
				tierceronEngine.Database.CreateTrigger(tierceronEngine.Context, insTrigger)
			}
		}

		// 3. Write seed data to vault
		var baseTableTemplate extract.TemplateResultData
		LoadBaseTemplate(&baseTableTemplate, goMod, flowSource, service, flow, false, logger)

		// When called sets up an infinite loop listening for changes on either
		// the changedChannel or checks itself every 3 minutes for changes to
		// its own tables.
		initSeedVaultListenerCB = func(remoteDataSource map[string]interface{}, identityColumnName string, vaultIndexColumnName string, flowPushRemote func(map[string]interface{}, map[string]interface{}) error) {
			afterTime := time.Duration(time.Second * 10)
			isInit := true
			for {
				select {
				case <-signalChannel:
					eUtils.LogErrorMessage("Receiving shutdown presumably from vault.", logger, true)
					os.Exit(0)
				case <-changedChannel:
					seedVaultFromChanges(tierceronEngine,
						goMod,
						vaultAddress,
						&baseTableTemplate,
						service,
						vault,
						tierceronEngine.Database.Name(),
						flowName,
						identityColumnName,
						changeFlowName,
						vaultIndexColumnName,
						false,
						remoteDataSource,
						flowPushRemote,
						logger,
						flowSource)
				case <-time.After(afterTime):
					afterTime = time.Minute * 3
					eUtils.LogInfo("3 minutes... checking local mysql for changes.", logger)
					seedVaultFromChanges(tierceronEngine,
						goMod,
						vaultAddress,
						&baseTableTemplate,
						service,
						vault,
						tierceronEngine.Database.Name(),
						flowName,
						identityColumnName,
						changeFlowName,
						vaultIndexColumnName,
						isInit,
						remoteDataSource,
						flowPushRemote,
						logger,
						flowSource)
					isInit = false
				}
			}
		}
	} else {
		// Use the flow name directly.
		flowName = flow
		flowSource = flow
	}

	getFlowConfiguration = func(flowTemplatePath string) (map[string]interface{}, bool) {
		flowProject, flowService, flowConfigTemplatePath := eUtils.GetProjectService(flowTemplatePath)
		flowConfigTemplateName := eUtils.GetTemplateFileName(flowConfigTemplatePath, flowService)

		properties, err := NewProperties(vault, goMod, env, flowProject, flowService, logger)
		if err != nil {
			return nil, false
		}

		return properties.GetConfigValues(flowService, flowConfigTemplateName)
	}

	// Make a call on Call back to insert or update using the provided query.
	// If this is expected to result in a change to an existing table, thern trigger
	// something to the changed channel.
	applyDBQueryCB = func(query string, changed bool, operation string, flowNotifications []string) [][]string {
		if operation == "INSERT" {
			_, _, _, err := db.Query(tierceronEngine, query)
			if err != nil {
				eUtils.LogErrorObject(err, logger, false)
			}
			if changed {
				changedChannel <- true
				if flowNotifications != nil && len(flowNotifications) > 0 {
					// look up channels and notify them too.
					for _, flowNotification := range flowNotifications {
						if _, ok := channelMap[flowNotification]; ok {
							channelMap[flowNotification] <- true
						}
					}
				}
			}
		} else if operation == "UPDATE" {
			_, _, _, err := db.Query(tierceronEngine, query)
			if err != nil {
				eUtils.LogErrorObject(err, logger, false)
			}
			if changed {
				changedChannel <- true

				if flowNotifications != nil && len(flowNotifications) > 0 {
					// look up channels and notify them too.
					for _, flowNotification := range flowNotifications {
						channelMap[flowNotification] <- true
					}
				}
			}
		} else if operation == "SELECT" {
			_, _, matrixChangedEntries, err := db.Query(tierceronEngine, query)
			if err != nil {
				eUtils.LogErrorObject(err, logger, false)
			}
			return matrixChangedEntries
		}
		return nil
	}

	// Open a database connection to the provided source using provided
	// source configurations.
	getSourceDBConn := func(dbUrl string, username string, sourceDBConfig map[string]interface{}) (*sql.DB, error) {
		return OpenDirectConnection(dbUrl,
			username,
			configcore.DecryptSecretConfig(sourceDBConfig, sourceDatabaseConnectionMap), logger)
	}

	// Utilizing provided api auth headers, endpoint, and body data
	// this CB makes a call on behalf of the caller and returns a map
	// representation of json data provided by the endpoint.
	getSourceByAPICB := func(apiAuthHeaders map[string]string, host string, apiEndpoint string, bodyData io.Reader, getOrPost bool) (map[string]interface{}, error) {
		httpClient, err := helperkv.CreateHTTPClient(false, host, env, false)
		if err != nil {
			return nil, err
		}
		if getOrPost {
			return GetJSONFromClientByGet(httpClient, apiAuthHeaders, apiEndpoint, bodyData, logger)
		}
		return GetJSONFromClientByPost(httpClient, apiAuthHeaders, apiEndpoint, bodyData, logger)
	}

	// Create remote data source with only what is needed.
	remoteDataSource := map[string]interface{}{}
	remoteDataSource["dbsourceregion"] = sourceDatabaseConnectionMap["dbsourceregion"]
	remoteDataSource["dbingestinterval"] = sourceDatabaseConnectionMap["dbingestinterval"]

	dbsourceConn, err := OpenDirectConnection(sourceDatabaseConnectionMap["dbsourceurl"].(string), sourceDatabaseConnectionMap["dbsourceuser"].(string), sourceDatabaseConnectionMap["dbsourcepassword"].(string), logger)

	if err != nil {
		eUtils.LogErrorMessage("Couldn't get dedicated database connection.", logger, false)
		return err
	}
	defer dbsourceConn.Close()

	remoteDataSource["connection"] = dbsourceConn

	flowError := tcutil.ProcessFlowController(identityConfig,
		authData,
		getFlowConfiguration,
		remoteDataSource,
		getSourceByAPICB,
		flowSource,
		tierceronEngine.Database.Name(),
		flowName,
		getSourceDBConn,
		initTableSchemaCB,
		createTableTriggersCB,
		applyDBQueryCB,
		initSeedVaultListenerCB, func(msg string, err error) {
			if err != nil {
				eUtils.LogErrorObject(err, logger, false)
			} else {
				eUtils.LogInfo(msg, logger)
			}
		})
	if flowError != nil {
		eUtils.LogErrorObject(flowError, logger, true)
	}
	return nil
}

func ProcessFlows(pluginConfig map[string]interface{}, logger *log.Logger) error {
	// 1. Get Plugin configurations.
	goMod, vault, err := eUtils.InitVaultModForPlugin(pluginConfig, logger)
	if err != nil {
		eUtils.LogErrorMessage("Could not access vault.  Failure to start.", logger, false)
		return err
	}
	projects, services, _ := eUtils.GetProjectServices(pluginConfig["connectionPath"].([]string))
	var sourceDatabaseConfigs []map[string]interface{}
	var vaultDatabaseConfig map[string]interface{}
	var identityConfig map[string]interface{}

	for i := 0; i < len(projects); i++ {

		var indexValues []string

		if services[i] == "Database" {
			// TODO: This could be an api call vault list - to list what's available with rid's.
			// East and west...
			goMod.IndexName = "regionId"
			regions, err := goMod.ListIndexes(projects[i], goMod.IndexName)
			if err != nil {
				eUtils.LogErrorObject(err, logger, false)
				return err
			}
			indexValues = regions
		} else {
			indexValues = []string{""}
		}

		for _, indexValue := range indexValues {
			goMod.IndexValue = indexValue
			ok := false
			properties, err := NewProperties(vault, goMod, pluginConfig["env"].(string), projects[i], services[i], logger)
			if err != nil {
				eUtils.LogErrorObject(err, logger, false)
				return err
			}

			switch services[i] {
			case "Database":
				var sourceDatabaseConfig map[string]interface{}
				sourceDatabaseConfig, ok = properties.GetConfigValues(services[i], "config")
				if !ok {
					// Just ignore this one and go to the next one.
					eUtils.LogWarningMessage("Expected database configuration does not exist: "+indexValue, logger, false)
					continue
				}
				sourceDatabaseConfigs = append(sourceDatabaseConfigs, sourceDatabaseConfig)

			case "Identity":
				identityConfig, ok = properties.GetConfigValues(services[i], "config")
				if !ok {
					eUtils.LogErrorMessage("Couldn't get config values.", logger, false)
					return err
				}
			case "VaultDatabase":
				vaultDatabaseConfig, ok = properties.GetConfigValues(services[i], "config")
				if !ok {
					eUtils.LogErrorMessage("Couldn't get config values.", logger, false)
					return err
				}
			}
		}

	}
	sourceDatabaseConnectionsMap := map[string]map[string]interface{}{}

	// 4. Create config for vault for queries to vault.
	emptySlice := []string{""}

	configDriver := eUtils.DriverConfig{
		Regions:      emptySlice,
		Insecure:     true,
		Token:        pluginConfig["token"].(string),
		VaultAddress: pluginConfig["address"].(string),
		Env:          pluginConfig["env"].(string),
		Log:          logger,
	}

	tableList := pluginConfig["templatePath"].([]string)

	for _, table := range tableList {
		_, service, tableTemplateName := eUtils.GetProjectService(table)
		tableName := eUtils.GetTemplateFileName(tableTemplateName, service)
		configDriver.VersionFilter = append(configDriver.VersionFilter, tableName)
	}

	tierceronEngine, err := db.CreateEngine(&configDriver, tableList, pluginConfig["env"].(string), tcutil.GetDatabaseName())
	if err != nil {
		eUtils.LogErrorMessage("Couldn't build engine.", logger, false)
		return err
	}

	// 2. Establish mysql connection to remote mysql instance.
	for _, sourceDatabaseConfig := range sourceDatabaseConfigs {
		dbSourceConnBundle := map[string]interface{}{}
		dbSourceConnBundle["dbsourceurl"] = sourceDatabaseConfig["dbsourceurl"].(string)
		dbSourceConnBundle["dbsourceuser"] = sourceDatabaseConfig["dbsourceuser"].(string)
		dbSourceConnBundle["dbsourcepassword"] = sourceDatabaseConfig["dbsourcepassword"].(string)

		dbSourceConnBundle["encryptionSecret"] = sourceDatabaseConfig["dbencryptionSecret"].(string)
		if dbIngestInterval, ok := sourceDatabaseConfig["dbingestinterval"]; ok {
			ingestInterval, err := strconv.ParseInt(dbIngestInterval.(string), 10, 64)
			if err == nil {
				eUtils.LogInfo("Ingest interval: "+dbIngestInterval.(string), logger)
				dbSourceConnBundle["dbingestinterval"] = time.Duration(ingestInterval)
			}
		} else {
			eUtils.LogErrorMessage("Ingest interval invalid - Defaulting to 60 minutes.", logger, false)
			dbSourceConnBundle["dbingestinterval"] = time.Duration(60000)
		}

		sourceDatabaseConnectionsMap[sourceDatabaseConfig["dbsourceregion"].(string)] = dbSourceConnBundle
	}

	// Http query resources include:
	// 1. Auth -- Auth is provided by the external library tcutil.
	// 2. Get json by Api call.
	authComponents := tcutil.GetAuthComponents(identityConfig)
	httpClient, err := helperkv.CreateHTTPClient(false, authComponents["authDomain"].(string), pluginConfig["env"].(string), false)
	if err != nil {
		eUtils.LogErrorObject(err, logger, false)
		return err
	}

	authData, errPost := GetJSONFromClientByPost(httpClient, authComponents["authHeaders"].(map[string]string), authComponents["authUrl"].(string), authComponents["bodyData"].(io.Reader), logger)
	if errPost != nil {
		eUtils.LogErrorObject(errPost, logger, false)
		return errPost
	}

	// 2. Initialize Engine and create changes table.
	tierceronEngine.Context = sqle.NewEmptyContext()
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	var wg sync.WaitGroup

	for _, tableName := range configDriver.VersionFilter {
		changeTableName := tableName + "_Changes"

		if _, ok, _ := tierceronEngine.Database.GetTableInsensitive(tierceronEngine.Context, changeTableName); !ok {
			eUtils.LogInfo("Creating tierceron sql table: "+changeTableName, logger)
			m.Lock()
			err := tierceronEngine.Database.CreateTable(tierceronEngine.Context, changeTableName, sqle.NewPrimaryKeySchema(sqle.Schema{
				{Name: "id", Type: sqle.Text, Source: changeTableName},
				{Name: "updateTime", Type: sqle.Timestamp, Source: changeTableName},
			}))
			m.Unlock()
			if err != nil {
				eUtils.LogErrorObject(err, logger, false)
				return err
			}
		}
	}
	eUtils.LogInfo("Tables creation completed.", logger)

	channelMap = make(map[string]chan bool)
	for _, table := range tableList {
		channelMap[table] = make(chan bool, 5)
	}

	for _, flowName := range tcutil.GetAdditionalFlows() {
		channelMap[flowName] = make(chan bool, 5)
	}

	for _, sourceDatabaseConnectionMap := range sourceDatabaseConnectionsMap {
		for _, table := range tableList {
			wg.Add(1)
			go func(t string) {
				eUtils.LogInfo("Beginning flow: "+t, logger)
				defer wg.Done()
				flowMod, flowVault, err := eUtils.InitVaultModForPlugin(pluginConfig, logger)
				if err != nil {
					eUtils.LogErrorMessage("Could not access vault.  Failure to start flow.", logger, false)
					return
				}

				ProcessFlow(tierceronEngine,
					identityConfig,
					vaultDatabaseConfig,
					pluginConfig["address"].(string),
					flowMod,
					sourceDatabaseConnectionMap,
					flowVault,
					authData,
					pluginConfig["env"].(string),
					t,
					TableSyncFlow,
					channelMap[table], // tableChangedChannel
					signalChannel,
					logger,
				)
			}(table)
		}
		for _, flowName := range tcutil.GetAdditionalFlows() {
			wg.Add(1)
			go func(f string) {
				eUtils.LogInfo("Beginning flow: "+f, logger)
				defer wg.Done()
				flowMod, flowVault, err := eUtils.InitVaultModForPlugin(pluginConfig, logger)
				if err != nil {
					eUtils.LogErrorMessage("Could not access vault.  Failure to start flow.", logger, false)
					return
				}

				ProcessFlow(tierceronEngine,
					identityConfig,
					vaultDatabaseConfig,
					pluginConfig["address"].(string),
					flowMod,
					sourceDatabaseConnectionMap,
					flowVault,
					authData,
					pluginConfig["env"].(string),
					f,
					TableEnrichFlow,
					channelMap[flowName], // tableChangedChannel
					signalChannel,
					logger,
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

package util

import (
	"database/sql"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tierceron/trcx/db"
	extract "tierceron/trcx/extract"

	"tierceron/utils"
	helperkv "tierceron/vaulthelper/kv"
	"tierceron/vaulthelper/system"

	eUtils "tierceron/utils"

	tcutil "VaultConfig.TenantConfig/util"

	sys "tierceron/vaulthelper/system"

	configcore "VaultConfig.Bootstrap/configcore"
	sqle "github.com/dolthub/go-mysql-server/sql"
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
	return `CREATE TRIGGER tcInsertTrigger BEFORE UPDATE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + idColumnName + `, current_timestamp());` +
		` END;`
}

func seedVaultFromChanges(tierceronEngine *db.TierceronEngine,
	goMod *helperkv.Modifier,
	vaultAddress string,
	baseTableTemplate *extract.TemplateResultData,
	service string,
	v *system.Vault,
	databaseName string,
	tableName string,
	identityColumnName string,
	changeTable string,
	changedColumnName string,
	logger *log.Logger) error {
	changeIdQuery := getChangeIdQuery(databaseName, changeTable)
	_, _, matrixChangedEntries, err := db.Query(tierceronEngine, changeIdQuery)
	if err != nil {
		eUtils.LogErrorObject(err, logger, false)
	}

	for _, changedEntry := range matrixChangedEntries {
		changedId := changedEntry[0]

		changedTableQuery := `SELECT * FROM ` + databaseName + `.` + tableName + ` WHERE ` + identityColumnName + `='` + changedId + `'` // TODO: Implement query using changedId

		_, changedTableColumns, changedTableData, err := db.Query(tierceronEngine, changedTableQuery)
		if err != nil {
			eUtils.LogErrorObject(err, logger, false)
			return err
		}

		tableDataMap := map[string]interface{}{}
		for i, column := range changedTableColumns {
			tableDataMap[column] = changedTableData[0][i]
		}
		// Convert matrix/slice to tenantConfiguration map
		// Columns are keys, values in tenantData

		//Use trigger to make another table
		seedError := SeedVaultById(goMod, service, vaultAddress, v.GetToken(), baseTableTemplate, tableDataMap, tableDataMap[changedColumnName].(string), logger)
		if seedError != nil {
			eUtils.LogErrorObject(seedError, logger, false)
			return seedError
		}
	}
	return nil
}

func ProcessTable(tierceronEngine *db.TierceronEngine,
	identityConfig map[string]interface{},
	vaultDatabaseConfig map[string]interface{},
	vaultAddress string,
	goMod *helperkv.Modifier,
	sourceDatabaseConnectionMap map[string]interface{},
	vault *sys.Vault,
	authData map[string]interface{},
	env string,
	templateTablePath string,
	changedChannel chan bool,
	signalChannel chan os.Signal,
	logger *log.Logger) error {

	// 	i. Init engine
	//     a. Get project, service, and table config template name.
	project, service, tableTemplateName := utils.GetProjectService(templateTablePath)
	tableName := utils.GetTemplateFileName(tableTemplateName, service)
	changeTableName := tableName + "_Changes"

	err := tierceronEngine.Database.CreateTable(tierceronEngine.Context, changeTableName, sqle.NewPrimaryKeySchema(sqle.Schema{
		{Name: "id", Type: sqle.Text, Source: changeTableName},
		{Name: "updateTime", Type: sqle.Timestamp, Source: changeTableName},
	}))
	if err != nil {
		eUtils.LogErrorObject(err, logger, false)
		return err
	}

	// Set up schema callback for table to track.
	initTableSchemaCB := func(tableSchema sqle.PrimaryKeySchema, tableName string) {
		//	ii. Init database and tables in local mysql engine instance.
		err = tierceronEngine.Database.CreateTable(tierceronEngine.Context, tableName, tableSchema)
		if err != nil {
			eUtils.LogErrorObject(err, logger, false)
		}
	}

	// Set up call back to enable a trigger to track
	// whenever a row in a table changes...
	createTableTriggersCB := func(identityColumnName string) {
		//Create triggers
		var updTrigger sqle.TriggerDefinition
		var insTrigger sqle.TriggerDefinition
		insTrigger.Name = "tcInsertTrigger"
		updTrigger.Name = "tcUpdateTrigger"
		updTrigger.CreateStatement = getUpdateTrigger(tierceronEngine.Database.Name(), tableName, identityColumnName)
		insTrigger.CreateStatement = getInsertTrigger(tierceronEngine.Database.Name(), tableName, identityColumnName)
		tierceronEngine.Database.CreateTrigger(tierceronEngine.Context, updTrigger)
		tierceronEngine.Database.CreateTrigger(tierceronEngine.Context, insTrigger)
	}

	// Make a call on Call back to insert or update using the provided query.
	// If this is expected to result in a change to an existing table, thern trigger
	// something to the changed channel.
	applyDBQueryCB := func(query string, changed bool) {
		_, _, _, err := db.Query(tierceronEngine, query)
		if err != nil {
			eUtils.LogErrorObject(err, logger, false)
		}
		if changed {
			changedChannel <- true
		}
	}

	// Open a database connection to the provided source using provided
	// source configurations.
	getSourceDBConn := func(dbUrl string, username string, sourceDBConfig map[string]interface{}) (*sql.DB, error) {
		return OpenDirectConnection(dbUrl,
			username,
			configcore.DecryptSecretConfig(sourceDBConfig, sourceDatabaseConnectionMap))
	}

	getFlowConfiguration := func(flowTemplatePath string) (map[string]interface{}, bool) {
		flowProject, flowService, flowConfigTemplatePath := utils.GetProjectService(flowTemplatePath)
		flowConfigTemplateName := utils.GetTemplateFileName(flowConfigTemplatePath, flowService)

		properties, err := NewProperties(vault, goMod, env, flowProject, flowService, logger)
		if err != nil {
			return nil, false
		}

		return properties.GetConfigValues(flowService, flowConfigTemplateName)
	}

	// 3. Write seed data to vault
	var baseTableTemplate extract.TemplateResultData
	LoadBaseTemplate(&baseTableTemplate, goMod, project, service, templateTablePath, logger)

	// Generates a seed configuration utilizing the current tableName and provided id
	// This is equivalent to a 'row' in a table.
	seedVaultCB := func(tableData map[string]interface{}, id string) {
		err := SeedVaultById(goMod, service, vaultAddress, vault.GetToken(), &baseTableTemplate, tableData, id, logger)
		if err != nil {
			eUtils.LogErrorObject(err, logger, false)
		}
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
			return GetJSONFromClientByGet(httpClient, apiAuthHeaders, apiEndpoint, bodyData)
		}
		return GetJSONFromClientByPost(httpClient, apiAuthHeaders, apiEndpoint, bodyData)
	}

	// When called sets up an infinite loop listening for changes on either
	// the changedChannel or checks itself every 3 minutes for changes to
	// its own tables.
	seedVaultDeltaCB := func(idColumnName string, changedColumnName string) {
		for {
			select {
			case <-signalChannel:
				eUtils.LogErrorMessage("Receiving shutdown presumably from vault.", logger, true)
				os.Exit(0)
			case <-changedChannel:
				seedVaultFromChanges(tierceronEngine, goMod, vaultAddress, &baseTableTemplate, service, vault, tierceronEngine.Database.Name(), tableName, idColumnName, changeTableName, changedColumnName, logger)
			case <-time.After(time.Minute * 3):
				eUtils.LogInfo("3 minutes... checking for changes.", logger)
				seedVaultFromChanges(tierceronEngine, goMod, vaultAddress, &baseTableTemplate, service, vault, tierceronEngine.Database.Name(), tableName, idColumnName, changeTableName, changedColumnName, logger)
			}
		}
	}

	tcutil.ProcessTableController(identityConfig,
		authData,
		getFlowConfiguration,
		sourceDatabaseConnectionMap["connection"].(*sql.DB),
		getSourceByAPICB,
		project,
		tierceronEngine.Database.Name(),
		tableName,
		getSourceDBConn,
		initTableSchemaCB,
		createTableTriggersCB,
		applyDBQueryCB,
		seedVaultCB,
		seedVaultDeltaCB, func(msg string, err error) {
			if err != nil {
				eUtils.LogErrorObject(err, logger, false)
			} else {
				eUtils.LogInfo(msg, logger)
			}
		})
	return nil
}

func ProcessTables(pluginConfig map[string]interface{}, logger *log.Logger) error {
	// 1. Get Plugin configurations.
	projects, services, _ := utils.GetProjectServices(pluginConfig["connectionPath"].([]string))
	goMod, _ := helperkv.NewModifier(true, pluginConfig["token"].(string), pluginConfig["address"].(string), pluginConfig["env"].(string), []string{}, logger)
	goMod.Env = pluginConfig["env"].(string)
	goMod.Version = "0"
	vault, err := sys.NewVault(true, pluginConfig["address"].(string), goMod.Env, false, false, false, logger)
	if err != nil {
		eUtils.LogErrorObject(err, logger, false)
		return err
	}
	vault.SetToken(pluginConfig["token"].(string))
	var sourceDatabaseConfigs []map[string]interface{}
	var vaultDatabaseConfig map[string]interface{}
	var identityConfig map[string]interface{}

	for i := 0; i < len(projects); i++ {

		var idEnvironments []string

		if services[i] == "Database" {
			// This could be an api call list list what's available with rid's.
			// East and west...
			idEnvironments = []string{".rid.1", ".rid.2"}
		} else {
			idEnvironments = []string{""}
		}

		for _, idEnvironment := range idEnvironments {
			ok := false
			properties, err := NewProperties(vault, goMod, pluginConfig["env"].(string)+idEnvironment, projects[i], services[i], logger)
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
					eUtils.LogWarningMessage("Expected database configuration does not exist: "+idEnvironment, logger, false)
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

	// 2. Establish mysql connection to remote mysql instance.
	for _, sourceDatabaseConfig := range sourceDatabaseConfigs {
		dbsourceConn, err := OpenDirectConnection(sourceDatabaseConfig["dbsourceurl"].(string), sourceDatabaseConfig["dbsourceuser"].(string), sourceDatabaseConfig["dbsourcepassword"].(string))

		if err != nil {
			eUtils.LogErrorMessage("Couldn't get database connection.", logger, false)
			return err
		}

		if dbsourceConn != nil {
			defer dbsourceConn.Close()
		}
		dbSourceConnBundle := map[string]interface{}{}
		dbSourceConnBundle["connection"] = dbsourceConn
		dbSourceConnBundle["encryptionSecret"] = sourceDatabaseConfig["dbencryptionSecret"].(string)

		sourceDatabaseConnectionsMap[sourceDatabaseConfig["dbsourceregion"].(string)] = dbSourceConnBundle
	}

	// 4. Create config for vault for queries to vault.
	emptySlice := []string{""}
	configDriver := utils.DriverConfig{
		Regions:      emptySlice,
		Insecure:     true,
		Token:        pluginConfig["token"].(string),
		VaultAddress: pluginConfig["address"].(string),
		Env:          pluginConfig["env"].(string),
	}

	tableList := pluginConfig["templatePath"].([]string)

	// Http query resources include:
	// 1. Auth -- Auth is provided by the external library tcutil.
	// 2. Get json by Api call.
	authComponents := tcutil.GetAuthComponents(identityConfig)
	httpClient, err := helperkv.CreateHTTPClient(false, authComponents["authDomain"].(string), pluginConfig["env"].(string), false)
	if err != nil {
		eUtils.LogErrorObject(err, logger, false)
		return err
	}

	authData, errPost := GetJSONFromClientByPost(httpClient, authComponents["authHeaders"].(map[string]string), authComponents["authUrl"].(string), authComponents["bodyData"].(io.Reader))
	if errPost != nil {
		eUtils.LogErrorObject(errPost, logger, false)
		return errPost
	}

	tierceronEngine, err := db.CreateEngine(&configDriver, tableList, pluginConfig["env"].(string), tcutil.GetDatabaseName(), logger)
	if err != nil {
		eUtils.LogErrorMessage("Couldn't build engine.", logger, false)
		return err
	}

	// 2. Initialize Engine and create changes table.
	tierceronEngine.Context = sqle.NewEmptyContext()
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	for _, sourceDatabaseConnectionMap := range sourceDatabaseConnectionsMap {
		for _, table := range tableList {
			ProcessTable(tierceronEngine,
				identityConfig,
				vaultDatabaseConfig,
				pluginConfig["address"].(string),
				goMod,
				sourceDatabaseConnectionMap,
				vault,
				authData,
				pluginConfig["env"].(string),
				table,
				make(chan bool, 5), // tableChangedChannel
				signalChannel,
				logger,
			)
		}
	}
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

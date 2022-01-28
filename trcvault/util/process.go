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

	"strings"

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

func ProcessTable(tierceronEngine *db.TierceronEngine, config map[string]interface{},
	vaultAddress string,
	goMod *helperkv.Modifier,
	mysqlConn *sql.DB,
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
	templateSplit := strings.Split(tableTemplateName, service+"/")
	tableName := strings.Split(templateSplit[len(templateSplit)-1], ".")[0]
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
	initTableSchemaCB := func(tableSchema sqle.PrimaryKeySchema) {
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
			configcore.DecryptSecretConfig(sourceDBConfig, config))
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

	// TODO: cms Construction Management Services - presently our only source for enriching our internal tables.
	// Is this truly the only other source?
	httpClient, err := helperkv.CreateHTTPClient(false, config["cmsDomain"].(string), env, false)

	// Utilizing provided api auth headers, endpoint, and body data
	// this CB makes a call on behalf of the caller and returns a map
	// representation of json data provided by the endpoint.
	getSourceByAPICB := func(apiAuthHeaders map[string]string, apiEndpoint string, bodyData io.Reader) map[string]interface{} {
		return GetJSONFromClient(httpClient, apiAuthHeaders, apiEndpoint, bodyData)
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
				seedVaultFromChanges(tierceronEngine, goMod, vaultAddress, &baseTableTemplate, service, vault, tierceronEngine.Database.Name(), tableName, idColumnName, changeTableName, changedColumnName, logger)
			}
		}
	}

	tcutil.ProcessTableController(config,
		authData,
		mysqlConn,
		getSourceByAPICB,
		project,
		tierceronEngine.Database.Name(),
		tableName,
		getSourceDBConn,
		initTableSchemaCB,
		createTableTriggersCB,
		applyDBQueryCB,
		seedVaultCB,
		seedVaultDeltaCB)
	return nil
}

func ProcessTables(pluginConfig map[string]interface{}, logger *log.Logger) error {
	// 1. Get Plugin configurations.
	project, service, _ := utils.GetProjectService(pluginConfig["connectionPath"].(string))
	goMod, _ := helperkv.NewModifier(true, pluginConfig["token"].(string), pluginConfig["address"].(string), pluginConfig["env"].(string), []string{}, logger)
	goMod.Env = pluginConfig["env"].(string)
	goMod.Version = "0"
	vault, err := sys.NewVault(true, pluginConfig["address"].(string), goMod.Env, false, false, false, logger)
	if err != nil {
		eUtils.LogErrorObject(err, logger, false)
		return err
	}
	vault.SetToken(pluginConfig["token"].(string))
	properties, err := NewProperties(vault, goMod, pluginConfig["env"].(string), project, service, logger)
	if err != nil {
		eUtils.LogErrorObject(err, logger, false)
		return err
	}

	config, ok := properties.GetConfigValues(service, "config")
	if !ok {
		eUtils.LogErrorMessage("Couldn't get config values.", logger, false)
		return err
	}

	// 2. Establish mysql connection to remote mysql instance.
	mysqlConn, err := OpenDirectConnection(config["mysqldburl"].(string), config["mysqldbuser"].(string), config["mysqldbpassword"].(string))
	if mysqlConn != nil {
		defer mysqlConn.Close()
	}

	if err != nil {
		eUtils.LogErrorMessage("Couldn't get sql connection.", logger, false)
		return err
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
	authComponents := tcutil.GetAuthComponents(config)
	httpClient, err := helperkv.CreateHTTPClient(false, authComponents["authDomain"].(string), pluginConfig["env"].(string), false)
	if err != nil {
		eUtils.LogErrorObject(err, logger, false)
		return err
	}

	authData := GetJSONFromClient(httpClient, authComponents["authHeaders"].(map[string]string), authComponents["authUrl"].(string), authComponents["bodyData"].(io.Reader))

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

	for _, table := range tableList {
		ProcessTable(tierceronEngine,
			config,
			pluginConfig["address"].(string),
			goMod,
			mysqlConn,
			vault,
			authData,
			pluginConfig["env"].(string),
			table,
			make(chan bool, 5), // tableChangedChannel
			signalChannel,
			logger,
		)
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

package util

import (
	"database/sql"
	"fmt"
	"io"
	"time"

	"tierceron/trcx/db"
	extract "tierceron/trcx/extract"

	"tierceron/utils"
	helperkv "tierceron/vaulthelper/kv"
	"tierceron/vaulthelper/system"

	tcutil "VaultConfig.TenantConfig/util"

	"log"
	"strings"

	sys "tierceron/vaulthelper/system"

	configcore "VaultConfig.Bootstrap/configcore"
	sqle "github.com/dolthub/go-mysql-server/sql"
)

var changedChannel = make(chan bool, 5)

func getChangeIdQuery(databaseName string, changeTable string) string {
	return "SELECT id FROM " + databaseName + `.` + changeTable
}

func getDeleteChangeQuery(databaseName string, changeTable string, id string) string {
	return "DELETE FROM " + databaseName + `.` + changeTable + " WHERE id = '" + id + "'"
}

func GetUpdateTrigger(databaseName string, tableName string, idColumnName string) string {
	return `CREATE TRIGGER tcUpdateTrigger BEFORE UPDATE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` UPDATE ` + databaseName + `.` + tableName + `_Changes SET id=new.tenantId, updateTime=current_timestamp() WHERE EXISTS (select id from ` + databaseName + `.` + tableName + `_Changes where id=new.` + idColumnName + `);` +
		` END;`
}

func GetInsertTrigger(databaseName string, tableName string, idColumnName string) string {
	return `CREATE TRIGGER tcInsertTrigger BEFORE UPDATE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + idColumnName + `, current_timestamp());` +
		` END;`
}

func SeedVaultFromChanges(tierceronEngine *db.TierceronEngine, goMod *helperkv.Modifier, pluginConfig map[string]interface{}, baseTableTemplate *extract.TemplateResultData, service string, v *system.Vault, databaseName string, tableName string, idColumnName string, changeTable string) {
	changeIdQuery := getChangeIdQuery(databaseName, changeTable)
	_, _, matrixChangedEntries, err := db.Query(tierceronEngine, changeIdQuery)
	if err != nil {
		log.Println(err)
	}

	for _, changedEntry := range matrixChangedEntries {
		changedId := changedEntry[0]

		changedTableQuery := `SELECT * FROM ` + databaseName + `.` + tableName + ` WHERE ` + idColumnName + `='` + changedId + `'` // TODO: Implement query using changedId

		_, changedTableColumns, changedTableData, err := db.Query(tierceronEngine, changedTableQuery)
		if err != nil {
			log.Println(err)
		}

		tableDataMap := map[string]string{}
		for i, column := range changedTableColumns {
			tableDataMap[column] = changedTableData[0][i]
		}
		// Convert matrix/slice to tenantConfiguration map
		// Columns are keys, values in tenantData

		//Use trigger to make another table
		seedError := SeedVaultById(goMod, service, pluginConfig["address"].(string), v.GetToken(), baseTableTemplate, tableDataMap, tableDataMap["enterpriseId"])
		if seedError != nil {
			log.Println(seedError)
		}
	}
}

func ProcessTable(pluginConfig map[string]interface{},
	configDriver *utils.DriverConfig,
	config map[string]interface{},
	goMod *helperkv.Modifier,
	mysqlConn *sql.DB,
	vault *sys.Vault,
	env string, templateTablePath string) {

	// 5. Upload tenants into a mysql table
	// 	i. Init engine
	//     a. Get project, service, and table config template name.
	project, service, tableTemplateName := utils.GetProjectService(pluginConfig["templatePath"].(string))
	templateSplit := strings.Split(tableTemplateName, service+"/")
	tableName := strings.Split(templateSplit[len(templateSplit)-1], ".")[0]
	templatePaths := []string{pluginConfig["templatePath"].(string)}
	tierceronEngine, err := db.CreateEngine(configDriver, templatePaths, env, service)
	if err != nil {
		log.Println(err)
	}

	tierceronEngine.Context = sqle.NewEmptyContext()
	//	ii. Init database and tables in local mysql engine instance.
	err = tierceronEngine.Database.CreateTable(tierceronEngine.Context, tableName, tcutil.GetTenantSchema(project))
	if err != nil {
		log.Println(err)
	}

	// 3. Retrieve tenant configurations from mysql.
	tenantConfigurations, err := tcutil.GetTenantConfigurations(mysqlConn)

	changeTableName := tableName + "_Changes"
	err = tierceronEngine.Database.CreateTable(tierceronEngine.Context, changeTableName, sqle.NewPrimaryKeySchema(sqle.Schema{
		{Name: "id", Type: sqle.Text, Source: changeTableName},
		{Name: "updateTime", Type: sqle.Timestamp, Source: changeTableName},
	}))
	if err != nil {
		log.Println(err)
	}

	//Create triggers
	var updTrigger sqle.TriggerDefinition
	var insTrigger sqle.TriggerDefinition
	insTrigger.Name = "tcInsertTrigger"
	updTrigger.Name = "tcUpdateTrigger"
	updTrigger.CreateStatement = GetUpdateTrigger(tierceronEngine.Database.Name(), tableName, tcutil.GetTenantIdColumnName())
	insTrigger.CreateStatement = GetInsertTrigger(tierceronEngine.Database.Name(), tableName, tcutil.GetTenantIdColumnName())
	tierceronEngine.Database.CreateTrigger(tierceronEngine.Context, updTrigger)
	tierceronEngine.Database.CreateTrigger(tierceronEngine.Context, insTrigger)

	for _, tenant := range tenantConfigurations { //Loop through tenant configs and add to mysql table
		tenant["enterpriseId"] = ""
		_, _, _, err := db.Query(tierceronEngine, tcutil.GetTenantConfigurationInsert(tenant, tierceronEngine.Database.Name(), tableName))
		if err != nil {
			log.Println(err)
		}
	}
	/*
		// e. Query for enterprise vs no-enterprise id in mysql table
			//sql query
			sqlstr := "SELECT * FROM " + tierceronEngine.Database.Name() + "." + project + " WHERE enterpriseId = ''"
			tierceronEngine.Context = tierceronEngine.Context.WithCurrentDB(tierceronEngine.Database.Name())
			_, _, _, err = vaultvutil.Query(tierceronEngine, sqlstr)
			if err != nil {
				log.Println(err)
			}
	*/

	//easier way to query?
	var enterpriseTenants []map[string]string
	var nonEnterpriseTenants []map[string]string
	for _, tenant := range tenantConfigurations {
		if tenant["enterpriseId"] != "" {
			enterpriseTenants = append(enterpriseTenants, tenant)
		} else {
			nonEnterpriseTenants = append(nonEnterpriseTenants, tenant)
		}
	}

	// 2. Pull enterprises from vault --> local queryable manageable mysql db.
	/* //UNCOMMENT THIS LATER***
		listValues, err := goMod.ListEnv("values/")
		if err != nil { //This call only works if vault has permission to list metadata at values/
			log.Println(err) //otherwise permission denied.
		} else if listValues == nil {
			log.Println("No environments were found when querying vault.")
		} else {
			for _, valuesPath := range listValues.Data {
				for _, envInterface := range valuesPath.([]interface{}) {
					if strings.Contains(envInterface.(string), goMod.Env) && strings.Contains(envInterface.(string), ".") {
						eidStr := strings.Split(envInterface.(string), ".")[1]
						eidStr = strings.ReplaceAll(eidStr, "/", "")
						eid, err := svaultonv.Atoi(eidStr)
						if err != nil {
							fmt.Printf("Failed to convert eid to an integer: %s \n", eidStr)
						}
						availEids = append(availEids, eid)
					}
				}
			}
		}
	}
	*/

	// 3. Write seed data to vault
	var baseTableTemplate extract.TemplateResultData
	LoadBaseTemplate(&baseTableTemplate, goMod, project, service, pluginConfig["templatePath"].(string))

	//Puts tenant configurations inside generated seed template.
	for _, tableData := range enterpriseTenants {
		err := SeedVaultById(goMod, service, pluginConfig["address"].(string), vault.GetToken(), &baseTableTemplate, tableData, tableData["enterpriseId"])
		if err != nil {
			log.Println(err)
		}
	}

	//
	// 1. ETL from mysql -> vault?  Either in memory or mysql->file->Vault
	//     Templates have file directory format: Project/Service/config
	//     Database will have:  Database = Service, table = config
	//     Further factoring can put Project->mysql instance by port...
	//     We want another configuration file..  that would have port numbers by id?
	//         this config would have a name(Project) and a port (mysql port)
	//     Multiples would be queryably -- so, each instance of the config would get its own id (like an enterprise)
	//
	// 2. Pull enterprises from vault --> local queryable manageable mysql db.  *done*  *milestone*  Just check in the method that returns slice of enterprises.
	// 3. Write seed data to vault... if it has an enterprise...		*done*
	//     a. if it doesn't have an enterprise id... then write it directly to mysql database... but do this after
	//        happy path is done.
	//        -- Connect to spectrum db (using data in each enterprise)
	//        -- Query table PA_VALUE_VARIABLES for salesforceId
	//        -- If there is a salesforceId -- query over to Team with sfid.. and get list of enterprises registered.
	//           -- take returned enterprise id and dump it into this row..
	//           -- if no enterpriseid returned...  they are not yet registered!
	//              if not yet registered with team...
	//              goto AutoRegistration...

	authComponents := tcutil.GetAuthComponents(config)
	httpClient, err := helperkv.CreateHTTPClient(false, authComponents["authDomain"].(string), env, false)
	if err != nil {
		log.Println(err)
	}

	authData := GetJSONFromClient(httpClient, authComponents["authHeaders"].(map[string]string), authComponents["authUrl"].(string), authComponents["bodyData"].(io.Reader))

	for _, tenantConfiguration := range nonEnterpriseTenants {
		if tenantConfiguration["tenantId"] == "qa14p8" {
			spectrumConn, err := OpenDirectConnection(tenantConfiguration["jdbcUrl"],
				tenantConfiguration["username"],
				configcore.DecryptSecretConfig(tenantConfiguration, config))

			if spectrumConn != nil {
				defer spectrumConn.Close()
			}

			if err != nil {
				log.Println(err)
				continue
			}

			var registrationReferenceId string
			err = spectrumConn.QueryRow(tcutil.GetRegistrationReferenceIdQuery()).Scan(&registrationReferenceId)
			if err != nil {
				log.Println(err)
				continue
			} else if registrationReferenceId == "" {
				log.Println("No eid found.")
				continue
			} else {
				authData["refId"] = strings.TrimSpace(registrationReferenceId)
			}
			sourceIdComponents := tcutil.GetSourceIdComponents(config, authData)

			clientData := GetJSONFromClient(httpClient, sourceIdComponents["apiAuthHeaders"].(map[string]string), sourceIdComponents["apiEndpoint"].(string), sourceIdComponents["bodyData"].(io.Reader))
			// End Refactor
			if len(clientData["items"].([]interface{})) > 0 {
				enterpriseMap := clientData["items"].([]interface{})[0].(map[string]interface{})
				if enterpriseMap["id"].(float64) != 0 {
					tenantConfiguration["enterpriseId"] = fmt.Sprintf("%.0f", enterpriseMap["id"].(float64))

					//SQL update row
					_, _, _, err := db.Query(tierceronEngine, tcutil.GetTenantConfigurationUpdate(tenantConfiguration, tierceronEngine.Database.Name(), tableName))
					if err != nil {
						log.Println(err)
					}

					changedChannel <- true
				}
			}
		}
	}
	//Write back to SQL engine
	//Upload the tenant to vault with new id ->
	//start up mysql instance locally -> can leave this commented out
	// point db.dex at vault.dex : sql port

	// Work with enterprise data stuff... to register enterprises...

	// 4. Write a go routine that periodically runs 3a...
	//    This is basically a 'watcher' routine that periodically updates Vault if the internal
	//    mysql table changes in any way...
	//    -- for now this can be a no-op (does nothing)...  with a sleep...
	func() {
		for {
			select {
			case <-changedChannel:
				SeedVaultFromChanges(tierceronEngine, goMod, pluginConfig, &baseTableTemplate, service, vault, tierceronEngine.Database.Name(), tableName, tcutil.GetTenantIdColumnName(), changeTableName)
			case <-time.After(time.Minute * 3):
				SeedVaultFromChanges(tierceronEngine, goMod, pluginConfig, &baseTableTemplate, service, vault, tierceronEngine.Database.Name(), tableName, tcutil.GetTenantIdColumnName(), changeTableName)
			}
		}
	}()
}

func DoProcessEnvConfig(env string, pluginConfig map[string]interface{}) error {
	// 1. Get Plugin configurations.
	project, service, _ := utils.GetProjectService(pluginConfig["connectionPath"].(string))
	goMod, _ := helperkv.NewModifier(true, pluginConfig["token"].(string), pluginConfig["address"].(string), env, []string{})
	goMod.Env = env
	goMod.Version = "0"
	vault, err := sys.NewVault(true, pluginConfig["address"].(string), goMod.Env, false, false, false)
	if err != nil {
		log.Println(err)
	}
	vault.SetToken(pluginConfig["token"].(string))
	properties, err := NewProperties(vault, goMod, env, project, service)
	if err != nil {
		log.Println(err)
	}

	config, ok := properties.GetConfigValues(service, "config")
	if !ok {
		log.Println("Couldn't get config values.")
	}

	// 2. Establish mysql connection to remote mysql instance.
	mysqlConn, err := OpenDirectConnection(config["mysqldburl"].(string), config["mysqldbuser"].(string), config["mysqldbpassword"].(string))
	if mysqlConn != nil {
		defer mysqlConn.Close()
	}

	if err != nil {
		return err
	}

	if err != nil {
		return err
	}

	// 4. Create config for vault for queries to vault.
	emptySlice := []string{""}
	configDriver := utils.DriverConfig{
		Regions:      emptySlice,
		Insecure:     true,
		Token:        pluginConfig["token"].(string),
		VaultAddress: pluginConfig["address"].(string),
		Env:          env,
	}

	ProcessTable(pluginConfig,
		&configDriver,
		config,
		goMod,
		mysqlConn,
		vault,
		env,
		pluginConfig["templatePath"].(string))
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
	time.Sleep(15 * time.Second)
	return nil
}

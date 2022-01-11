package util

import (
	//	"sync"

	"time"

	vcutils "tierceron/trcconfig/utils"
	il "tierceron/trcinit/initlib"
	"tierceron/trcx/db"
	extract "tierceron/trcx/extract"
	"tierceron/trcx/xutil"

	//	"tierceron/trcx/xutil"
	"tierceron/utils"
	eUtils "tierceron/utils"
	helperkv "tierceron/vaulthelper/kv"

	tcutil "VaultConfig.TenantConfig/util"
	"gopkg.in/yaml.v2"

	"fmt"
	"log"
	"strings"

	sys "tierceron/vaulthelper/system"

	configcore "VaultConfig.Bootstrap/configcore"
	"github.com/dolthub/go-mysql-server/sql"
	//	"gopkg.in/yaml.v2"
)

func DoProcessEnvConfig(env string, pluginConfig map[string]interface{}) error {
	// TODO: kick off singleton of enterprise registration...
	// If all went well, everything we need should be in:
	//     environmentConfigs
	// 1. ETL from mysql -> vault?  Either in memory or mysql->file->Vault
	project, service, templateFile := utils.GetProjectService(pluginConfig["connectionPath"].(string))
	goMod, _ := helperkv.NewModifier(true, pluginConfig["token"].(string), pluginConfig["address"].(string), env, []string{})
	goMod.Env = env
	goMod.Version = "0"
	v, err := sys.NewVault(true, pluginConfig["address"].(string), goMod.Env, false, false, false)
	if err != nil {
		log.Println(err)
	}
	v.SetToken(pluginConfig["token"].(string))
	properties, err := NewProperties(v, goMod, env, project, service)
	if err != nil {
		log.Println(err)
	}

	config, ok := properties.GetConfigValues(service, "config")
	if !ok {
		log.Println("Couldn't get config values.")
	}

	// a. Establish mysql connection
	mysqlConn, err := OpenDirectConnection(config["mysqldburl"].(string), config["mysqldbuser"].(string), config["mysqldbpassword"].(string))
	if mysqlConn != nil {
		defer mysqlConn.Close()
	}

	if err != nil {
		return err
	}
	// b. Retrieve tenant configurations
	tenantConfigurations, err := tcutil.GetTenantConfigurations(mysqlConn)

	if err != nil {
		return err
	}

	// c. Create config for engine for queries
	emptySlice := []string{""}
	configDriver := eUtils.DriverConfig{
		Regions:      emptySlice,
		Insecure:     true,
		Token:        pluginConfig["token"].(string),
		VaultAddress: pluginConfig["address"].(string),
		Env:          env,
	}

	// d. Upload tenants into a mysql table
	// 	i. Init engine
	project, service, templateFile = utils.GetProjectService(pluginConfig["templatePath"].(string))
	templateSplit := strings.Split(templateFile, service+"/")
	templateFile = strings.Split(templateSplit[len(templateSplit)-1], ".")[0]
	templatePaths := []string{pluginConfig["templatePath"].(string)}
	tierceronEngine, err := db.CreateEngine(configDriver, templatePaths, env, service)
	if err != nil {
		log.Println(err)
	}

	tierceronEngine.Context = sql.NewEmptyContext()
	//	ii. Init database and table in engine
	err = tierceronEngine.Database.CreateTable(tierceronEngine.Context, templateFile, tcutil.GetTenantSchema(project))
	if err != nil {
		log.Println(err)
	}

	for _, tenant := range tenantConfigurations { //Loop through tenant configs and add to mysql table
		_, _, _, err := db.Query(tierceronEngine, tcutil.GetTenantConfigurationInsert(tenant, tierceronEngine.Database.Name(), templateFile))
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
	//VaultX Section Begins
	var templateResult extract.TemplateResultData

	templateResult.ValueSection = map[string]map[string]map[string]string{}
	templateResult.ValueSection["values"] = map[string]map[string]string{}

	templateResult.SecretSection = map[string]map[string]map[string]string{}
	templateResult.SecretSection["super-secrets"] = map[string]map[string]string{}

	var cds *vcutils.ConfigDataStore
	commonPaths := make([]string, 1, 1)
	if goMod != nil {
		cds = new(vcutils.ConfigDataStore)
		goMod.Version = goMod.Version + "***X-Mode"
		cds.Init(goMod, true, true, project, commonPaths, service) //CommonPaths = "" - empty - not needed for tenant config
	}

	_, _, _, templateResult.TemplateDepth = extract.ToSeed(goMod,
		cds,
		pluginConfig["templatePath"].(string),
		&log.Logger{},
		project,
		service,
		true,
		&(templateResult.InterfaceTemplateSection),
		&(templateResult.ValueSection),
		&(templateResult.SecretSection),
	)

	//var wg sync.WaitGroup
	//Puts tenant configurations inside generated seed template.
	for _, tenantConfiguration := range enterpriseTenants {

		func(templateResult extract.TemplateResultData) {
			//defer wg.Done()
			//wg.Add(1)

			valueCombinedSection := map[string]map[string]map[string]string{}
			valueCombinedSection["values"] = map[string]map[string]string{}

			secretCombinedSection := map[string]map[string]map[string]string{}
			secretCombinedSection["super-secrets"] = map[string]map[string]string{}

			// Declare local variables
			templateCombinedSection := map[string]interface{}{}
			sliceTemplateSection := []interface{}{}
			sliceValueSection := []map[string]map[string]map[string]string{}
			sliceSecretSection := []map[string]map[string]map[string]string{}
			for key, value := range tenantConfiguration {
				templateResult.SecretSection["super-secrets"][service][key] = value
			}
			maxDepth := templateResult.TemplateDepth
			// Combine values of slice

			sliceTemplateSection = append(sliceTemplateSection, templateResult.InterfaceTemplateSection)
			sliceValueSection = append(sliceValueSection, templateResult.ValueSection)
			sliceSecretSection = append(sliceSecretSection, templateResult.SecretSection)

			xutil.CombineSection(sliceTemplateSection, maxDepth, templateCombinedSection)
			xutil.CombineSection(sliceValueSection, -1, valueCombinedSection)
			xutil.CombineSection(sliceSecretSection, -1, secretCombinedSection)

			template, errT := yaml.Marshal(templateCombinedSection)
			value, errV := yaml.Marshal(valueCombinedSection)
			secret, errS := yaml.Marshal(secretCombinedSection)

			if errT != nil {
				fmt.Println(errT)
			}

			if errV != nil {
				fmt.Println(errV)
			}

			if errS != nil {
				fmt.Println(errS)
			}
			templateData := string(template)
			// Remove single quotes generated by Marshal
			templateData = strings.ReplaceAll(templateData, "'", "")
			seedData := templateData + "\n\n\n" + string(value) + "\n\n\n" + string(secret) + "\n\n\n"
			//VaultX Section Ends
			//VaultInit Section Begins
			if err != nil {
				log.Println(err)
			}

			il.SeedVaultFromData(true, []byte(seedData), pluginConfig["address"].(string), v.GetToken(), goMod.Env, log.Default(), service, false, "dev."+tenantConfiguration["enterpriseId"])
		}(templateResult)
	}
	//wg.Wait()

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

	for _, tenantConfiguration := range nonEnterpriseTenants {
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

		sqlstr := "SELECT *"
		rows, err := spectrumConn.Query(sqlstr)
		if err != nil {
			log.Println(err)
		}
		fmt.Println(rows)
	}
	// Work with enterprise data stuff... to register enterprises...

	// 4. Write a go routine that periodically runs 3a...
	//    -- for now this can be a no-op (does nothing)...  with a sleep...
	go func() {
		time.Sleep(time.Second * 3)
	}()
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
	//        In order of priority: TenantConfiguration, SpectrumEnterpriseConfig, KafkaTableConfiguration, Mysqlfile, ReportJobs, Tokens?
	//     II. Open up mysql port and performance test queries...
	//         -- create a mysql client runner... I bet there are go libraries that let you do this...
	//     I don't wanna do this...
	//     d. Optionally update fieldtech TenantConfiguration back to mysql.
	//
	return nil
}

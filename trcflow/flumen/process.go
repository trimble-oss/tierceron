package flumen

import (
	"io"
	"log"
	"strconv"
	"sync"
	"time"

	"tierceron/buildopts"
	"tierceron/buildopts/flowopts"
	"tierceron/buildopts/harbingeropts"
	"tierceron/buildopts/testopts"
	trcvutils "tierceron/trcvault/util"
	trcdb "tierceron/trcx/db"

	flowcore "tierceron/trcflow/core"
	"tierceron/trcflow/deploy"
	helperkv "tierceron/vaulthelper/kv"

	eUtils "tierceron/utils"

	sys "tierceron/vaulthelper/system"

	sqle "github.com/dolthub/go-mysql-server/sql"
)

func ProcessFlows(pluginConfig map[string]interface{}, logger *log.Logger) error {
	logger.Println("ProcessFlows begun.")
	// 1. Get Plugin configurations.
	var tfmContext *flowcore.TrcFlowMachineContext
	var config *eUtils.DriverConfig
	var vault *sys.Vault
	var goMod *helperkv.Modifier
	var err error

	config, goMod, vault, err = eUtils.InitVaultModForPlugin(pluginConfig, logger)
	if err != nil {
		eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start.", false)
		return err
	}

	//Need new function writing to that path using pluginName ->
	//if not copied -> this plugin should fail to start up
	//Update deployed status & return if
	if pluginNameList, ok := pluginConfig["pluginNameList"].([]string); ok {
		deployedUpdateErr := deploy.PluginDeployedUpdate(goMod, pluginNameList, logger)
		if deployedUpdateErr != nil {
			eUtils.LogErrorMessage(config, deployedUpdateErr.Error(), false)
			eUtils.LogErrorMessage(config, "Could not update plugin deployed status in vault.", false)
			return err
		}
	}
	logger.Println("Deployed status updated.")

	tfmContext = &flowcore.TrcFlowMachineContext{
		Env:                       pluginConfig["env"].(string),
		GetAdditionalFlowsByState: flowopts.GetAdditionalFlowsByState,
	}
	projects, services, _ := eUtils.GetProjectServices(pluginConfig["connectionPath"].([]string))
	var sourceDatabaseConfigs []map[string]interface{}
	var vaultDatabaseConfig map[string]interface{}
	var trcIdentityConfig map[string]interface{}
	logger.Println("Grabbing configs.")
	for i := 0; i < len(projects); i++ {

		var indexValues []string

		if services[i] == "Database" {
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

		if services[i] == "VaultDatabase" || services[i] == "Identity" {
			goMod.SectionName = "config"
			goMod.SectionKey = "/Restricted/"
		}

		for _, indexValue := range indexValues {
			goMod.SubSectionValue = indexValue
			ok := false
			properties, err := trcvutils.NewProperties(config, vault, goMod, pluginConfig["env"].(string), projects[i], services[i])
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
				for _, supportedRegion := range buildopts.GetSupportedSourceRegions() {
					if sourceDatabaseConfig["dbsourceregion"] == supportedRegion {
						sourceDatabaseConfigs = append(sourceDatabaseConfigs, sourceDatabaseConfig)
					}
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
	eUtils.LogInfo(config, "Finished retrieving configs")
	sourceDatabaseConnectionsMap := map[string]map[string]interface{}{}

	// 4. Create config for vault for queries to vault.
	emptySlice := []string{""}

	configBasis := eUtils.DriverConfig{
		Regions:      emptySlice,
		Token:        pluginConfig["token"].(string),
		VaultAddress: pluginConfig["vaddress"].(string),
		Insecure:     true, // TODO: investigate insecure implementation...
		Env:          pluginConfig["env"].(string),
		Log:          logger,
	}

	templateList := pluginConfig["templatePath"].([]string)
	flowTemplateMap := map[string]string{}
	flowSourceMap := map[string]string{}
	flowControllerMap := map[string]chan int64{}

	for _, template := range templateList {
		source, service, tableTemplateName := eUtils.GetProjectService(template)
		tableName := eUtils.GetTemplateFileName(tableTemplateName, service)
		if tableName != tierceronFlowConfigurationTableName {
			configBasis.VersionFilter = append(configBasis.VersionFilter, tableName)
		}
		flowTemplateMap[tableName] = template
		flowSourceMap[tableName] = source
		flowControllerMap[tableName] = make(chan int64)
	}

	tfmContext.TierceronEngine, err = trcdb.CreateEngine(&configBasis, templateList, pluginConfig["env"].(string), harbingeropts.GetDatabaseName())
	tfmContext.Config = &configBasis

	if err != nil {
		eUtils.LogErrorMessage(config, "Couldn't build engine.", false)
		return err
	}
	eUtils.LogInfo(config, "Finished building engine")

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
	//time.Sleep(8 * time.Second)

	// Http query resources include:
	// 1. Auth -- Auth is provided by the external library.
	// 2. Get json by Api call.
	extensionAuthComponents := buildopts.GetExtensionAuthComponents(trcIdentityConfig)
	httpClient, err := helperkv.CreateHTTPClient(false, extensionAuthComponents["authDomain"].(string), pluginConfig["env"].(string), false)
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return err
	}

	tfmContext.ExtensionAuthData, err = trcvutils.GetJSONFromClientByPost(config, httpClient, extensionAuthComponents["authHeaders"].(map[string]string), extensionAuthComponents["authUrl"].(string), extensionAuthComponents["bodyData"].(io.Reader))
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return err
	}

	// 2. Initialize Engine and create changes table.
	tfmContext.TierceronEngine.Context = sqle.NewEmptyContext()
	tfmContext.Init(sourceDatabaseConnectionsMap, configBasis.VersionFilter, flowopts.GetAdditionalFlows(), flowopts.GetAdditionalFlows())

	//Initialize tfcContext for flow controller
	tfmFlumContext := &flowcore.TrcFlowMachineContext{
		Env:                       pluginConfig["env"].(string),
		GetAdditionalFlowsByState: flowopts.GetAdditionalFlowsByState,
	}

	tfmFlumContext.TierceronEngine, err = trcdb.CreateEngine(&configBasis, templateList, pluginConfig["env"].(string), flowopts.GetFlowDatabaseName())
	tfmFlumContext.TierceronEngine.Context = sqle.NewEmptyContext()
	tfmFlumContext.Init(sourceDatabaseConnectionsMap, []string{tierceronFlowConfigurationTableName}, flowopts.GetAdditionalFlows(), flowopts.GetAdditionalFlows())
	tfmFlumContext.Config = &configBasis
	tfmFlumContext.ExtensionAuthData = tfmContext.ExtensionAuthData

	var wg sync.WaitGroup
	for _, sourceDatabaseConnectionMap := range sourceDatabaseConnectionsMap {
		for _, table := range GetTierceronTableNames() {
			tfContext := flowcore.TrcFlowContext{RemoteDataSource: make(map[string]interface{})}
			tfContext.RemoteDataSource["flowControllerMap"] = flowControllerMap
			tfContext.RemoteDataSource["vaultImportChannel"] = make(chan bool)
			wg.Add(1)
			go func(tableFlow flowcore.FlowNameType, tcfContext flowcore.TrcFlowContext) {
				eUtils.LogInfo(config, "Beginning flow: "+tableFlow.ServiceName())
				defer wg.Done()
				tcfContext.Flow = tableFlow
				tcfContext.FlowSource = flowSourceMap[tableFlow.TableName()]
				tcfContext.FlowPath = flowTemplateMap[tableFlow.TableName()]

				config, tcfContext.GoMod, tcfContext.Vault, err = eUtils.InitVaultModForPlugin(pluginConfig, logger)
				if err != nil {
					eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start flow.", false)
					return
				}
				tcfContext.FlowSourceAlias = flowopts.GetFlowDatabaseName()

				tfmFlumContext.ProcessFlow(
					config,
					&tcfContext,
					FlumenProcessFlowController,
					vaultDatabaseConfig,
					sourceDatabaseConnectionMap,
					tableFlow,
					flowcore.TableSyncFlow,
				)
			}(flowcore.FlowNameType(table), tfContext)
			<-tfContext.RemoteDataSource["vaultImportChannel"].(chan bool)
		}
	}

	for _, sourceDatabaseConnectionMap := range sourceDatabaseConnectionsMap {
		for _, table := range configBasis.VersionFilter {
			wg.Add(1)
			go func(tableFlow flowcore.FlowNameType) {
				eUtils.LogInfo(config, "Beginning flow: "+tableFlow.ServiceName())
				defer wg.Done()
				tfContext := flowcore.TrcFlowContext{RemoteDataSource: map[string]interface{}{}}
				tfContext.RemoteDataSource["flowStateChannel"] = flowControllerMap[tableFlow.TableName()]
				tfContext.Flow = tableFlow
				tfContext.FlowSource = flowSourceMap[tableFlow.TableName()]
				tfContext.FlowPath = flowTemplateMap[tableFlow.TableName()]

				config, tfContext.GoMod, tfContext.Vault, err = eUtils.InitVaultModForPlugin(pluginConfig, logger)
				if err != nil {
					eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start flow.", false)
					return
				}
				tfContext.FlowSourceAlias = harbingeropts.GetDatabaseName()

				tfmContext.ProcessFlow(
					config,
					&tfContext,
					flowopts.ProcessFlowController,
					vaultDatabaseConfig,
					sourceDatabaseConnectionMap,
					tableFlow,
					flowcore.TableSyncFlow,
				)
			}(flowcore.FlowNameType(table))
		}
		for _, enhancement := range flowopts.GetAdditionalFlows() {
			wg.Add(1)
			go func(enhancementFlow flowcore.FlowNameType) {
				eUtils.LogInfo(config, "Beginning flow: "+enhancementFlow.ServiceName())
				defer wg.Done()
				tfContext := flowcore.TrcFlowContext{RemoteDataSource: map[string]interface{}{}}
				tfContext.Flow = enhancementFlow

				config, tfContext.GoMod, tfContext.Vault, err = eUtils.InitVaultModForPlugin(pluginConfig, logger)
				if err != nil {
					eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start flow.", false)
					return
				}

				tfmContext.ProcessFlow(
					config,
					&tfContext,
					flowopts.ProcessFlowController,
					vaultDatabaseConfig,
					sourceDatabaseConnectionMap,
					enhancementFlow,
					flowcore.TableEnrichFlow,
				)
			}(enhancement)
		}

		for _, test := range testopts.GetAdditionalTestFlows() {
			wg.Add(1)
			go func(testFlow flowcore.FlowNameType) {
				eUtils.LogInfo(config, "Beginning flow: "+testFlow.ServiceName())
				defer wg.Done()
				tfContext := flowcore.TrcFlowContext{RemoteDataSource: map[string]interface{}{}}
				tfContext.Flow = testFlow
				config, tfContext.GoMod, tfContext.Vault, err = eUtils.InitVaultModForPlugin(pluginConfig, logger)
				if err != nil {
					eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start flow.", false)
					return
				}

				tfmContext.ProcessFlow(
					config,
					&tfContext,
					flowopts.ProcessTestFlowController,
					vaultDatabaseConfig,
					sourceDatabaseConnectionMap,
					testFlow,
					flowcore.TableTestFlow,
				)
			}(test)
		}
	}

	// TODO: Start up dolt mysql instance listening on a port so we can use the plugin instead to host vault encrypted data.
	// Variables such as username, password, port are in vaultDatabaseConfig -- configs coming from encrypted vault.
	// The engine is in tfmContext...  that's the one we need to make available for connecting via dbvis...
	// be sure to enable encryption on the connection...
	wg.Add(1)
	vaultDatabaseConfig["vaddress"] = pluginConfig["vaddress"]
	interfaceErr := harbingeropts.BuildInterface(config, goMod, tfmContext, vaultDatabaseConfig, &TrcDBServerEventListener{})
	if interfaceErr != nil {
		wg.Done()
		eUtils.LogErrorMessage(config, "Failed to start up database interface:"+interfaceErr.Error(), false)
		return interfaceErr
	}

	wg.Add(1)
	//add 10 to the port number for flowDatabase
	portNumber, err := strconv.Atoi(vaultDatabaseConfig["dbport"].(string))
	if err != nil {
		eUtils.LogErrorMessage(config, "Failed to parse port number for Flow Database:"+interfaceErr.Error(), false)
		return interfaceErr
	}
	portNumber = portNumber + 10
	vaultDatabaseConfig["dbport"] = strconv.Itoa(portNumber)

	interfaceErr = harbingeropts.BuildInterface(config, goMod, tfmFlumContext, vaultDatabaseConfig, &TrcDBServerEventListener{})
	if interfaceErr != nil {
		wg.Done()
		eUtils.LogErrorMessage(config, "Failed to start up database interface:"+interfaceErr.Error(), false)
		return interfaceErr
	}
	wg.Wait()
	logger.Println("ProcessFlows complete.")

	return nil
}

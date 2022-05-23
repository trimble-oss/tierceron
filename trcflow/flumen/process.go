package flumen

import (
	"io"
	"log"
	"strconv"
	"sync"
	"time"

	"tierceron/trcvault/util"
	"tierceron/trcx/db"

	flowcore "tierceron/trcflow/core"
	helperkv "tierceron/vaulthelper/kv"

	testflowimpl "VaultConfig.Test/util"

	eUtils "tierceron/utils"

	sys "tierceron/vaulthelper/system"

	flowimpl "VaultConfig.TenantConfig/util"
	sqle "github.com/dolthub/go-mysql-server/sql"
)

func ProcessFlows(pluginConfig map[string]interface{}, logger *log.Logger) error {
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
		deployedUpdateErr := PluginDeployedUpdate(goMod, pluginNameList)
		if deployedUpdateErr != nil {
			eUtils.LogErrorMessage(config, deployedUpdateErr.Error(), false)
			eUtils.LogErrorMessage(config, "Could not update plugin deployed status in vault.", false)
			return err
		}
	}

	tfmContext = &flowcore.TrcFlowMachineContext{
		Env:                       pluginConfig["env"].(string),
		GetAdditionalFlowsByState: testflowimpl.GetAdditionalFlowsByState,
	}
	projects, services, _ := eUtils.GetProjectServices(pluginConfig["connectionPath"].([]string))
	var sourceDatabaseConfigs []map[string]interface{}
	var vaultDatabaseConfig map[string]interface{}
	var trcIdentityConfig map[string]interface{}

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
				for _, supportedRegion := range flowimpl.GetSupportedSourceRegions() {
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

	templateList := pluginConfig["templatePath"].([]string)
	flowTemplateMap := map[string]string{}
	flowSourceMap := map[string]string{}

	for _, template := range templateList {
		source, service, tableTemplateName := eUtils.GetProjectService(template)
		tableName := eUtils.GetTemplateFileName(tableTemplateName, service)
		configBasis.VersionFilter = append(configBasis.VersionFilter, tableName)
		flowTemplateMap[tableName] = template
		flowSourceMap[tableName] = source
	}

	tfmContext.TierceronEngine, err = db.CreateEngine(&configBasis, templateList, pluginConfig["env"].(string), flowimpl.GetDatabaseName())
	tfmContext.Config = &configBasis

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

	tfmContext.ExtensionAuthData, err = util.GetJSONFromClientByPost(config, httpClient, extensionAuthComponents["authHeaders"].(map[string]string), extensionAuthComponents["authUrl"].(string), extensionAuthComponents["bodyData"].(io.Reader))
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return err
	}

	// 2. Initialize Engine and create changes table.
	tfmContext.TierceronEngine.Context = sqle.NewEmptyContext()

	var wg sync.WaitGroup
	tfmContext.Init(sourceDatabaseConnectionsMap, configBasis.VersionFilter, flowimpl.GetAdditionalFlows(), testflowimpl.GetAdditionalFlows())

	for _, sourceDatabaseConnectionMap := range sourceDatabaseConnectionsMap {
		for _, table := range configBasis.VersionFilter {
			wg.Add(1)
			go func(tableFlow flowcore.FlowNameType) {
				eUtils.LogInfo(config, "Beginning flow: "+tableFlow.ServiceName())
				defer wg.Done()
				tfContext := flowcore.TrcFlowContext{RemoteDataSource: map[string]interface{}{}}
				tfContext.Flow = tableFlow
				tfContext.FlowSource = flowSourceMap[tableFlow.TableName()]
				tfContext.FlowPath = flowTemplateMap[tableFlow.TableName()]

				config, tfContext.GoMod, tfContext.Vault, err = eUtils.InitVaultModForPlugin(pluginConfig, logger)
				if err != nil {
					eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start flow.", false)
					return
				}
				tfContext.FlowSourceAlias = flowimpl.GetDatabaseName()

				tfmContext.ProcessFlow(
					config,
					&tfContext,
					flowimpl.ProcessFlowController,
					vaultDatabaseConfig,
					sourceDatabaseConnectionMap,
					tableFlow,
					flowcore.TableSyncFlow,
				)
			}(flowcore.FlowNameType(table))
		}
		for _, enhancement := range flowimpl.GetAdditionalFlows() {
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
					flowimpl.ProcessFlowController,
					vaultDatabaseConfig,
					sourceDatabaseConnectionMap,
					enhancementFlow,
					flowcore.TableEnrichFlow,
				)
			}(enhancement)
		}

		for _, test := range testflowimpl.GetAdditionalFlows() {
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
					testflowimpl.ProcessTestFlowController,
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

	wg.Wait()
	return nil
}

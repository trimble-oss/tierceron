package flumen

import (
	"errors"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"tierceron/buildopts"
	"tierceron/buildopts/flowopts"
	"tierceron/buildopts/harbingeropts"
	"tierceron/buildopts/testopts"
	trcvutils "tierceron/trcvault/util"
	trcdb "tierceron/trcx/db"

	flowcore "tierceron/trcflow/core"
	flowcorehelper "tierceron/trcflow/core/flowcorehelper"
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
		InitConfigWG:              &sync.WaitGroup{},
		FlowMap:                   map[flowcore.FlowNameType]*flowcore.TrcFlowContext{},
	}
	projects, services, _ := eUtils.GetProjectServices(pluginConfig["connectionPath"].([]string))
	var sourceDatabaseConfigs []map[string]interface{}
	var vaultDatabaseConfig map[string]interface{}
	var spiralDatabaseConfig map[string]interface{}
	var trcIdentityConfig map[string]interface{}
	logger.Println("Grabbing configs.")
	for i := 0; i < len(projects); i++ {

		var indexValues []string

		if services[i] == "Database" {
			goMod.SectionName = "regionId"
			goMod.SectionKey = "/Index/"
			regions, err := goMod.ListSubsection("/Index/", projects[i], goMod.SectionName, logger)
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
		} else if services[i] == "SpiralDatabase" {
			goMod.SectionName = "config"
			goMod.SectionKey = "/Protected/"
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
			case "SpiralDatabase":
				spiralDatabaseConfig, ok = properties.GetConfigValues(services[i], "config")
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
		Log:          config.Log,
	}

	templateList := pluginConfig["templatePath"].([]string)
	flowTemplateMap := map[string]string{}
	flowSourceMap := map[string]string{}
	flowStateControllerMap := map[string]chan flowcorehelper.CurrentFlowState{}
	flowStateReceiverMap := map[string]chan flowcorehelper.FlowStateUpdate{}

	for _, template := range templateList {
		source, service, tableTemplateName := eUtils.GetProjectService(template)
		tableName := eUtils.GetTemplateFileName(tableTemplateName, service)
		if tableName != flowcorehelper.TierceronFlowConfigurationTableName {
			configBasis.VersionFilter = append(configBasis.VersionFilter, tableName)
		}
		flowTemplateMap[tableName] = template
		flowSourceMap[tableName] = source
		flowStateControllerMap[tableName] = make(chan flowcorehelper.CurrentFlowState, 1)
		flowStateReceiverMap[tableName] = make(chan flowcorehelper.FlowStateUpdate, 1)
	}

	for _, enhancement := range flowopts.GetAdditionalFlows() {
		flowStateControllerMap[enhancement.TableName()] = make(chan flowcorehelper.CurrentFlowState, 1)
		flowStateReceiverMap[enhancement.TableName()] = make(chan flowcorehelper.FlowStateUpdate, 1)
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
		tfmContext.ExtensionAuthDataReloader = make(map[string]interface{}, 1)
		tfmContext.ExtensionAuthDataReloader["config"] = config
		tfmContext.ExtensionAuthDataReloader["identityConfig"] = trcIdentityConfig
		//return err
	}

	// 2. Initialize Engine and create changes table.
	tfmContext.TierceronEngine.Context = sqle.NewEmptyContext()
	tfmContext.Init(sourceDatabaseConnectionsMap, configBasis.VersionFilter, flowopts.GetAdditionalFlows(), flowopts.GetAdditionalFlows())

	//Initialize tfcContext for flow controller
	tfmFlumeContext := &flowcore.TrcFlowMachineContext{
		InitConfigWG:              &sync.WaitGroup{},
		Env:                       pluginConfig["env"].(string),
		GetAdditionalFlowsByState: flowopts.GetAdditionalFlowsByState,
		FlowControllerInit:        true,
		FlowControllerUpdateLock:  sync.Mutex{},
		FlowControllerUpdateAlert: make(chan string, 1),
	}

	tfmFlumeContext.TierceronEngine, err = trcdb.CreateEngine(&configBasis, templateList, pluginConfig["env"].(string), flowopts.GetFlowDatabaseName())
	tfmFlumeContext.TierceronEngine.Context = sqle.NewEmptyContext()
	tfmFlumeContext.Init(sourceDatabaseConnectionsMap, []string{flowcorehelper.TierceronFlowConfigurationTableName}, flowopts.GetAdditionalFlows(), flowopts.GetAdditionalFlows())
	tfmFlumeContext.Config = &configBasis
	tfmFlumeContext.ExtensionAuthData = tfmContext.ExtensionAuthData
	var flowWG sync.WaitGroup
	for _, sourceDatabaseConnectionMap := range sourceDatabaseConnectionsMap {
		for _, table := range GetTierceronTableNames() {
			tfContext := flowcore.TrcFlowContext{RemoteDataSource: make(map[string]interface{}), ReadOnly: false}
			tfContext.RemoteDataSource["flowStateControllerMap"] = flowStateControllerMap
			tfContext.RemoteDataSource["flowStateReceiverMap"] = flowStateReceiverMap
			tfContext.RemoteDataSource["flowStateInitAlert"] = make(chan bool, 1)
			var controllerInitWG sync.WaitGroup
			tfContext.RemoteDataSource["controllerInitWG"] = &controllerInitWG
			controllerInitWG.Add(1)
			tfmFlumeContext.InitConfigWG.Add(1)
			flowWG.Add(1)
			go func(tableFlow flowcore.FlowNameType, tcfContext *flowcore.TrcFlowContext, dc *eUtils.DriverConfig) {
				eUtils.LogInfo(dc, "Beginning flow: "+tableFlow.ServiceName())
				defer flowWG.Done()
				tcfContext.Flow = tableFlow
				tcfContext.FlowSource = flowSourceMap[tableFlow.TableName()]
				tcfContext.FlowPath = flowTemplateMap[tableFlow.TableName()]
				tfmContext.FlowMap[tcfContext.Flow] = tcfContext
				var initErr error
				dc, tcfContext.GoMod, tcfContext.Vault, initErr = eUtils.InitVaultMod(dc)
				if initErr != nil {
					eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start flow.", false)
					return
				}
				tcfContext.FlowSourceAlias = flowopts.GetFlowDatabaseName()

				tfmFlumeContext.ProcessFlow(
					dc,
					tcfContext,
					FlumenProcessFlowController,
					vaultDatabaseConfig,
					sourceDatabaseConnectionMap,
					tableFlow,
					flowcore.TableSyncFlow,
				)
			}(flowcore.FlowNameType(table), &tfContext, config)

			controllerInitWG.Wait() //Waiting for remoteDataSource to load up to prevent data race.
			if initReciever, ok := tfContext.RemoteDataSource["flowStateInitAlert"].(chan bool); ok {
			initAlert: //This waits for flow states to be loaded before starting all non-controller flows
				for {
					select {
					case _, ok := <-initReciever:
						if ok {
							break initAlert
						}
					default:
						time.Sleep(time.Duration(time.Second))
					}
				}
			} else {
				initRecieverErr := errors.New("Failed to retrieve channel alert for controller init")
				eUtils.LogErrorMessage(config, initRecieverErr.Error(), false)
				return initRecieverErr
			}
		}

	}

	flowMapLock := &sync.Mutex{}
	for _, sourceDatabaseConnectionMap := range sourceDatabaseConnectionsMap {
		for _, table := range configBasis.VersionFilter {
			flowWG.Add(1)
			tfmContext.InitConfigWG.Add(1)
			go func(tableFlow flowcore.FlowNameType, dc *eUtils.DriverConfig) {
				eUtils.LogInfo(dc, "Beginning data source flow: "+tableFlow.ServiceName())
				defer flowWG.Done()
				tfContext := flowcore.TrcFlowContext{RemoteDataSource: map[string]interface{}{}, FlowLock: &sync.Mutex{}, ReadOnly: false}
				tfContext.RemoteDataSource["flowStateController"] = flowStateControllerMap[tableFlow.TableName()]
				tfContext.RemoteDataSource["flowStateReceiver"] = flowStateReceiverMap[tableFlow.TableName()]
				tfContext.Flow = tableFlow
				tfContext.FlowSource = flowSourceMap[tableFlow.TableName()]
				tfContext.FlowPath = flowTemplateMap[tableFlow.TableName()]
				flowMapLock.Lock()
				tfmContext.FlowMap[tfContext.Flow] = &tfContext
				flowMapLock.Unlock()
				var initErr error
				dc, tfContext.GoMod, tfContext.Vault, initErr = eUtils.InitVaultMod(dc)
				if initErr != nil {
					eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start flow.", false)
					return
				}
				tfContext.FlowSourceAlias = harbingeropts.GetDatabaseName()

				tfmContext.ProcessFlow(
					dc,
					&tfContext,
					flowopts.ProcessFlowController,
					vaultDatabaseConfig,
					sourceDatabaseConnectionMap,
					tableFlow,
					flowcore.TableSyncFlow,
				)
			}(flowcore.FlowNameType(table), config)
		}
		for _, enhancement := range flowopts.GetAdditionalFlows() {
			flowWG.Add(1)
			tfmContext.InitConfigWG.Add(1)

			go func(enhancementFlow flowcore.FlowNameType, dc *eUtils.DriverConfig) {
				eUtils.LogInfo(dc, "Beginning additional flow: "+enhancementFlow.ServiceName())
				defer flowWG.Done()
				tfmContext.InitConfigWG.Done()

				tfContext := flowcore.TrcFlowContext{RemoteDataSource: map[string]interface{}{}, FlowLock: &sync.Mutex{}, ReadOnly: false}
				tfContext.Flow = enhancementFlow
				tfContext.RemoteDataSource["flowStateController"] = flowStateControllerMap[enhancementFlow.TableName()]
				tfContext.RemoteDataSource["flowStateReceiver"] = flowStateReceiverMap[enhancementFlow.TableName()]
				var initErr error
				dc, tfContext.GoMod, tfContext.Vault, initErr = eUtils.InitVaultMod(dc)
				if initErr != nil {
					eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start flow.", false)
					return
				}

				tfmContext.ProcessFlow(
					dc,
					&tfContext,
					flowopts.ProcessFlowController,
					vaultDatabaseConfig,
					sourceDatabaseConnectionMap,
					enhancementFlow,
					flowcore.TableEnrichFlow,
				)
			}(enhancement, config)
		}

		for _, test := range testopts.GetAdditionalTestFlows() {
			flowWG.Add(1)
			tfmContext.InitConfigWG.Add(1)
			go func(testFlow flowcore.FlowNameType, dc *eUtils.DriverConfig, tfmc *flowcore.TrcFlowMachineContext) {
				eUtils.LogInfo(dc, "Beginning test flow: "+testFlow.ServiceName())
				defer flowWG.Done()
				tfmContext.InitConfigWG.Done()
				tfContext := flowcore.TrcFlowContext{RemoteDataSource: map[string]interface{}{}, FlowLock: &sync.Mutex{}, ReadOnly: false}
				tfContext.Flow = testFlow
				var initErr error
				dc, tfContext.GoMod, tfContext.Vault, initErr = eUtils.InitVaultMod(dc)
				if initErr != nil {
					eUtils.LogErrorMessage(dc, "Could not access vault.  Failure to start flow.", false)
					return
				}

				tfmc.ProcessFlow(
					dc,
					&tfContext,
					flowopts.ProcessTestFlowController,
					vaultDatabaseConfig,
					sourceDatabaseConnectionMap,
					testFlow,
					flowcore.TableTestFlow,
				)
			}(test, config, tfmContext)
		}
	}
	tfmFlumeContext.InitConfigWG.Wait()
	tfmFlumeContext.FlowControllerUpdateLock.Lock()
	tfmFlumeContext.InitConfigWG = nil
	tfmFlumeContext.FlowControllerUpdateLock.Unlock()

	vaultDatabaseConfig["vaddress"] = pluginConfig["vaddress"]
	//Set up controller config
	controllerVaultDatabaseConfig := make(map[string]interface{})
	for index, config := range vaultDatabaseConfig {
		controllerVaultDatabaseConfig[index] = config
	}

	controllerCheck := 0
	if cdbport, ok := vaultDatabaseConfig["controllerdbport"]; ok {
		controllerVaultDatabaseConfig["dbport"] = cdbport
		controllerCheck++
	}
	if cdbpass, ok := vaultDatabaseConfig["controllerdbpassword"]; ok {
		controllerVaultDatabaseConfig["dbpassword"] = cdbpass
		controllerCheck++
	}
	if cdbuser, ok := vaultDatabaseConfig["controllerdbuser"]; ok {
		controllerVaultDatabaseConfig["dbuser"] = cdbuser
		controllerCheck++
	}

	if controllerCheck == 3 {
		controllerVaultDatabaseConfig["vaddress"] = strings.Split(controllerVaultDatabaseConfig["vaddress"].(string), ":")[0]
		controllerInterfaceErr := harbingeropts.BuildInterface(config, goMod, tfmFlumeContext, controllerVaultDatabaseConfig, &TrcDBServerEventListener{Log: config.Log})
		if controllerInterfaceErr != nil {
			eUtils.LogErrorMessage(config, "Failed to start up controller database interface:"+controllerInterfaceErr.Error(), false)
			return controllerInterfaceErr
		}
	}

	vaultDatabaseConfig["vaddress"] = pluginConfig["vaddress"]
	//Set up controller config
	controllerVaultDatabaseConfig = make(map[string]interface{})
	for index, config := range vaultDatabaseConfig {
		controllerVaultDatabaseConfig[index] = config
	}

	vaultDatabaseConfig["vaddress"] = pluginConfig["vaddress"]
	//Set up controller config
	controllerVaultDatabaseConfig = make(map[string]interface{})
	for index, config := range vaultDatabaseConfig {
		controllerVaultDatabaseConfig[index] = config
	}

	// Wait for all tables to be built before starting interface.
	tfmContext.InitConfigWG.Wait()

	// TODO: Start up dolt mysql instance listening on a port so we can use the plugin instead to host vault encrypted data.
	// Variables such as username, password, port are in vaultDatabaseConfig -- configs coming from encrypted vault.
	// The engine is in tfmContext...  that's the one we need to make available for connecting via dbvis...
	// be sure to enable encryption on the connection...

	//Setting up DFS USER
	if dfsUser, ok := spiralDatabaseConfig["dbuser"]; ok {
		vaultDatabaseConfig["dfsUser"] = dfsUser
	}
	if dfsPass, ok := spiralDatabaseConfig["dbpassword"]; ok {
		vaultDatabaseConfig["dfsPass"] = dfsPass
	}

	interfaceErr := harbingeropts.BuildInterface(config, goMod, tfmContext, vaultDatabaseConfig, &TrcDBServerEventListener{Log: config.Log})
	if interfaceErr != nil {
		eUtils.LogErrorMessage(config, "Failed to start up database interface:"+interfaceErr.Error(), false)
		return interfaceErr
	}

	flowWG.Wait()

	logger.Println("ProcessFlows complete.")
	return nil
}

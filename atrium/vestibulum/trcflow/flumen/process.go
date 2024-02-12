package flumen

import (
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/trimble-oss/tierceron/atrium/buildopts/flowopts"
	"github.com/trimble-oss/tierceron/atrium/buildopts/testopts"
	trcdb "github.com/trimble-oss/tierceron/atrium/trcdb"
	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/harbingeropts"
	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"

	flowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"
	flowcorehelper "github.com/trimble-oss/tierceron/atrium/trcflow/core/flowcorehelper"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcflow/deploy"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"

	eUtils "github.com/trimble-oss/tierceron/pkg/utils"

	sys "github.com/trimble-oss/tierceron/pkg/vaulthelper/system"

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
	if pluginNameList, ok := pluginConfig["pluginNameList"].([]string); ok || true {
		tempAddr := pluginConfig["vaddress"]
		tempToken := pluginConfig["token"]
		if caddress, cOk := pluginConfig["caddress"]; cOk {
			pluginConfig["vaddress"] = caddress
		}
		if cToken, cOk := pluginConfig["ctoken"]; cOk {
			pluginConfig["token"] = cToken
		}
		pluginConfig["exitOnFailure"] = true

		cConfig, cGoMod, cVault, err := eUtils.InitVaultModForPlugin(pluginConfig, logger)
		if err != nil {
			eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start.", false)
			return err
		}
		deployedUpdateErr := deploy.PluginDeployedUpdate(cConfig, cGoMod, cVault, pluginNameList, pluginConfig["certifyPath"].([]string), logger)
		if deployedUpdateErr != nil {
			eUtils.LogErrorMessage(config, deployedUpdateErr.Error(), false)
			eUtils.LogErrorMessage(config, "Could not update plugin deployed status in vault.", false)
			return err
		}
		pluginConfig["vaddress"] = tempAddr
		pluginConfig["token"] = tempToken
		pluginConfig["exitOnFailure"] = false
	}
	logger.Println("Deployed status updated.")

	tfmContext = &flowcore.TrcFlowMachineContext{
		Env:                       pluginConfig["env"].(string),
		GetAdditionalFlowsByState: flowopts.BuildOptions.GetAdditionalFlowsByState,
		FlowMap:                   map[flowcore.FlowNameType]*flowcore.TrcFlowContext{},
	}
	projects, services, _ := eUtils.GetProjectServices(pluginConfig["connectionPath"].([]string))
	var sourceDatabaseConfigs []map[string]interface{}
	var vaultDatabaseConfig map[string]interface{}
	var spiralDatabaseConfig map[string]interface{}
	var trcIdentityConfig map[string]interface{}
	logger.Println("Grabbing configs.")
	for i := 0; i < len(projects); i++ {
		eUtils.LogInfo(config, fmt.Sprintf("Loading service: %s", services[i]))

		var regionValues []string

		if services[i] == "Database" {
			goMod.SectionName = "regionId"
			goMod.SectionKey = "/Index/"
			regions, err := goMod.ListSubsection("/Index/", projects[i], goMod.SectionName, logger)
			if err != nil {
				eUtils.LogErrorObject(config, err, false)
				eUtils.LogInfo(config, "Skipping service: "+services[i])
				continue
			}
			regionValues = regions
		} else {
			regionValues = []string{""}
		}

		if services[i] == "VaultDatabase" || services[i] == "Identity" {
			goMod.SectionName = "config"
			goMod.SectionKey = "/Restricted/"
		} else if services[i] == "SpiralDatabase" {
			goMod.SectionName = "config"
			goMod.SectionKey = "/Protected/"
		}

		for _, regionValue := range regionValues {
			eUtils.LogInfo(config, fmt.Sprintf("Processing region: %s", regionValue))
			goMod.SubSectionValue = regionValue
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
					eUtils.LogWarningMessage(config, "Expected database configuration does not exist: "+regionValue, false)
					continue
				}
				for _, supportedRegion := range buildopts.BuildOptions.GetSupportedSourceRegions() {
					if sourceDatabaseConfig["dbsourceregion"] == supportedRegion {
						eUtils.LogInfo(config, fmt.Sprintf("Loading service: %s for region: %s", services[i], regionValue))
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
		Insecure:     true, // Always local...
		Env:          pluginConfig["env"].(string),
		Log:          config.Log,
	}

	// Need to create askflumeflow template --> fill with default vals
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

	for _, enhancement := range flowopts.BuildOptions.GetAdditionalFlows() {
		flowStateControllerMap[enhancement.TableName()] = make(chan flowcorehelper.CurrentFlowState, 1)
		flowStateReceiverMap[enhancement.TableName()] = make(chan flowcorehelper.FlowStateUpdate, 1)
	}

	tfmContext.TierceronEngine, err = trcdb.CreateEngine(&configBasis, templateList, pluginConfig["env"].(string), harbingeropts.BuildOptions.GetDatabaseName())
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

	eUtils.LogInfo(config, "Finished building source configs")
	// Http query resources include:
	// 1. Auth -- Auth is provided by the external library.
	// 2. Get json by Api call.
	extensionAuthComponents := buildopts.BuildOptions.GetExtensionAuthComponents(trcIdentityConfig)
	if len(extensionAuthComponents) > 0 {
		httpClient, err := helperkv.CreateHTTPClient(false, extensionAuthComponents["authDomain"].(string), pluginConfig["env"].(string), false)
		if httpClient != nil {
			defer httpClient.CloseIdleConnections()
		}
		if err != nil {
			eUtils.LogErrorObject(config, err, false)
			return err
		}

		eUtils.LogInfo(config, "Finished creating auth extension connection")

		tfmContext.ExtensionAuthData, _, err = trcvutils.GetJSONFromClientByPost(config, httpClient, extensionAuthComponents["authHeaders"].(map[string]string), extensionAuthComponents["authUrl"].(string), extensionAuthComponents["bodyData"].(io.Reader))
		if err != nil {
			eUtils.LogErrorObject(config, err, false)
			//return err
		}
		// Set up reloader in case things go sideways later on.
		tfmContext.ExtensionAuthDataReloader = make(map[string]interface{}, 1)
		tfmContext.ExtensionAuthDataReloader["config"] = config
		tfmContext.ExtensionAuthDataReloader["identityConfig"] = trcIdentityConfig
	}

	eUtils.LogInfo(config, "Finished building source extension configs")

	// 2. Initialize Engine and create changes table.
	tfmContext.TierceronEngine.Context = sqle.NewEmptyContext()
	tfmContext.Init(sourceDatabaseConnectionsMap, configBasis.VersionFilter, flowopts.BuildOptions.GetAdditionalFlows(), flowopts.BuildOptions.GetAdditionalFlows())

	//Initialize tfcContext for flow controller
	tfmFlumeContext := &flowcore.TrcFlowMachineContext{
		InitConfigWG:              &sync.WaitGroup{},
		Env:                       pluginConfig["env"].(string),
		GetAdditionalFlowsByState: flowopts.BuildOptions.GetAdditionalFlowsByState,
		FlowControllerInit:        true,
		FlowControllerUpdateLock:  sync.Mutex{},
		FlowControllerUpdateAlert: make(chan string, 1),
	}

	if len(sourceDatabaseConnectionsMap) == 0 {
		sourceDatabaseConnectionsMap = make(map[string]map[string]interface{}, 1)
		sourceDatabaseDetails := make(map[string]interface{}, 1)
		sourceDatabaseDetails["dbsourceregion"] = "NA"
		var d time.Duration = 60000
		sourceDatabaseDetails["dbingestinterval"] = d
		sourceDatabaseConnectionsMap["NA"] = sourceDatabaseDetails
		sourceDatabaseDetails["sqlConn"] = nil
	}

	eUtils.LogInfo(config, "Finished building engine & changes tables")

	tfmFlumeContext.TierceronEngine, err = trcdb.CreateEngine(&configBasis, templateList, pluginConfig["env"].(string), flowopts.BuildOptions.GetFlowDatabaseName())
	tfmFlumeContext.TierceronEngine.Context = sqle.NewEmptyContext()
	tfmFlumeContext.Init(sourceDatabaseConnectionsMap, []string{flowcorehelper.TierceronFlowConfigurationTableName}, flowopts.BuildOptions.GetAdditionalFlows(), flowopts.BuildOptions.GetAdditionalFlows())
	tfmFlumeContext.Config = &configBasis
	tfmFlumeContext.ExtensionAuthData = tfmContext.ExtensionAuthData
	var flowWG sync.WaitGroup
	for _, sourceDatabaseConnectionMap := range sourceDatabaseConnectionsMap {
		for _, table := range GetTierceronTableNames() {
			tfContext := flowcore.TrcFlowContext{RemoteDataSource: make(map[string]interface{}), ReadOnly: false, Init: true, Log: tfmContext.Config.Log, ContextNotifyChan: make(chan bool, 1)}
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
				tcfContext.FlowSourceAlias = flowopts.BuildOptions.GetFlowDatabaseName()

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
			if initReceiver, ok := tfContext.RemoteDataSource["flowStateInitAlert"].(chan bool); ok {
			initAlert: //This waits for flow states to be loaded before starting all non-controller flows
				for {
					select {
					case _, ok := <-initReceiver:
						if ok {
							break initAlert
						}
					default:
						time.Sleep(time.Duration(time.Second))
					}
				}
			} else {
				initReceiverErr := errors.New("Failed to retrieve channel alert for controller init")
				eUtils.LogErrorMessage(config, initReceiverErr.Error(), false)
				return initReceiverErr
			}
		}

	}

	flowMapLock := &sync.Mutex{}
	for _, sourceDatabaseConnectionMap := range sourceDatabaseConnectionsMap {
		for _, table := range configBasis.VersionFilter {
			flowWG.Add(1)
			go func(tableFlow flowcore.FlowNameType, dc *eUtils.DriverConfig) {
				eUtils.LogInfo(dc, "Beginning data source flow: "+tableFlow.ServiceName())
				defer flowWG.Done()
				tfContext := flowcore.TrcFlowContext{RemoteDataSource: map[string]interface{}{}, FlowLock: &sync.Mutex{}, ReadOnly: false, Init: true, Log: tfmContext.Config.Log, ContextNotifyChan: make(chan bool, 1)}
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
				tfContext.FlowSourceAlias = harbingeropts.BuildOptions.GetDatabaseName()

				tfmContext.ProcessFlow(
					dc,
					&tfContext,
					flowopts.BuildOptions.ProcessFlowController,
					vaultDatabaseConfig,
					sourceDatabaseConnectionMap,
					tableFlow,
					flowcore.TableSyncFlow,
				)
			}(flowcore.FlowNameType(table), config)
		}
		for _, enhancement := range flowopts.BuildOptions.GetAdditionalFlows() {
			flowWG.Add(1)

			go func(enhancementFlow flowcore.FlowNameType, dc *eUtils.DriverConfig) {
				eUtils.LogInfo(dc, "Beginning additional flow: "+enhancementFlow.ServiceName())
				defer flowWG.Done()

				tfContext := flowcore.TrcFlowContext{RemoteDataSource: map[string]interface{}{}, FlowLock: &sync.Mutex{}, ReadOnly: false, Init: true, Log: tfmContext.Config.Log, ContextNotifyChan: make(chan bool, 1)}
				tfContext.Flow = enhancementFlow
				tfContext.RemoteDataSource["flowStateController"] = flowStateControllerMap[enhancementFlow.TableName()]
				tfContext.RemoteDataSource["flowStateReceiver"] = flowStateReceiverMap[enhancementFlow.TableName()]
				flowMapLock.Lock()
				tfmContext.FlowMap[tfContext.Flow] = &tfContext
				flowMapLock.Unlock()
				var initErr error
				dc, tfContext.GoMod, tfContext.Vault, initErr = eUtils.InitVaultMod(dc)
				if initErr != nil {
					eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start flow.", false)
					return
				}

				tfmContext.ProcessFlow(
					dc,
					&tfContext,
					flowopts.BuildOptions.ProcessFlowController,
					vaultDatabaseConfig,
					sourceDatabaseConnectionMap,
					enhancementFlow,
					flowcore.TableEnrichFlow,
				)
			}(enhancement, config)
		}

		if testopts.BuildOptions != nil {
			for _, test := range testopts.BuildOptions.GetAdditionalTestFlows() {
				flowWG.Add(1)
				go func(testFlow flowcore.FlowNameType, dc *eUtils.DriverConfig, tfmc *flowcore.TrcFlowMachineContext) {
					eUtils.LogInfo(dc, "Beginning test flow: "+testFlow.ServiceName())
					defer flowWG.Done()
					tfContext := flowcore.TrcFlowContext{RemoteDataSource: map[string]interface{}{}, FlowLock: &sync.Mutex{}, ReadOnly: false, Init: true, Log: tfmContext.Config.Log, ContextNotifyChan: make(chan bool, 1)}
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
						flowopts.BuildOptions.ProcessTestFlowController,
						vaultDatabaseConfig,
						sourceDatabaseConnectionMap,
						testFlow,
						flowcore.TableTestFlow,
					)
				}(test, config, tfmContext)
			}
		}
	}
	tfmFlumeContext.InitConfigWG.Wait()
	tfmFlumeContext.FlowControllerLock.Lock()
	tfmFlumeContext.InitConfigWG = nil
	tfmFlumeContext.FlowControllerLock.Unlock()

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

	controllerVaultDatabaseConfig["controller"] = true

	if controllerCheck == 3 {
		controllerVaultDatabaseConfig["vaddress"] = strings.Split(controllerVaultDatabaseConfig["vaddress"].(string), ":")[0]
		controllerInterfaceErr := harbingeropts.BuildOptions.BuildInterface(config, goMod, tfmFlumeContext, controllerVaultDatabaseConfig, &TrcDBServerEventListener{Log: config.Log})
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

	interfaceErr := harbingeropts.BuildOptions.BuildInterface(config, goMod, tfmContext, vaultDatabaseConfig, &TrcDBServerEventListener{Log: config.Log})
	if interfaceErr != nil {
		eUtils.LogErrorMessage(config, "Failed to start up database interface:"+interfaceErr.Error(), false)
		return interfaceErr
	}

	flowWG.Wait()

	logger.Println("ProcessFlows complete.")
	return nil
}

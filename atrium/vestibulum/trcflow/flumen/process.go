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

	"github.com/glycerine/bchan"
	trcflowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"
	"github.com/trimble-oss/tierceron/pkg/utils/config"

	"github.com/trimble-oss/tierceron/atrium/buildopts/testopts"
	trcdb "github.com/trimble-oss/tierceron/atrium/trcdb"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/pluginutil"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/pluginutil/certify"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcflow/flows/argossocii"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcflow/flows/dataflowstatistics"
	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/harbingeropts"
	"github.com/trimble-oss/tierceron/buildopts/kernelopts"
	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"

	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig"
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"

	eUtils "github.com/trimble-oss/tierceron/pkg/utils"

	sys "github.com/trimble-oss/tierceron/pkg/vaulthelper/system"

	sqle "github.com/dolthub/go-mysql-server/sql"
)

func BootFlowMachine(flowMachineInitContext *flowcore.FlowMachineInitContext, driverConfig *config.DriverConfig, pluginConfig map[string]any, logger *log.Logger) (any, error) {
	logger.Println("ProcessFlows begun.")
	// 1. Get Plugin configurations.
	var tfmContext *trcflowcore.TrcFlowMachineContext
	var vault *sys.Vault
	var goMod *helperkv.Modifier
	var err error

	_, goMod, vault, err = eUtils.InitVaultMod(driverConfig)
	if err != nil {
		if driverConfig != nil {
			eUtils.LogErrorMessage(driverConfig.CoreConfig, "Could not access vault.  Failure to start.", false)
		} else {
			logger.Println("Could not access vault.  Failure to start.")
		}
		return nil, err
	}
	goMod.Env = goMod.EnvBasis
	kernelId := pluginConfig["kernelId"].(string)

	//Need new function writing to that path using pluginName ->
	//if not copied -> this plugin should fail to start up
	//Update deployed status & return if
	if pluginNameList, ok := pluginConfig["pluginNameList"].([]string); ok {
		tempAddr := pluginConfig["vaddress"]
		tempTokenPtr := pluginConfig["tokenptr"]
		if caddress, cOk := pluginConfig["caddress"]; cOk {
			pluginConfig["vaddress"] = caddress
		}
		if cTokenPtr, cOk := pluginConfig["ctokenptr"]; cOk {
			pluginConfig["tokenptr"] = cTokenPtr
		}
		pluginConfig["exitOnFailure"] = true

		cConfig, cGoMod, cVault, err := eUtils.InitVaultMod(driverConfig)
		if err != nil {
			eUtils.LogErrorMessage(driverConfig.CoreConfig, "Could not access vault.  Failure to start.", false)
			return nil, err
		}

		// TODO: should these have capabilities?
		if len(pluginNameList) > 0 {
			pluginConfig["pluginName"] = pluginNameList[0]
		}
		pluginutil.PluginInitNewRelic(driverConfig, cGoMod, pluginConfig)
		logger = driverConfig.CoreConfig.Log

		deployedUpdateErr := certify.PluginDeployedUpdate(cConfig, cGoMod, cVault, pluginNameList, pluginConfig["certifyPath"].([]string), logger)
		if deployedUpdateErr != nil {
			eUtils.LogErrorMessage(driverConfig.CoreConfig, deployedUpdateErr.Error(), false)
			eUtils.LogErrorMessage(driverConfig.CoreConfig, "Could not update plugin deployed status in vault.", false)
			return nil, err
		}
		pluginConfig["vaddress"] = tempAddr
		pluginConfig["tokenptr"] = tempTokenPtr
		pluginConfig["exitOnFailure"] = false
	}
	logger.Println("Deployed status updated.")

	tfmContext = &trcflowcore.TrcFlowMachineContext{
		ShellRunner:               driverConfig.ShellRunner,
		Env:                       pluginConfig["env"].(string),
		KernelId:                  kernelId,
		IsSupportedFlow:           flowMachineInitContext.IsSupportedFlow,
		GetAdditionalFlowsByState: flowMachineInitContext.GetTestFlowsByState, // Chewbacca say what?!?!
		FlowMap:                   map[flowcore.FlowNameType]*trcflowcore.TrcFlowContext{},
		FlowMapLock:               sync.RWMutex{},
		FlowControllerUpdateLock:  sync.Mutex{},
		FlowControllerUpdateAlert: make(chan string, 1),
		PreloadChan:               make(chan trcflowcore.PermissionUpdate, 1),
		PermissionChan:            make(chan trcflowcore.PermissionUpdate, 1),
		DfsChan:                   flowMachineInitContext.DfsChan, // Channel for sending data flow statistics
	}
	projects, services, _ := eUtils.GetProjectServices(nil, pluginConfig["connectionPath"].([]string))
	var sourceDatabaseConfigs []map[string]any
	var vaultDatabaseConfig map[string]any
	var spiralDatabaseConfig map[string]any
	var trcIdentityConfig map[string]any
	logger.Println("Grabbing configs.")
	for i := 0; i < len(projects); i++ {
		eUtils.LogInfo(driverConfig.CoreConfig, fmt.Sprintf("Loading service: %s", services[i]))

		var regionValues []string

		if services[i] == "Database" {
			goMod.SectionName = "regionId"
			goMod.SectionKey = "/Index/"
			regions, err := goMod.ListSubsection("/Index/", projects[i], goMod.SectionName, logger)
			if err != nil {
				eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
				eUtils.LogInfo(driverConfig.CoreConfig, "Skipping service: "+services[i])
				continue
			}
			regionValues = regions
			// Filter by allowed if in kernel mode.
			if kernelopts.BuildOptions.IsKernel() {
				regionValues = eUtils.FilterSupportedRegions(driverConfig, regionValues)
			}
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
			eUtils.LogInfo(driverConfig.CoreConfig, fmt.Sprintf("Processing region: %s", regionValue))
			goMod.SubSectionValue = regionValue
			ok := false
			properties, err := trcvutils.NewProperties(driverConfig.CoreConfig, vault, goMod, pluginConfig["env"].(string), projects[i], services[i])
			if err != nil {
				eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
				return nil, err
			}

			switch services[i] {
			case "Database":
				var sourceDatabaseConfig map[string]any

				sourceDatabaseConfig, ok = properties.GetConfigValues(services[i], "config")
				if !ok || len(sourceDatabaseConfig) == 0 {
					// Just ignore this one and go to the next one.
					eUtils.LogWarningMessage(driverConfig.CoreConfig, "Expected database configuration does not exist: "+regionValue, false)
					continue
				}
				for _, supportedRegion := range buildopts.BuildOptions.GetSupportedSourceRegions() {
					if sourceDatabaseConfig["dbsourceregion"] == supportedRegion {
						eUtils.LogInfo(driverConfig.CoreConfig, fmt.Sprintf("Loading data source: %s for region: %s", services[i], regionValue))
						sourceDatabaseConfigs = append(sourceDatabaseConfigs, sourceDatabaseConfig)
					}
				}

			case "Identity":
				trcIdentityConfig, ok = properties.GetConfigValues(services[i], "config")
				if !ok {
					eUtils.LogErrorMessage(driverConfig.CoreConfig, "Couldn't get config values.", false)
					return nil, err
				}
			case "VaultDatabase":
				if len(flowMachineInitContext.FlowMachineInterfaceConfigs) > 0 {
					// This is a special case where we are using the plugin to create a vault database interface.
					// We need to use the config from the flow machine interface configs.
					// Only supported on loopback interface fo security reasons.
					vaultDatabaseConfig = flowMachineInitContext.FlowMachineInterfaceConfigs
					vaultDatabaseConfig["vaddress"] = "127.0.0.1"
					ok = true
				} else {
					vaultDatabaseConfig, ok = properties.GetConfigValues(services[i], "config")
					vaultDatabaseConfig["vaddress"] = pluginConfig["vaddress"]
				}
				if !ok {
					eUtils.LogErrorMessage(driverConfig.CoreConfig, "Couldn't get config values.", false)
					return nil, err
				}
			case "SpiralDatabase":
				spiralDatabaseConfig, ok = properties.GetConfigValues(services[i], "config")
				if !ok {
					eUtils.LogErrorMessage(driverConfig.CoreConfig, "Couldn't get config values.", false)
					return nil, err
				}
			}
		}

	}

	eUtils.LogInfo(driverConfig.CoreConfig, "Finished retrieving configs")
	sourceDatabaseConnectionsMap := map[string]map[string]any{}
	currentTokenNamePtr := driverConfig.CoreConfig.GetCurrentToken("config_token_%s")

	// 4. Create config for vault for queries to vault.
	driverConfigBasis := config.DriverConfig{
		CoreConfig: &coreconfig.CoreConfig{
			Regions:             driverConfig.CoreConfig.Regions,
			CurrentTokenNamePtr: currentTokenNamePtr,
			TokenCache:          driverConfig.CoreConfig.TokenCache,
			Insecure:            driverConfig.CoreConfig.Insecure,
			Env:                 driverConfig.CoreConfig.Env,
			EnvBasis:            driverConfig.CoreConfig.EnvBasis,
			IsEditor:            driverConfig.CoreConfig.IsEditor,
			ExitOnFailure:       driverConfig.CoreConfig.ExitOnFailure,
			Log:                 driverConfig.CoreConfig.Log,
		},
	}

	// Need to create askflumeflow template --> fill with default vals
	templateList := pluginConfig["templatePath"].([]string)
	flowTemplateMap := map[string]string{}
	flowSourceMap := map[string]string{}
	flowStateControllerMap := map[string]chan flowcore.CurrentFlowState{}
	flowStateReceiverMap := map[string]chan flowcore.FlowStateUpdate{}

	for _, tableFlow := range flowMachineInitContext.GetTableFlows() {
		tableName := tableFlow.FlowHeader.Name.TableName()
		if tableName != flowcore.TierceronControllerFlow.TableName() && !flowMachineInitContext.IsSupportedFlow(tableName) {
			if !driverConfigBasis.CoreConfig.IsEditor {
				eUtils.LogInfo(driverConfigBasis.CoreConfig, "Skipping unsupported process flow: "+tableName)
			}
			continue
		}
		if tableName != flowcore.TierceronControllerFlow.TableName() {
			driverConfigBasis.VersionFilter = append(driverConfigBasis.VersionFilter, tableName)
		}
		flowTemplateMap[tableName] = tableFlow.FlowTemplatePath
		flowSourceMap[tableName] = tableFlow.FlowHeader.Source
		flowStateControllerMap[tableName] = make(chan flowcore.CurrentFlowState, 1)
		flowStateReceiverMap[tableName] = make(chan flowcore.FlowStateUpdate, 1)
	}

	for _, enhancement := range flowMachineInitContext.GetFilteredBusinessFlows(kernelId) {
		flowStateControllerMap[enhancement.FlowHeader.TableName()] = make(chan flowcore.CurrentFlowState, 1)
		flowStateReceiverMap[enhancement.FlowHeader.TableName()] = make(chan flowcore.FlowStateUpdate, 1)
	}

	tfmContext.SetFlumeDbType(flowcore.TrcDb)
	tfmContext.TierceronEngine, err = trcdb.CreateEngine(&driverConfigBasis, templateList, pluginConfig["env"].(string), coreopts.BuildOptions.GetDatabaseName(flowcore.TrcDb))
	tfmContext.DriverConfig = &driverConfigBasis

	if err != nil {
		eUtils.LogErrorMessage(driverConfig.CoreConfig, "Couldn't build engine.", false)
		return nil, err
	}
	eUtils.LogInfo(driverConfig.CoreConfig, "Finished building engine")

	// 2. Establish mysql connection to remote mysql instance.
	for _, sourceDatabaseConfig := range sourceDatabaseConfigs {
		dbSourceConnBundle := map[string]any{}
		dbSourceConnBundle["dbsourceurl"] = sourceDatabaseConfig["dbsourceurl"].(string)
		dbSourceConnBundle["dbsourceuser"] = sourceDatabaseConfig["dbsourceuser"].(string)
		dbSourceConnBundle["dbsourcepassword"] = sourceDatabaseConfig["dbsourcepassword"].(string)
		dbSourceConnBundle["dbsourceregion"] = sourceDatabaseConfig["dbsourceregion"].(string)

		dbSourceConnBundle["encryptionSecret"] = sourceDatabaseConfig["dbencryptionSecret"].(string)
		if dbIngestInterval, ok := sourceDatabaseConfig["dbingestinterval"]; ok {
			ingestInterval, err := strconv.ParseInt(dbIngestInterval.(string), 10, 64)
			if err == nil {
				eUtils.LogInfo(driverConfig.CoreConfig, "Ingest interval: "+dbIngestInterval.(string))
				dbSourceConnBundle["dbingestinterval"] = time.Duration(ingestInterval)
			}
		} else {
			eUtils.LogErrorMessage(driverConfig.CoreConfig, "Ingest interval invalid - Defaulting to 60 minutes.", false)
			dbSourceConnBundle["dbingestinterval"] = time.Duration(60)
		}

		sourceDatabaseConnectionsMap[sourceDatabaseConfig["dbsourceregion"].(string)] = dbSourceConnBundle
	}
	//time.Sleep(8 * time.Second)

	eUtils.LogInfo(driverConfig.CoreConfig, "Finished building source configs")
	// Http query resources include:
	// 1. Auth -- Auth is provided by the external library.
	// 2. Get json by Api call.
	extensionAuthComponents := buildopts.BuildOptions.GetExtensionAuthComponents(trcIdentityConfig)
	if len(extensionAuthComponents) > 0 {

		if !strings.HasPrefix(extensionAuthComponents["authDomain"].(string), "https://") {
			eUtils.LogInfo(driverConfig.CoreConfig, "Invalid identity domain.  Must be https://...")
		}

		httpClient, err := helperkv.CreateHTTPClient(false, extensionAuthComponents["authDomain"].(string), pluginConfig["env"].(string), false)
		if httpClient != nil {
			defer httpClient.CloseIdleConnections()
		}
		if err != nil {
			eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
			return nil, err
		}

		eUtils.LogInfo(driverConfig.CoreConfig, "Finished creating auth extension connection")

		tfmContext.ExtensionAuthData, _, err = trcvutils.GetJSONFromClientByPost(driverConfig.CoreConfig, httpClient, extensionAuthComponents["authHeaders"].(map[string]string), extensionAuthComponents["authUrl"].(string), extensionAuthComponents["bodyData"].(io.Reader))
		if err != nil {
			eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
			//return err
		}
		// Set up reloader in case things go sideways later on.
		tfmContext.ExtensionAuthDataReloader = make(map[string]any, 1)
		tfmContext.ExtensionAuthDataReloader["config"] = driverConfig
		tfmContext.ExtensionAuthDataReloader["identityConfig"] = trcIdentityConfig
	}

	eUtils.LogInfo(driverConfig.CoreConfig, "Finished building source extension configs")

	// 2. Initialize Engine and create changes table.
	tfmContext.TierceronEngine.Context = sqle.NewEmptyContext()
	var filteredFlowNames []string
	for _, flow := range flowMachineInitContext.GetFilteredTableFlowDefinitions(kernelId) {
		filteredFlowNames = append(filteredFlowNames, flow.FlowHeader.FlowName())
	}

	tfmContext.Init(sourceDatabaseConnectionsMap, filteredFlowNames, flowMachineInitContext.GetFilteredBusinessFlowNames(kernelId), flowMachineInitContext.GetFilteredTestFlowNames(kernelId))

	//Initialize tfcContext for flow controller
	tfmFlumeContext := &trcflowcore.TrcFlowMachineContext{
		InitConfigWG:              &sync.WaitGroup{},
		Env:                       pluginConfig["env"].(string),
		KernelId:                  kernelId,
		IsSupportedFlow:           flowMachineInitContext.IsSupportedFlow,
		GetAdditionalFlowsByState: flowMachineInitContext.GetTestFlowsByState,
		FlowMap:                   tfmContext.FlowMap, // In order to support flow notifications, we need this here.
		FlowControllerInit:        true,
		FlowControllerUpdateLock:  sync.Mutex{},
		FlowControllerUpdateAlert: make(chan string, 1),
		PreloadChan:               make(chan trcflowcore.PermissionUpdate, 1),
		PermissionChan:            make(chan trcflowcore.PermissionUpdate, 1),
		DfsChan:                   flowMachineInitContext.DfsChan, // Channel for sending data flow statistics
	}

	if len(sourceDatabaseConnectionsMap) == 0 {
		sourceDatabaseConnectionsMap = make(map[string]map[string]any, 1)
		sourceDatabaseDetails := make(map[string]any, 1)
		sourceDatabaseDetails["dbsourceregion"] = "NA"
		var d time.Duration = 60
		sourceDatabaseDetails["dbingestinterval"] = d
		sourceDatabaseConnectionsMap["NA"] = sourceDatabaseDetails
		sourceDatabaseDetails["sqlConn"] = nil
	}

	eUtils.LogInfo(driverConfig.CoreConfig, "Finished building engine & changes tables")

	tfmFlumeContext.SetFlumeDbType(flowcore.TrcFlumeDb)
	tfmFlumeContext.TierceronEngine, err = trcdb.CreateEngine(&driverConfigBasis, templateList, pluginConfig["env"].(string), coreopts.BuildOptions.GetDatabaseName(flowcore.TrcFlumeDb))
	if err != nil {
		return nil, err
	}
	tfmFlumeContext.TierceronEngine.Context = sqle.NewEmptyContext()
	tfmFlumeContext.DriverConfig = &driverConfigBasis
	tfmFlumeContext.Init(sourceDatabaseConnectionsMap, []string{flowcore.TierceronControllerFlow.FlowName()}, []flowcore.FlowNameType{}, []flowcore.FlowNameType{})
	tfmFlumeContext.ExtensionAuthData = tfmContext.ExtensionAuthData
	var flowWG sync.WaitGroup

	for _, table := range GetTierceronTableNames() {
		if table != flowcore.TierceronControllerFlow.TableName() && !flowMachineInitContext.IsSupportedFlow(table) {
			if !driverConfigBasis.CoreConfig.IsEditor {
				eUtils.LogInfo(driverConfigBasis.CoreConfig, "Skipping unsupported flow: "+table)
			}
			continue
		}

		tfContext := trcflowcore.TrcFlowContext{RemoteDataSource: make(map[string]any), QueryLock: &sync.Mutex{}, FlowStateLock: &sync.RWMutex{}, PreviousFlowStateLock: &sync.RWMutex{}, ReadOnly: false, Init: true, Logger: tfmContext.DriverConfig.CoreConfig.Log, ContextNotifyChan: make(chan bool, 1), FlowLoadedNotifyChan: bchan.New(1)}
		tfContext.RemoteDataSource["flowStateControllerMap"] = flowStateControllerMap
		tfContext.RemoteDataSource["flowStateReceiverMap"] = flowStateReceiverMap
		tfContext.RemoteDataSource["flowStateInitAlert"] = make(chan bool, 1)
		var controllerInitWG sync.WaitGroup
		tfContext.RemoteDataSource["controllerInitWG"] = &controllerInitWG
		controllerInitWG.Add(1)
		tfmFlumeContext.InitConfigWG.Add(1)
		flowWG.Add(1)
		tableFlow := flowcore.FlowHeaderType{Name: flowcore.FlowNameType(table), Instances: "*"}
		tfContext.FlowHeader = &tableFlow
		flowSource, _, _, flowPath := eUtils.GetProjectService(nil, pluginConfig["flumeTemplatePath"].([]string)[0])
		tfContext.FlowHeader.Source = flowSource
		tfContext.FlowPath = flowPath
		tfmContext.FlowMapLock.Lock()
		tfmContext.FlowMap[flowcore.FlowNameType(tfContext.FlowHeader.FlowName())] = &tfContext
		tfmContext.FlowMapLock.Unlock()

		go func(tcfContext *trcflowcore.TrcFlowContext, dc *config.DriverConfig) {
			eUtils.LogInfo(dc.CoreConfig, "Beginning controller flow: "+tcfContext.FlowHeader.ServiceName())
			defer flowWG.Done()
			var initErr error
			_, tcfContext.GoMod, tcfContext.Vault, initErr = eUtils.InitVaultMod(dc)
			if initErr != nil {
				eUtils.LogErrorMessage(driverConfig.CoreConfig, "Could not access vault.  Failure to start flow.", false)
				return
			}
			tcfContext.GoMod.Env = tcfContext.GoMod.EnvBasis
			tcfContext.FlowHeader.SourceAlias = coreopts.BuildOptions.GetDatabaseName(flowcore.TrcFlumeDb)

			tfmFlumeContext.ProcessFlow(
				tcfContext,
				FlumenProcessFlowController,
				vaultDatabaseConfig,
				sourceDatabaseConnectionsMap,
				tfContext.FlowHeader.Name,
				trcflowcore.TableSyncFlow,
			)
		}(&tfContext, &driverConfigBasis)

		controllerInitWG.Wait() //Waiting for remoteDataSource to load up to prevent data race.
		if initReceiver, ok := tfContext.RemoteDataSource["flowStateInitAlert"].(chan bool); ok {
			eUtils.LogInfo(driverConfig.CoreConfig, "Controller has been initialized...sending alert to interface...")
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
			eUtils.LogErrorMessage(driverConfig.CoreConfig, initReceiverErr.Error(), false)
			return nil, initReceiverErr
		}
	}

	for _, table := range flowMachineInitContext.GetTableFlows() {
		if table.FlowHeader.TableName() == flowcore.TierceronControllerFlow.TableName() || !flowMachineInitContext.IsSupportedFlow(table.FlowHeader.FlowName()) {
			if !driverConfigBasis.CoreConfig.IsEditor {
				eUtils.LogInfo(driverConfigBasis.CoreConfig, "Skipping unsupported flow: "+table.FlowHeader.FlowName())
			}
			continue
		}
		tfContext := trcflowcore.TrcFlowContext{RemoteDataSource: map[string]any{}, QueryLock: &sync.Mutex{}, FlowStateLock: &sync.RWMutex{}, PreviousFlowStateLock: &sync.RWMutex{}, ReadOnly: false, Init: true, Logger: tfmContext.DriverConfig.CoreConfig.Log, ContextNotifyChan: make(chan bool, 1), FlowLoadedNotifyChan: bchan.New(1)}
		tableFlow := table
		tfContext.RemoteDataSource["flowStateController"] = flowStateControllerMap[tableFlow.FlowHeader.TableName()]
		tfContext.RemoteDataSource["flowStateReceiver"] = flowStateReceiverMap[tableFlow.FlowHeader.TableName()]
		tfContext.FlowHeader = &tableFlow.FlowHeader
		tfContext.FlowHeader.Source = flowSourceMap[tableFlow.FlowHeader.TableName()]
		tfContext.FlowPath = flowTemplateMap[tableFlow.FlowHeader.TableName()]
		tfmContext.FlowMapLock.Lock()
		tfmContext.FlowMap[flowcore.FlowNameType(tfContext.FlowHeader.FlowName())] = &tfContext
		tfmContext.FlowMapLock.Unlock()

		flowWG.Add(1)
		go func(tcfContext *trcflowcore.TrcFlowContext, dc *config.DriverConfig) {
			eUtils.LogInfo(dc.CoreConfig, "Beginning data source flow: "+tcfContext.FlowHeader.ServiceName())
			defer flowWG.Done()
			var initErr error
			_, tcfContext.GoMod, tfContext.Vault, initErr = eUtils.InitVaultMod(dc)
			if initErr != nil {
				eUtils.LogErrorMessage(driverConfig.CoreConfig, "Could not access vault.  Failure to start flow.", false)
				return
			}
			tfContext.GoMod.Env = tfContext.GoMod.EnvBasis
			flowPath := fmt.Sprintf("super-secrets/Index/FlumeDatabase/flowName/%s/%s", tcfContext.FlowHeader.TableName(), flowcore.TierceronControllerFlow.FlowName())
			dataMap, readErr := tfContext.GoMod.ReadData(flowPath)
			if readErr == nil && len(dataMap) > 0 {
				if dataMap["flowAlias"] != nil {
					tfContext.FlowState.FlowAlias = dataMap["flowAlias"].(string)
				}
			}
			tcfContext.FlowHeader.SourceAlias = coreopts.BuildOptions.GetDatabaseName(flowcore.TrcDb)
			if tcfContext.FlowHeader.FlowName() == flowcore.ArgosSociiFlow.FlowName() {
				go func(tfmContext *trcflowcore.TrcFlowMachineContext, tfContext *trcflowcore.TrcFlowContext) {
					for tableLoadedPerm := range tfmContext.PreloadChan {
						if tableLoadedPerm.TableName == flowcore.ArgosSociiFlow.FlowName() {
							populateArgosSocii(tfContext.GoMod, driverConfig, tfmContext)
							tfContext.NotifyFlowComponentLoaded()
							break
						}
					}
				}(tfmContext, &tfContext)
			}
			tfmContext.ProcessFlow(
				&tfContext,
				func(tfmContext flowcore.FlowMachineContext, tcfContext flowcore.FlowContext) error {
					switch tcfContext.GetFlowHeader().FlowName() {
					case flowcore.DataFlowStatConfigurationsFlow.FlowName():
						// DFS flow always handled internally.
						// TODO: Enhance to wait for all but dfs flow...
						// Then the existing WaitAllFlowsLoaded should
						// wait just for dfs to load.
						// This ensured dfs has latest data.
						//						tfmContext.WaitAllFlowsLoaded()
						return dataflowstatistics.ProcessDataFlowStatConfigurations(tfmContext, tcfContext)
					case flowcore.ArgosSociiFlow.FlowName():
						tfContext.SetFlowLibraryContext(argossocii.GetProcessFlowDefinition())
						return flowcore.ProcessTableConfigurations(tfmContext, tcfContext)
					default:
						return flowMachineInitContext.FlowController(tfmContext, tcfContext)
					}
				},
				vaultDatabaseConfig,
				sourceDatabaseConnectionsMap,
				tcfContext.FlowHeader.Name,
				trcflowcore.TableSyncFlow,
			)
		}(&tfContext, &driverConfigBasis)
	}

	for _, businessFlow := range flowMachineInitContext.GetFilteredBusinessFlows(kernelId) {
		if !flowMachineInitContext.IsSupportedFlow(businessFlow.FlowHeader.FlowName()) {
			if !driverConfigBasis.CoreConfig.IsEditor {
				eUtils.LogInfo(tfmContext.DriverConfig.CoreConfig, "Skipping unsupported business flow: "+businessFlow.FlowHeader.FlowName())
			}
			continue
		}

		flowWG.Add(1)

		go func(bizFlow flowcore.FlowDefinition, dc *config.DriverConfig) {
			eUtils.LogInfo(dc.CoreConfig, "Beginning additional flow: "+bizFlow.FlowHeader.ServiceName())
			defer flowWG.Done()

			tfContext := trcflowcore.TrcFlowContext{RemoteDataSource: map[string]any{}, QueryLock: &sync.Mutex{}, FlowStateLock: &sync.RWMutex{}, PreviousFlowStateLock: &sync.RWMutex{}, ReadOnly: false, Init: true, Logger: tfmContext.DriverConfig.CoreConfig.Log, ContextNotifyChan: make(chan bool, 1), FlowLoadedNotifyChan: bchan.New(1)}
			tfContext.FlowHeader = &bizFlow.FlowHeader
			tfContext.RemoteDataSource["flowStateController"] = flowStateControllerMap[bizFlow.FlowHeader.TableName()]
			tfContext.RemoteDataSource["flowStateReceiver"] = flowStateReceiverMap[bizFlow.FlowHeader.TableName()]
			tfmContext.FlowMapLock.Lock()
			tfmContext.FlowMap[flowcore.FlowNameType(tfContext.FlowHeader.FlowName())] = &tfContext
			tfmContext.FlowMapLock.Unlock()
			var initErr error
			_, tfContext.GoMod, tfContext.Vault, initErr = eUtils.InitVaultMod(dc)
			if initErr != nil {
				eUtils.LogErrorMessage(driverConfig.CoreConfig, "Could not access vault.  Failure to start flow.", false)
				return
			}
			tfContext.GoMod.Env = tfContext.GoMod.EnvBasis

			tfmContext.ProcessFlow(
				&tfContext,
				flowMachineInitContext.FlowController,
				vaultDatabaseConfig, // unused.
				sourceDatabaseConnectionsMap,
				bizFlow.FlowHeader.FlowNameType(),
				trcflowcore.TableEnrichFlow,
			)
		}(businessFlow, &driverConfigBasis)
	}

	if testopts.BuildOptions != nil {
		for _, test := range flowMachineInitContext.GetTestFlows() {
			flowWG.Add(1)
			go func(testFlow flowcore.FlowDefinition, dc *config.DriverConfig, tfmc *trcflowcore.TrcFlowMachineContext) {
				eUtils.LogInfo(dc.CoreConfig, "Beginning test flow: "+testFlow.FlowHeader.ServiceName())
				defer flowWG.Done()
				tfContext := trcflowcore.TrcFlowContext{RemoteDataSource: map[string]any{}, QueryLock: &sync.Mutex{}, FlowStateLock: &sync.RWMutex{}, PreviousFlowStateLock: &sync.RWMutex{}, ReadOnly: false, Init: true, Logger: tfmContext.DriverConfig.CoreConfig.Log, ContextNotifyChan: make(chan bool, 1), FlowLoadedNotifyChan: bchan.New(1)}
				tfContext.FlowHeader = &testFlow.FlowHeader
				var initErr error
				dc, tfContext.GoMod, tfContext.Vault, initErr = eUtils.InitVaultMod(dc)
				if initErr != nil {
					eUtils.LogErrorMessage(dc.CoreConfig, "Could not access vault.  Failure to start flow.", false)
					return
				}
				tfContext.GoMod.Env = tfContext.GoMod.EnvBasis

				tfmc.ProcessFlow(
					&tfContext,
					flowMachineInitContext.TestFlowController,
					vaultDatabaseConfig, // unused..
					sourceDatabaseConnectionsMap,
					tfContext.FlowHeader.Name,
					trcflowcore.TableTestFlow,
				)
			}(test, &driverConfigBasis, tfmContext)
		}
	}

	go func() {
		err := BuildFlumeDatabaseInterface(tfmFlumeContext, tfmContext, goMod, vaultDatabaseConfig, spiralDatabaseConfig, &flowWG)
		if err != nil {
			tfmContext.DriverConfig.CoreConfig.Log.Println("Error building flume database interface:", err)
		}
	}()

	logger.Println("ProcessFlows complete.")
	return tfmContext, nil
}

func populateArgosSocii(goMod *helperkv.Modifier, driverConfig *config.DriverConfig, tfmContext flowcore.FlowMachineContext) {
	goMod.Reset()
	if flow := tfmContext.GetFlowContext(flowcore.ArgosSociiFlow.Name); flow != nil {
		if flow.GetFlowLibraryContext() != nil && flow.GetFlowLibraryContext().GetTableConfigurationInsert != nil {
			pluginNameValues, err := goMod.List("super-secrets/Index/TrcVault/trcplugin", driverConfig.CoreConfig.Log)
			argosId := 0

			if err == nil && pluginNameValues != nil {
				retrictedMappingsMap := coreopts.BuildOptions.GetPluginRestrictedMappings()

				for _, pluginNameValue := range pluginNameValues.Data["keys"].([]any) {
					if pluginName := pluginNameValue.(string); pluginName != "" {
						pluginName = strings.TrimSuffix(pluginName, "/")
						if _, ok := retrictedMappingsMap[pluginName]; !ok {
							continue
						}

						pluginMap, err := goMod.ReadData(fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", pluginName))
						if err == nil {
							if projectService, ok := pluginMap["trcprojectservice"].(string); ok && len(projectService) > 0 {
								projectServiceSlice := strings.Split(projectService, "/")
								argosId = argosId + 1
								var data = make(map[string]any)
								data["argosId"] = fmt.Sprintf("%d", argosId)
								data["argosIdentitasNomen"] = pluginName
								data["argosProiectum"] = projectServiceSlice[0]
								data["argosServitium"] = projectServiceSlice[1]
								data["argosNotitia"] = "Tierceron service"

								flowInsertQueryMap := flow.GetFlowLibraryContext().GetTableConfigurationInsert(data, flow.GetFlowHeader().SourceAlias, flowcore.ArgosSociiFlow.FlowName())
								tfmContext.CallDBQuery(flow, flowInsertQueryMap, nil, false, "INSERT", nil, "")
							}
						}
					}
				}
			}
		}
	}
}

func BuildFlumeDatabaseInterface(tfmFlumeContext *trcflowcore.TrcFlowMachineContext, tfmContext *trcflowcore.TrcFlowMachineContext, goMod *helperkv.Modifier, vaultDatabaseConfig map[string]any, spiralDatabaseConfig map[string]any, flowWG *sync.WaitGroup) error {
	eUtils.LogInfo(tfmFlumeContext.DriverConfig.CoreConfig, "Waiting for controller initialization...")
	tfmFlumeContext.InitConfigWG.Wait()
	tfmFlumeContext.FlowControllerLock.Lock()
	tfmFlumeContext.InitConfigWG = nil
	tfmFlumeContext.FlowControllerLock.Unlock()

	//Set up controller config
	controllerVaultDatabaseConfig := make(map[string]any)
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
		eUtils.LogInfo(tfmFlumeContext.DriverConfig.CoreConfig, "Starting controller interface...")
		controllerVaultDatabaseConfig["vaddress"] = strings.Split(controllerVaultDatabaseConfig["vaddress"].(string), ":")[0]
		controllerInterfaceErr := harbingeropts.BuildOptions.BuildInterface(tfmFlumeContext.DriverConfig, goMod, tfmFlumeContext, controllerVaultDatabaseConfig, &TrcDBServerEventListener{Log: tfmFlumeContext.DriverConfig.CoreConfig.Log})
		if controllerInterfaceErr != nil {
			eUtils.LogErrorMessage(tfmFlumeContext.DriverConfig.CoreConfig, "Failed to start up controller database interface:"+controllerInterfaceErr.Error(), false)
			return controllerInterfaceErr
		}
	} else {
		go func(tfC *trcflowcore.TrcFlowMachineContext) {
			for permUpdate := range tfC.PermissionChan {
				tableName := permUpdate.TableName

				if tableName != flowcore.ArgosSociiFlow.TableName() {
					harbingeropts.BuildOptions.TableGrantNotify(tfC, tableName)
				}
			}
		}(tfmFlumeContext)
	}

	// Starts up dolt mysql instance listening on a port so we can use the plugin instead to host vault encrypted data.
	// Variables such as username, password, port are in vaultDatabaseConfig -- configs coming from encrypted vault.
	// The engine is in tfmContext...  that's the one we need to make available for connecting via dbvis...
	// be sure to enable encryption on the connection...

	if vaultDatabaseConfig["dbuser"] != nil && vaultDatabaseConfig["dbpassword"] != nil && vaultDatabaseConfig["dbport"] != nil {
		//Setting up DFS USER
		if dfsUser, ok := spiralDatabaseConfig["dbuser"]; ok {
			vaultDatabaseConfig["dfsUser"] = dfsUser
		}
		if dfsPass, ok := spiralDatabaseConfig["dbpassword"]; ok {
			vaultDatabaseConfig["dfsPass"] = dfsPass
		}
		eUtils.LogInfo(tfmFlumeContext.DriverConfig.CoreConfig, "Starting db interface...")
		interfaceErr := harbingeropts.BuildOptions.BuildInterface(tfmFlumeContext.DriverConfig, goMod, tfmContext, vaultDatabaseConfig, &TrcDBServerEventListener{Log: tfmFlumeContext.DriverConfig.CoreConfig.Log})
		if interfaceErr != nil {
			eUtils.LogErrorMessage(tfmFlumeContext.DriverConfig.CoreConfig, "Failed to start up database interface:"+interfaceErr.Error(), false)
			return interfaceErr
		}
	} else {
		go func(tfC *trcflowcore.TrcFlowMachineContext) {
			for permUpdate := range tfC.PermissionChan {
				tableName := permUpdate.TableName

				if tableName != flowcore.ArgosSociiFlow.TableName() {
					harbingeropts.BuildOptions.TableGrantNotify(tfC, tableName)
				}
			}
		}(tfmContext)
	}

	// Databases not fully online until th flowWG is done and released.
	flowWG.Wait()

	return nil
}

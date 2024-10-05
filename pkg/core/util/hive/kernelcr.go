package hive

import (
	"errors"
	"fmt"
	"log"
	"plugin"
	"reflect"
	"strings"

	"github.com/trimble-oss/tierceron-core/v2/core"
	flowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	vcutils "github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase/utils"
	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/system"
)

var PluginMods map[string]*plugin.Plugin = map[string]*plugin.Plugin{}
var logger *log.Logger
var dfstat *core.TTDINode

type PluginHandler struct {
	IsRunning     bool
	sender        chan int
	err_receiver  chan error
	ttdi_receiver chan *core.TTDINode
	Services      *[]string
}

func Init(pluginName string, properties *map[string]interface{}) {
	if PluginMods == nil || PluginMods[pluginName] == nil {
		logger.Println("No plugin module set for initializing plugin service.")
		return
	}
	symbol, err := PluginMods[pluginName].Lookup("Init")
	if err != nil {
		fmt.Println(err)
		logger.Printf("Unable to lookup plugin export: %s\n", err)
	}
	logger.Printf("Initializing plugin module for %s\n", pluginName)
	reflect.ValueOf(symbol).Call([]reflect.Value{reflect.ValueOf(properties)})
}

func (pluginHandler *PluginHandler) PluginserviceStart(driverConfig *eUtils.DriverConfig, pluginToolConfig map[string]interface{}) {
	if driverConfig.CoreConfig.Log != nil {
		if logger == nil {
			logger = driverConfig.CoreConfig.Log
		}
	} else {
		fmt.Println("No logger passed in to plugin service")
		return
	}
	pluginName := driverConfig.SubSectionValue
	if len(pluginName) == 0 {
		driverConfig.CoreConfig.Log.Println("No plugin name specified to start plugin service.")
		return
	}
	if PluginMods == nil || PluginMods[pluginName] == nil {
		driverConfig.CoreConfig.Log.Printf("No plugin module initialized to start plugin service: %s\n", pluginName)
		return
	}
	var service string
	if s, ok := driverConfig.DeploymentConfig["trcplugin"].(string); ok {
		service = s
	} else {
		fmt.Println("Unable to process plugin service.")
		driverConfig.CoreConfig.Log.Println("Unable to process plugin service.")
		return
	}
	driverConfig.CoreConfig.Log.Printf("Starting initialization for plugin service: %s\n", service)
	pluginConfig := make(map[string]interface{})
	pluginConfig["vaddress"] = *driverConfig.CoreConfig.VaultAddressPtr
	pluginConfig["tokenptr"] = driverConfig.CoreConfig.TokenPtr
	pluginConfig["env"] = driverConfig.CoreConfig.EnvBasis

	_, mod, vault, err := eUtils.InitVaultModForPlugin(pluginConfig, driverConfig.CoreConfig.Log)
	if err != nil {
		fmt.Printf("Problem initializing mod: %s\n", err)
		driverConfig.CoreConfig.Log.Printf("Problem initializing mod: %s\n", err)
		return
	}

	if vault != nil {
		defer vault.Close()
	}
	if pluginprojserv, ok := pluginToolConfig["trcprojectservice"]; ok {
		if projserv, k := pluginprojserv.(string); k {
			projServ := strings.Split(projserv, "/")
			if len(projServ) != 2 {
				fmt.Printf("Improper formatting of project/service for %s\n", service)
				driverConfig.CoreConfig.Log.Printf("Improper formatting of project/service for %s\n", service)
				return
			}
			properties, err := trcvutils.NewProperties(&driverConfig.CoreConfig, vault, mod, mod.Env, projServ[0], projServ[1])
			if err != nil && !strings.Contains(err.Error(), "no data paths found when initing CDS") {
				fmt.Println("Couldn't create properties for regioned certify:" + err.Error())
				return
			}
			getConfigPaths, err := PluginMods[pluginName].Lookup("GetConfigPaths")
			if err != nil {
				driverConfig.CoreConfig.Log.Printf("Unable to access config for %s\n", service)
				driverConfig.CoreConfig.Log.Printf("Returned with %v\n", err)
				fmt.Printf("Unable to access config for %s\n", service)
				fmt.Printf("Returned with %v\n", err)
				return
			}
			pluginConfigPaths := getConfigPaths.(func() []string)
			paths := pluginConfigPaths()
			serviceConfig := make(map[string]interface{})
			for _, path := range paths {
				if strings.HasPrefix(path, "Common") {
					cert_ps := strings.Split(path, "/")
					if len(cert_ps) != 2 {
						eUtils.LogErrorObject(&driverConfig.CoreConfig, errors.New("unable to process cert"), false)
					}
					templatePath := "./trc_templates/" + path
					driverConfig.CoreConfig.WantCerts = true
					_, configuredCert, _, err := vcutils.ConfigTemplate(driverConfig, mod, templatePath, true, cert_ps[0], cert_ps[1], true, true)
					if err != nil {
						eUtils.LogErrorObject(&driverConfig.CoreConfig, err, false)
					}
					driverConfig.CoreConfig.WantCerts = false
					serviceConfig[path] = []byte(configuredCert[1])
				} else {
					sc, ok := properties.GetConfigValues(projServ[1], path)
					if !ok {
						fmt.Printf("Unable to access configuration data for %s\n", service)
						driverConfig.CoreConfig.Log.Printf("Unable to access configuration data for %s\n", service)
						return
					}
					serviceConfig[path] = &sc
				}
			}
			// Initialize channels
			pluginHandler.sender = make(chan int)
			pluginHandler.err_receiver = make(chan error)
			pluginHandler.ttdi_receiver = make(chan *core.TTDINode)
			chan_map := make(map[string]interface{})
			chan_map[core.PLUGIN_CHANNEL_EVENT_IN] = pluginHandler.sender
			chan_map[core.PLUGIN_CHANNEL_EVENT_OUT] = make(map[string]interface{})
			chan_map[core.PLUGIN_CHANNEL_EVENT_OUT].(map[string]interface{})[core.ERROR_CHANNEL] = pluginHandler.err_receiver
			chan_map[core.PLUGIN_CHANNEL_EVENT_OUT].(map[string]interface{})[core.DATA_FLOW_STAT_CHANNEL] = pluginHandler.ttdi_receiver
			serviceConfig[core.PLUGIN_EVENT_CHANNELS_MAP_KEY] = chan_map
			serviceConfig["log"] = driverConfig.CoreConfig.Log
			serviceConfig["env"] = driverConfig.CoreConfig.Env
			go pluginHandler.handle_errors(driverConfig)
			go pluginHandler.handle_dataflowstat(driverConfig, mod, vault)
			Init(pluginName, &serviceConfig)
			driverConfig.CoreConfig.Log.Printf("Sending start message to plugin service %s\n", service)
			pluginHandler.sender <- core.PLUGIN_EVENT_START
			pluginHandler.IsRunning = true
			driverConfig.CoreConfig.Log.Printf("Successfully sent start message to plugin service %s\n", service)
		}
	}
}

func (pluginHandler *PluginHandler) handle_errors(driverConfig *eUtils.DriverConfig) {
	for {
		result := <-pluginHandler.err_receiver
		switch {
		case result != nil:
			fmt.Println(result)
			eUtils.LogErrorObject(&driverConfig.CoreConfig, result, false)
			return
		}
	}
}

func (pluginHandler *PluginHandler) handle_dataflowstat(driverConfig *eUtils.DriverConfig, mod *kv.Modifier, vault *system.Vault) {
	// tfmContext := &flowcore.TrcFlowMachineContext{
	// 	Env:                       driverConfig.CoreConfig.Env,
	// 	GetAdditionalFlowsByState: flowopts.BuildOptions.GetAdditionalFlowsByState,
	// 	FlowMap:                   map[flowcore.FlowNameType]*flowcore.TrcFlowContext{},
	// }
	// tfContext := &flowcore.TrcFlowContext{
	// 	GoMod:    mod,
	// 	Vault:    vault,
	// 	FlowLock: &sync.Mutex{},
	// }
	for {
		dfstat = <-pluginHandler.ttdi_receiver
		switch {
		case dfstat != nil:
			driverConfig.CoreConfig.Log.Printf("Received dataflow statistic: %s\n", dfstat.Name)
			tenantIndexPath, tenantDFSIdPath := coreopts.BuildOptions.GetDFSPathName()
			if len(tenantIndexPath) == 0 || len(tenantDFSIdPath) == 0 {
				driverConfig.CoreConfig.Log.Println("GetDFSPathName returned an empty index path value.")
				return
			}
			flowcore.DeliverStatistic(nil, nil, mod, dfstat, dfstat.Name, tenantIndexPath, tenantDFSIdPath, driverConfig.CoreConfig.Log, true)
			driverConfig.CoreConfig.Log.Printf("Delivered dataflow statistic: %s\n", dfstat.Name)
		case dfstat == nil:
			driverConfig.CoreConfig.Log.Println("Shutting down dataflow statistic receiver in kernel")
			return
		}
	}
}

func (pluginHandler *PluginHandler) PluginserviceStop(driverConfig *eUtils.DriverConfig) {
	pluginName := driverConfig.SubSectionValue
	if len(pluginName) == 0 {
		driverConfig.CoreConfig.Log.Printf("No plugin name provided to stop plugin service.\n")
		return
	}
	if PluginMods == nil || PluginMods[pluginName] == nil {
		driverConfig.CoreConfig.Log.Printf("No plugin mod initialized or set for %s to stop plugin\n", pluginName)
		return
	}
	driverConfig.CoreConfig.Log.Printf("Sending stop message to plugin: %s\n", pluginName)
	pluginHandler.sender <- core.PLUGIN_EVENT_STOP
	driverConfig.CoreConfig.Log.Printf("Stop message successfully sent to plugin: %s\n", pluginName)
	PluginMods[pluginName] = nil
	pluginHandler.IsRunning = false
}

func LoadPluginPath(driverConfig *eUtils.DriverConfig, pluginToolConfig map[string]interface{}) string {
	var deployroot string
	var service string
	if s, ok := pluginToolConfig["trcplugin"].(string); ok {
		driverConfig.CoreConfig.Log.Printf("Loading plugin path for service: %s\n", s)
		service = s
	} else {
		fmt.Println("Unable to stop plugin service.")
		driverConfig.CoreConfig.Log.Println("Unable to stop plugin service.")
		return ""
	}
	if d, ok := pluginToolConfig["trcdeployroot"].(string); ok {
		driverConfig.CoreConfig.Log.Printf("Loading plugin deploy root for service: %s\n", d)
		deployroot = d
	} else {
		fmt.Println("Unable to stop plugin service.")
		driverConfig.CoreConfig.Log.Println("Unable to stop plugin service.")
		return ""
	}
	pluginPath := fmt.Sprintf("%s/%s%s", deployroot, service, ".so")
	return pluginPath
}

func LoadPluginMod(driverConfig *eUtils.DriverConfig, pluginPath string) {
	pluginM, err := plugin.Open(pluginPath)
	if err != nil {
		fmt.Printf("Unable to open plugin module for service: %s\n", pluginPath)
		driverConfig.CoreConfig.Log.Printf("Unable to open plugin module for service: %s\n", pluginPath)
		return
	}
	pluginName := driverConfig.SubSectionValue
	if len(pluginName) > 0 {
		driverConfig.CoreConfig.Log.Printf("Successfully opened plugin module for %s\n", pluginName)
		PluginMods[pluginName] = pluginM
	} else {
		fmt.Println("Unable to load plugin module because missing plugin name")
		driverConfig.CoreConfig.Log.Println("Unable to load plugin module because missing plugin name")
		return
	}
}

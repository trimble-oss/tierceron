package hive

import (
	"errors"
	"fmt"
	"log"
	"plugin"
	"reflect"
	"strings"

	"github.com/trimble-oss/tierceron-core/core"
	vcutils "github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase/utils"
	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

var PluginMods map[string]*plugin.Plugin = map[string]*plugin.Plugin{}
var logger *log.Logger

type PluginHandler struct {
	IsRunning bool
	sender    chan int
	receiver  chan error
	Services  *[]string
}

func Init(pluginName string, properties *map[string]interface{}) {
	if PluginMods == nil || PluginMods[pluginName] == nil {
		return
	}
	symbol, err := PluginMods[pluginName].Lookup("Init")
	if err != nil {
		fmt.Println(err)
		logger.Printf("Unable to lookup plugin export: %s\n", err)
	}

	reflect.ValueOf(symbol).Call([]reflect.Value{reflect.ValueOf(properties)})
}

func (pluginHandler *PluginHandler) PluginserviceStart(driverConfig *eUtils.DriverConfig, pluginToolConfig map[string]interface{}) {
	pluginName := driverConfig.SubSectionValue
	if len(pluginName) == 0 {
		return
	}
	if PluginMods == nil || PluginMods[pluginName] == nil {
		return
	}
	if driverConfig.CoreConfig.Log != nil {
		if logger == nil {
			logger = driverConfig.CoreConfig.Log
		}
	} else {
		fmt.Println("No logger passed in to plugin service")
		return //or set log to fmt?
	}
	var service string
	if s, ok := driverConfig.DeploymentConfig["trcplugin"].(string); ok {
		service = s
	} else {
		fmt.Println("Unable to process plugin service.")
		driverConfig.CoreConfig.Log.Println("Unable to process plugin service.")
		return
	}
	pluginConfig := make(map[string]interface{})
	pluginConfig["vaddress"] = driverConfig.CoreConfig.VaultAddress
	pluginConfig["token"] = driverConfig.CoreConfig.Token
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
			pluginHandler.sender = make(chan int)
			pluginHandler.receiver = make(chan error)
			chan_map := make(map[string]interface{})
			chan_map[core.PLUGIN_CHANNEL_EVENT_IN] = pluginHandler.sender
			chan_map[core.PLUGIN_CHANNEL_EVENT_OUT] = pluginHandler.receiver
			serviceConfig[core.PLUGIN_EVENT_CHANNELS_MAP_KEY] = chan_map
			serviceConfig["log"] = driverConfig.CoreConfig.Log
			Init(pluginName, &serviceConfig)
			pluginHandler.sender <- core.PLUGIN_EVENT_START
			go pluginHandler.handle_errors(driverConfig)
			pluginHandler.IsRunning = true
		}
	}
}

func (pluginHandler *PluginHandler) handle_errors(driverConfig *eUtils.DriverConfig) {
	for {
		result := <-pluginHandler.receiver
		switch {
		case result != nil:
			fmt.Println(result)
			eUtils.LogErrorObject(&driverConfig.CoreConfig, result, false)
			return
		}
	}
}

func (pluginHandler *PluginHandler) PluginserviceStop(driverConfig *eUtils.DriverConfig) {
	pluginName := driverConfig.SubSectionValue
	if len(pluginName) == 0 {
		return
	}
	if PluginMods == nil || PluginMods[pluginName] == nil {
		return
	}
	PluginMods[pluginName] = nil

	pluginHandler.sender <- core.PLUGIN_EVENT_STOP
	pluginHandler.IsRunning = false
}

func LoadPluginPath(driverConfig *eUtils.DriverConfig) string {
	var service string
	if s, ok := driverConfig.DeploymentConfig["trcplugin"].(string); ok {
		service = s
	} else {
		fmt.Println("Unable to stop plugin service.")
		driverConfig.CoreConfig.Log.Println("Unable to stop plugin service.")
		return ""
	}
	pluginPath := "./plugins/" + service + ".so"
	return pluginPath
}

func LoadPluginMod(driverConfig *eUtils.DriverConfig, pluginPath string) {
	pluginM, err := plugin.Open(pluginPath)
	if err != nil {
		fmt.Printf("Unable to open plugin module for service: %s\n", pluginPath)
		return
	}
	pluginName := driverConfig.SubSectionValue
	if len(pluginName) > 0 {
		PluginMods[pluginName] = pluginM
	} else {
		fmt.Printf("Missing pluginName for LoadPlugin: %s\n", pluginName)
		return
	}
}

package hive

import (
	"fmt"
	"log"
	"plugin"
	"reflect"
	"strings"

	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

var pluginMod *plugin.Plugin
var logger *log.Logger

type PluginHandler struct {
	IsRunning bool
	shutdown  chan *bool
	Services  *[]string
}

func (pluginHandler *PluginHandler) Start() {
	symbol, err := pluginMod.Lookup("Start")
	if err != nil {
		fmt.Println(err)
		logger.Printf("Unable to lookup plugin export: %s\n", err)
	}
	go reflect.ValueOf(symbol).Call(nil)
	go func(plugin *PluginHandler) {
		shutdownMsg := true
		plugin.shutdown <- &shutdownMsg
	}(pluginHandler)
}

func Stop() {
	symbol, err := pluginMod.Lookup("Stop")
	if err != nil {
		fmt.Println(err)
		logger.Printf("Unable to lookup plugin export: %s\n", err)
	}
	go reflect.ValueOf(symbol).Call(nil)
	pluginMod = nil
}

func Init(properties *map[string]interface{}) {
	symbol, err := pluginMod.Lookup("Init")
	if err != nil {
		fmt.Println(err)
		logger.Printf("Unable to lookup plugin export: %s\n", err)
	}

	reflect.ValueOf(symbol).Call([]reflect.Value{reflect.ValueOf(properties)})
}

func (pluginHandler *PluginHandler) PluginserviceStart(driverConfig *eUtils.DriverConfig, pluginToolConfig map[string]interface{}) {
	if pluginMod == nil {
		return
	}
	if logger == nil && driverConfig.CoreConfig.Log != nil {
		logger = driverConfig.CoreConfig.Log
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
			var configPath string
			symbol, err := pluginMod.Lookup("Config")
			if err != nil {
				driverConfig.CoreConfig.Log.Printf("Unable to access config for %s\n", service)
				driverConfig.CoreConfig.Log.Printf("Returned with %v\n", err)
				fmt.Printf("Unable to access config for %s\n", service)
				fmt.Printf("Returned with %v\n", err)
				return
			}
			var checkPathType *string
			if reflect.TypeOf(checkPathType) != reflect.TypeOf(symbol) {
				fmt.Printf("Wrong type returned for config path for %s\n", service)
				driverConfig.CoreConfig.Log.Printf("Wrong type returned for config path for %s\n", service)
				return
			} else {
				configPath = *symbol.(*string)
			}
			serviceConfig, ok := properties.GetConfigValues(projServ[1], configPath)
			if !ok {
				fmt.Printf("Unable to access configuration data for %s\n", service)
				driverConfig.CoreConfig.Log.Printf("Unable to access configuration data for %s\n", service)
				return
			}
			serviceConfig["log"] = driverConfig.CoreConfig.Log
			Init(&serviceConfig)
			pluginHandler.Start()
		}
	}
	pluginHandler.IsRunning = true
	pluginHandler.shutdown = make(chan *bool)
}

func (pluginHandler *PluginHandler) PluginserviceStop(driverConfig *eUtils.DriverConfig) {
	if pluginMod == nil {
		return
	}
	go func(plugin *PluginHandler) {
		isDone := <-plugin.shutdown
		if *isDone {
			Stop()
		}
	}(pluginHandler)
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
		fmt.Println("Unable to open plugin module for service.")
		return
	}
	pluginMod = pluginM
}

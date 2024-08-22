package hive

import (
	"fmt"
	"log"
	"plugin"
	"reflect"
	"strings"

	"github.com/trimble-oss/tierceron/atrium/vestibulum/pluginutil"
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

func GetConfigPath() {
	//return service's config path
}

func Start() {
	symbol, err := pluginMod.Lookup("Start")
	if err != nil {
		fmt.Println(err)
		logger.Printf("Unable to lookup plugin export: %s\n", err)
	}
	reflect.ValueOf(symbol).Call(nil)
}

func Stop() {
	symbol, err := pluginMod.Lookup("Stop")
	if err != nil {
		fmt.Println(err)
		logger.Printf("Unable to lookup plugin export: %s\n", err)
	}
	reflect.ValueOf(symbol).Call(nil)
}

func Init(properties *map[string]interface{}) {
	symbol, err := pluginMod.Lookup("Init")
	if err != nil {
		fmt.Println(err)
		logger.Printf("Unable to lookup plugin export: %s\n", err)
	}

	reflect.ValueOf(symbol).Call([]reflect.Value{reflect.ValueOf(properties)})
}

func (pluginHandler *PluginHandler) PluginserviceStart(driverConfig *eUtils.DriverConfig) {
	if logger == nil {
		logger = driverConfig.CoreConfig.Log
	}

	for _, service := range *pluginHandler.Services {
		//TODO: Verify service...
		pluginConfig := make(map[string]interface{})
		pluginConfig["vaddress"] = driverConfig.CoreConfig.VaultAddress
		pluginConfig["token"] = driverConfig.CoreConfig.Token
		pluginConfig["env"] = driverConfig.CoreConfig.EnvBasis

		_, mod, vault, err := eUtils.InitVaultModForPlugin(pluginConfig, driverConfig.CoreConfig.Log)
		if err != nil {
			fmt.Printf("Problem initializing mod: %s\n", err)
			driverConfig.CoreConfig.Log.Printf("Problem initializing mod: %s\n", err)
			continue
		}
		if vault != nil {
			defer vault.Close()
		}
		pluginConfig["pluginName"] = service
		pluginCertifyMap, plcErr := pluginutil.GetPluginCertifyMap(mod, pluginConfig)
		if plcErr != nil {
			fmt.Printf("Unable to read certification data for %s\n", service)
			driverConfig.CoreConfig.Log.Printf("Unable to read certification data for %s\n", service)
			driverConfig.CoreConfig.Log.Println(plcErr)
			continue
		}
		if _, ok := pluginCertifyMap["trcprojectservice"].(string); ok {
			projServ := strings.Split(pluginCertifyMap["trcprojectservice"].(string), "/")
			if len(projServ) != 2 {
				fmt.Printf("Improper formatting of project/service for %s\n", service)
				driverConfig.CoreConfig.Log.Printf("Improper formatting of project/service for %s\n", service)
				continue
			}
			properties, err := trcvutils.NewProperties(&driverConfig.CoreConfig, vault, mod, mod.Env, projServ[0], projServ[1])
			if err != nil && !strings.Contains(err.Error(), "no data paths found when initing CDS") {
				fmt.Println("Couldn't create properties for regioned certify:" + err.Error())
				continue
			}
			//TODO: figure out how to make local_config/application appear generally...
			serviceConfig, ok := properties.GetConfigValues(projServ[1], "/local_config/application")
			if !ok {
				fmt.Printf("Unable to access configuration data for %s\n", service)
				driverConfig.CoreConfig.Log.Printf("Unable to access configuration data for %s\n", service)
				continue
			}
			serviceConfig["log"] = driverConfig.CoreConfig.Log
			pluginPath := "./plugins/" + service + ".so"
			pluginM, err := plugin.Open(pluginPath)
			if err != nil {
				fmt.Printf("Unable to open plugin module for service: %s\n", service)
				return
			}
			pluginMod = pluginM
			Init(&serviceConfig)
			Start()
		}
	}
	pluginHandler.IsRunning = true
	pluginHandler.shutdown = make(chan *bool)
	go func(plugin *PluginHandler) {
		shutdownMsg := true
		plugin.shutdown <- &shutdownMsg
	}(pluginHandler)
}

func (pluginHandler *PluginHandler) PluginserviceStop(driverConfig *eUtils.DriverConfig) {
	if !pluginHandler.IsRunning {
		fmt.Println("plugin service has already been stopped")
		for _, service := range *pluginHandler.Services {
			pluginPath := "./plugins/" + service + ".so"
			pluginM, err := plugin.Open(pluginPath)
			if err != nil {
				fmt.Printf("Unable to open plugin module for service: %s\n", service)
				return
			}
			pluginMod = pluginM
			Stop()
		}
		return
	}
	go func(plugin *PluginHandler) {
		isDone := <-plugin.shutdown
		if *isDone {
			fmt.Println("Exit and stop server")
		}
	}(pluginHandler)

	pluginHandler.IsRunning = false
}

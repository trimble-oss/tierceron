package hive

import (
	"fmt"
	"log"
	"os"
	"plugin"

	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

var pluginMod *plugin.Plugin
var logger *log.Logger

type PluginHandler struct {
	IsRunning bool
	shutdown  chan *bool
	Services  *[]string
}

type PluginService interface {
	Start()
	Stop()
	Init(properties *map[string]interface{})
}

func Start() {
	symbol, err := pluginMod.Lookup("Start")
	if err != nil {
		fmt.Println(err)
		logger.Printf("Unable to lookup plugin export: %s\n", err)
	}
	var pluginServ PluginService
	pluginServ, ok := symbol.(PluginService)
	if !ok {
		fmt.Println("Unexpected type from module symbol")
		logger.Println("Unexpected type from module symbol")
		os.Exit(-1)
	}
	pluginServ.Start()
}

func Stop() {
	symbol, err := pluginMod.Lookup("Stop")
	if err != nil {
		fmt.Println(err)
		logger.Printf("Unable to lookup plugin export: %s\n", err)
	}
	var pluginServ PluginService
	pluginServ, ok := symbol.(PluginService)
	if !ok {
		fmt.Println("Unexpected type from module symbol")
		logger.Println("Unexpected type from module symbol")
		os.Exit(-1)
	}
	pluginServ.Stop()
}

func Init(properties *map[string]interface{}) {
	symbol, err := pluginMod.Lookup("Init")
	if err != nil {
		fmt.Println(err)
		logger.Printf("Unable to lookup plugin export: %s\n", err)
	}
	var pluginServ PluginService
	pluginServ, ok := symbol.(PluginService)
	if !ok {
		fmt.Println("Unexpected type from module symbol")
		logger.Println("Unexpected type from module symbol")
		os.Exit(-1)
	}
	pluginServ.Init(properties)
}

func (pluginHandler *PluginHandler) PluginserviceStart(driverConfig *eUtils.DriverConfig) {
	if logger == nil {
		logger = driverConfig.CoreConfig.Log
	}
	for _, service := range *pluginHandler.Services {
		pluginPath := "./plugins/" + service + ".so"
		pluginM, err := plugin.Open(pluginPath)
		if err != nil {
			fmt.Printf("Unable to open plugin module for service: %s\n", service)
			os.Exit(-1)
		}
		pluginMod = pluginM
		//TODO: Load properties of service and call init
		Start()
	}
	pluginHandler.IsRunning = true
	pluginHandler.shutdown = make(chan *bool)
	go func(plugin *PluginHandler) {
		shutdownMsg := true
		plugin.shutdown <- &shutdownMsg
	}(pluginHandler)
}

func (pluginHandler *PluginHandler) PluginserviceStop() {
	if !pluginHandler.IsRunning {
		fmt.Println("plugin service has already been stopped")
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

package main

import (
	"github.com/trimble-oss/tierceron/pkg/core/util/hive"
	hcore "github.com/trimble-oss/tierceron/pkg/core/util/hive/plugins/trcshcmd/hcore"
)

// Register callbacks with the hive package to avoid import cycles
func init() {
	hive.RegisterPluginCallbacks("trcshcmd", hcore.Init, hcore.Start)
}

func GetConfigPaths(pluginName string) []string {
	return hcore.GetConfigPaths(pluginName)
}

func Init(pluginName string, properties *map[string]any) {
	hcore.Init(pluginName, properties)
}

func Start() {
	hcore.Start()
}

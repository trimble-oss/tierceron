package main

import (
	hcore "github.com/trimble-oss/tierceron/pkg/core/util/hive/plugins/trcshcmd/hcore"
)

func GetConfigPaths(pluginName string) []string {
	return hcore.GetConfigPaths(pluginName)
}

func Init(pluginName string, properties *map[string]any) {
	hcore.Init(pluginName, properties)
}

func Start() {
	hcore.Start()
}

package main

import "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trctalkctl/hcore"

func GetConfigPaths(pluginName string) []string {
	return hcore.GetConfigPaths(pluginName)
}

func Init(pluginName string, properties *map[string]any) {
	hcore.Init(pluginName, properties)
}

func main() {}

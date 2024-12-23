//go:build hardwired
// +build hardwired

package pluginopts

import (
	hccore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trchealthcheck/hcore"
)

// IsPluginHardwired - Override to hardwire plugins into the kernel for debugging.
func IsPluginHardwired() bool {
	return true
}

// GetConfigPaths - Override GetConfigPaths calls.
func GetConfigPaths(pluginName string) []string {
	switch pluginName {
	case "trchelloworld":
		return hccore.GetConfigPaths(pluginName)
	}
	return []string{}
}

// Init - Override plugin Init calls
func Init(pluginName string, properties *map[string]interface{}) {
	switch pluginName {
	case "trchelloworld":
		hccore.Init(pluginName, properties)
	}
}

// GetPluginMessages - Override plugin messages
func GetPluginMessages(string) []string {
	return []string{}
}

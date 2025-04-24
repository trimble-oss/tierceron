//go:build hardwired
// +build hardwired

package pluginopts

import (
	fcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcfenestra/hcore"
	hccore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trchealthcheck/hcore"
	rcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcrosea/hcore"
	score "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcspiralis/hcore"
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
	case "fenestra":
		return fcore.GetConfigPaths(pluginName)
	case "spiralis":
		return score.GetConfigPaths(pluginName)
	case "rosea":
		return rcore.GetConfigPaths(pluginName)
	}
	return []string{}
}

// Init - Override plugin Init calls
func Init(pluginName string, properties *map[string]interface{}) {
	switch pluginName {
	case "trchelloworld":
		hccore.Init(pluginName, properties)
	case "fenestra":
		fcore.Init(pluginName, properties)
	case "spiralis":
		score.Init(pluginName, properties)
	case "rosea":
		rcore.Init(pluginName, properties)
	}
}

// GetPluginMessages - Override plugin messages
func GetPluginMessages(pluginName string) []string {
	switch pluginName {
	case "trchelloworld":
		//		return hccore.GetPluginMessages(pluginName)
	case "fenestra":
		return fcore.GetPluginMessages(pluginName)
	case "spiralis":
		return score.GetPluginMessages(pluginName)
	case "rosea":
		return rcore.GetPluginMessages(pluginName)
	}
	return []string{}
}

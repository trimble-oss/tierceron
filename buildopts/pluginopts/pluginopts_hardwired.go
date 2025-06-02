//go:build hardwired
// +build hardwired

package pluginopts

import (
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	trcdbcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcdb/hcore"
	fcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcfenestra/hcore"
	hccore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trchealthcheck/hcore"
	rcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcrosea/hcore"
	score "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcspiralis/hcore"
)

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
	case "trcdb":
		return trcdbcore.GetConfigPaths(pluginName)
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
	case "trcdb":
		trcdbcore.Init(pluginName, properties)
	}
}

func GetFlowMachineInitContext(pluginName string) *flowcore.FlowMachineInitContext {
	switch pluginName {
	case "trcdb":
		return trcdbcore.GetFlowMachineInitContext(pluginName)
	default:
		return nil
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
	case "trcdb":
		return trcdbcore.GetPluginMessages(pluginName)
	}
	return []string{}
}

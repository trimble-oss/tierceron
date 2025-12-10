//go:build hardwired
// +build hardwired

package pluginopts

import (
	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig"
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	trcdbcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcdb/hcore"
	fcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcfenestra/hcore"
	hccore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trchealthcheck/hcore"
	pcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcprocurator/tcore"
	rcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcrosea/hcore"
	score "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcspiralis/hcore"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
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
	case "procurator":
		return pcore.GetConfigPaths(pluginName)
	}
	return []string{}
}

// Init - Override plugin Init calls
func Init(pluginName string, properties *map[string]any) {
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
	case "procurator":
		pcore.Init(pluginName, properties)
	}
}

func GetFlowMachineInitContext(coreConfig *coreconfig.CoreConfig, pluginName string) *flowcore.FlowMachineInitContext {
	switch pluginName {
	case "trcdb":
		flowMachineInitContext := trcdbcore.GetFlowMachineInitContext(coreConfig, pluginName)
		if flowMachineInitContext != nil {
			if flowMachineInitContext.GetDatabaseName == nil {
				flowMachineInitContext.GetDatabaseName = coreopts.BuildOptions.GetDatabaseName
			}
		}
		return flowMachineInitContext
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
	case "procurator":
		return pcore.GetPluginMessages(pluginName)
	}
	return []string{}
}

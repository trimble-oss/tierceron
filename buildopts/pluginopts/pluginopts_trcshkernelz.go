//go:build trcshkernelz
// +build trcshkernelz

package pluginopts

import (
	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig"
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	trcdbcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcdb/hcore"
	trcshcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trctrcsh/core"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
)

// GetConfigPaths - Override GetConfigPaths calls.
func GetConfigPaths(pluginName string) []string {
	switch pluginName {
	case "trcsh":
		return trcshcore.GetConfigPaths(pluginName)
	}
	return []string{}
}

// Init - Override plugin Init calls
func Init(pluginName string, properties *map[string]any) {
	switch pluginName {
	case "trcsh":
		trcshcore.Init(pluginName, properties)
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
	case "trcsh":
		return trcshcore.GetPluginMessages(pluginName)
	}
	return []string{}
}

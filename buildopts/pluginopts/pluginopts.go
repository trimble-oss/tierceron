//go:build !hardwired
// +build !hardwired

package pluginopts

import flowcore "github.com/trimble-oss/tierceron-core/v2/flow"

// IsPluginHardwired - Override to hardwire plugins into the kernel for debugging.
func IsPluginHardwired() bool {
	return false
}

// GetConfigPaths - Override GetConfigPaths calls.
func GetConfigPaths(pluginName string) []string {
	return []string{}
}

// Init - Override plugin Init calls
func Init(pluginName string, properties *map[string]interface{}) {
}

// GetPluginMessages - Override plugin messages
func GetPluginMessages(pluginName string) []string {
	return []string{}
}

// GetFlowMachineInitContext - Override plugin GetFlowMachineInitContext
func GetFlowMachineInitContext(pluginName string) *flowcore.FlowMachineInitContext {
	return &flowcore.FlowMachineInitContext{}
}

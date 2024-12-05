//go:build !hardwired
// +build !hardwired

package pluginopts

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

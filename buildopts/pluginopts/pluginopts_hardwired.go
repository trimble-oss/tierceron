//go:build hardwired
// +build hardwired

package pluginopts

// IsPluginHardwired - Override to hardwire plugins into the kernel for debugging.
func IsPluginHardwired() bool {
	return true
}

// GetConfigPaths - Override GetConfigPaths calls.
func GetConfigPaths(pluginName string) []string {
	// switch pluginName {
	// case "helloworld":
	// 	return hccore.GetConfigPaths()
	// }
	return []string{}
}

// Init - Override plugin Init calls
func Init(pluginName string, properties *map[string]interface{}) {
	// switch pluginName {
	// case "helloworld":
	// 	hccore.Init(properties)
	// }
}

// GetPluginMessages - Override plugin messages
func GetPluginMessages(string) []string {
	return []string{}
}

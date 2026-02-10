//go:build trcshkernelz
// +build trcshkernelz

package hcore

func Init(pluginName string, properties *map[string]any) {
	initPlugin(pluginName, properties)
}

// Start sends the START command to the trcshcmd plugin
func Start() {
	startPlugin()
}

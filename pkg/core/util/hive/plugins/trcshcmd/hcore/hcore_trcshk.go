//go:build trcshkernel && !trcshkernelz
// +build trcshkernel,!trcshkernelz

package hcore

func Init(pluginName string, properties *map[string]any) {
	(*properties)["pluginRefused"] = true
	return
}

// Start sends the START command to the trcshcmd plugin
func Start() {
	// Disable in hive-kernel-k
	return
}

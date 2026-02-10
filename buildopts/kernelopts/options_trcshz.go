//go:build trcshkernelz
// +build trcshkernelz

package kernelopts

func IsKernel() bool {
	return true
}

func IsKernelZ() bool {
	return true
}

func Init(pluginName string, properties *map[string]any) {
	// Kernel does not have any compiled plugins by default
}

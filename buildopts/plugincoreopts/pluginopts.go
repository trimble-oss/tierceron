//go:build !hardwired
// +build !hardwired

package plugincoreopts

// IsPluginHardwired - Override to hardwire plugins into the kernel for debugging.
func IsPluginHardwired() bool {
	return false
}

//go:build memonly
// +build memonly

package memonly

// IsMemonly returns true if memonly build flag is specified, false otherwise.
func IsMemonly() bool {
	return true
}

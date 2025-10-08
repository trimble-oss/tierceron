//go:build localbuild
// +build localbuild

package localopts

// Whether this is a local build
func IsLocal() bool {
	return true
}

//go:build salty
// +build salty

package saltyopts

// Whether this is a local build
func GetSaltyGuardian() string {
	return SaltGuard
}

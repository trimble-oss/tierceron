//go:build !salty
// +build !salty

package saltyopts

// Returns the salty guardian
func GetSaltyGuardian() string {
	return "NotSalty"
}

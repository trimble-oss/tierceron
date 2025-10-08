//go:build salty
// +build salty

package saltyopts

// Whether this is a local build
func GetSaltyGuardian() string {
	data, err := SaltGuard.ReadFile("saltguard.txt")
	if err != nil || len(data) == 0 {
		return "NotSalty"
	}
	return string(data)

}

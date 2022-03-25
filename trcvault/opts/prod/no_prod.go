//go:build !prod
// +build !prod

package prod

func IsProd() bool {
	return false
}

//go:build !insecure
// +build !insecure

package insecure

func IsInsecure() bool {
	return false
}

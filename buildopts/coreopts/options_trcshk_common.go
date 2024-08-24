//go:build !kernel
// +build !kernel

package coreopts

func IsKernel() bool {
	return false
}

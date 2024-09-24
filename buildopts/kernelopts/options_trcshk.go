//go:build kernel
// +build kernel

package kernelopts

func IsKernel() bool {
	return true
}

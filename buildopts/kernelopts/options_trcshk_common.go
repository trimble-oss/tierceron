//go:build !trcshkernel
// +build !trcshkernel

package kernelopts

func IsKernel() bool {
	return false
}

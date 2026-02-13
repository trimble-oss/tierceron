//go:build !trcshkernel && !trcshkernelz
// +build !trcshkernel,!trcshkernelz

package kernelopts

func IsKernel() bool {
	return false
}

func IsKernelZ() bool {
	return false
}

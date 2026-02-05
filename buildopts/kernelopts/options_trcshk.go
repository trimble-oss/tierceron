//go:build trcshkernel && !trcshkernelz
// +build trcshkernel,!trcshkernelz

package kernelopts

func IsKernel() bool {
	return true
}

func IsKernelZ() bool {
	return false
}

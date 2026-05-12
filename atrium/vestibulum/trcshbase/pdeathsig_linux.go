//go:build linux

package trcshbase

import "golang.org/x/sys/unix"

// SetParentDeathSignal arranges for SIGTERM to be delivered to this process
// when its parent exits.  Linux only.
func SetParentDeathSignal() {
	unix.Prctl(unix.PR_SET_PDEATHSIG, uintptr(unix.SIGTERM), 0, 0, 0) //nolint:errcheck
}

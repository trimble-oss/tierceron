// +build dragonfly freebsd linux openbsd solaris

package mlock

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// Mlock - provides locking hook for OS's that support mlock
func Mlock() error {
	return unix.Mlockall(syscall.MCL_CURRENT | syscall.MCL_FUTURE)
}

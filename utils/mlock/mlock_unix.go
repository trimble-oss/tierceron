//go:build dragonfly || freebsd || linux || openbsd || solaris
// +build dragonfly freebsd linux openbsd solaris

package mlock

import (
	"log"
	"syscall"

	"golang.org/x/sys/unix"
)

// Mlock - provides locking hook for OS's that support mlock
func Mlock(logger *log.Logger) error {
	return unix.Mlockall(syscall.MCL_CURRENT | syscall.MCL_FUTURE)
}

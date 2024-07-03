//go:build dragonfly || freebsd || linux || openbsd || solaris
// +build dragonfly freebsd linux openbsd solaris

package mlock

import (
	"log"
	"reflect"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Mlock - provides locking hook for OS's that support mlock
func Mlock(logger *log.Logger) error {
	return unix.Mlockall(syscall.MCL_CURRENT | syscall.MCL_FUTURE)
}

var _zero uintptr

func Mlock2(logger *log.Logger, sensitive *string) error {
	var _p0 unsafe.Pointer
	var err error = nil
	sensitiveHeader := (*reflect.StringHeader)(unsafe.Pointer(sensitive))
	sensitiveLen := sensitiveHeader.Len
	if sensitiveLen > 0 {
		_p0 = unsafe.Pointer(sensitiveHeader.Data)
	} else {
		_p0 = unsafe.Pointer(&_zero)
	}
	_, _, e1 := unix.Syscall(unix.SYS_MLOCK2, uintptr(_p0), uintptr(sensitiveLen), 0)
	if e1 != 0 {
		switch e1 {
		case 0:
			err = nil
		case unix.EAGAIN:
			err = unix.EAGAIN
		case unix.EINVAL:
			err = unix.EINVAL
		case unix.ENOENT:
			err = unix.ENOENT
		}
	}
	return err
}

func MunlockAll(logger *log.Logger) error {
	return unix.Munlockall()
}

//go:build windows
// +build windows

package memprotectopts

import (
	"log"
	"reflect"
	"syscall"
	"unsafe"
)

const MEM_COMMIT = 0x1000
const MEM_RESERVE = 0x2000
const PAGE_READWRITE = 0x40

func MemProtectInit(logger *log.Logger) error {
	return nil
}

func MemUnprotectAll(logger *log.Logger) error {
	return nil
}

func MemProtect(logger *log.Logger, sensitive *string) error {
	virtualAllocProc := syscall.MustLoadDLL("kernel32.dll").MustFindProc("VirtualAlloc")
	var sensitiveLen int = (len(*sensitive))
	pMem, _, err := virtualAllocProc.Call(
		0,
		uintptr(sensitiveLen),
		MEM_COMMIT|MEM_RESERVE,
		PAGE_READWRITE,
	)

	if err != nil && err != syscall.Errno(0) {
		return err
	}
	s2 := make([]byte, sensitiveLen)
	byteSliceHeader := (*reflect.SliceHeader)(unsafe.Pointer(&s2))
	byteSliceHeader.Data = uintptr(unsafe.Pointer(pMem))
	copy(s2, []byte(*sensitive))
	*sensitive = string(s2)

	return nil
}

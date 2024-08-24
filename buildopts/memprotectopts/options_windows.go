//go:build windows
// +build windows

package memprotectopts

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const MEM_COMMIT = 0x1000
const MEM_RESERVE = 0x2000
const PAGE_READWRITE = 0x40

// MemProtectInit -- initialization of memory protection not required on Windows
func MemProtectInit(logger *log.Logger) error {
	return nil
}

// MemUnprotectAll -- not implemented on Windows
func MemUnprotectAll(logger *log.Logger) error {
	return nil
}

func SetChattr(f *os.File) error {
	return nil
}

func UnsetChattr(f *os.File) error {
	return nil
}

var kernel32 *syscall.DLL

func verifyKernelTrust() error {
	dllPath := filepath.Join("C:\\Windows\\System32", "kernel32.dll")
	evsignedfile16, err := windows.UTF16PtrFromString(dllPath)
	if err != nil {
		return err
	}
	data := &windows.WinTrustData{
		Size:             uint32(unsafe.Sizeof(windows.WinTrustData{})),
		UIChoice:         windows.WTD_UI_NONE,
		RevocationChecks: windows.WTD_REVOKE_WHOLECHAIN,
		UnionChoice:      windows.WTD_CHOICE_FILE,
		StateAction:      windows.WTD_STATEACTION_VERIFY,
		FileOrCatalogOrBlobOrSgnrOrCert: unsafe.Pointer(&windows.WinTrustFileInfo{
			Size:     uint32(unsafe.Sizeof(windows.WinTrustFileInfo{})),
			FilePath: evsignedfile16,
		}),
	}
	verifyErr := windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, data)
	if verifyErr != nil {
		return verifyErr
	}
	data.StateAction = windows.WTD_STATEACTION_CLOSE
	return windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, data)
}

// MemProtect protects sensitive memory
func MemProtect(logger *log.Logger, sensitive *string) error {
	if kernel32 == nil {
		verifyErr := verifyKernelTrust()
		if verifyErr != nil {
			return verifyErr
		}
		kernel32 = syscall.MustLoadDLL("C:\\Windows\\System32\\kernel32.dll")
		verifyErr = verifyKernelTrust()
		if verifyErr != nil {
			kernel32 = nil
			return verifyErr
		}
		fmt.Println("Trusting the kernel.")
	}
	virtualAllocProc := kernel32.MustFindProc("VirtualAlloc")
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

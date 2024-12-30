package trcshzig

/*
#cgo LDFLAGS: -lrt
#include <sys/mman.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <unistd.h>
#include <stdlib.h>

// Declare memfd_create
int memfd_create(const char *name, int flags);
*/
import "C"
import (
	"fmt"
	"log"
	"syscall"
	"unsafe"
)

// Define MFD_CLOEXEC for Go
const MFD_CLOEXEC = 1 << 0
const MFD_ALLOW_SEALING = C.int(0x0002)

const F_ADD_SEALS = 1032
const F_SEAL_WRITE = 0x2
const F_SEAL_SEAL = 0x4

func createMemfd(filename string) (C.int, error) {
	// Create an anonymous memory-backed file with the close-on-exec flag
	fd := C.memfd_create(C.CString(filename), MFD_ALLOW_SEALING)
	if fd < 0 {
		return -1, fmt.Errorf("failed to create memfd")
	}
	return fd, nil
}

func createMemfdSyscall() (uintptr, error) {
	namePtr := unsafe.Pointer(&[]byte("my_shared_memory")[0])

	// Create an anonymous memory-backed file with the close-on-exec flag
	fd, _, errno := syscall.Syscall(279, uintptr(unsafe.Pointer(namePtr)), uintptr(MFD_CLOEXEC), uintptr(MFD_ALLOW_SEALING))
	if errno > 0 {
		return uintptr(0), fmt.Errorf("failed to create memfd")
	}
	return fd, nil
}

func WriteMemFile(configService map[string]interface{}, filename string) {
	fd, err := createMemfd(filename)
	if err != nil {
		log.Fatalf("Error creating memfd: %v", err)
	}
	defer C.close(fd)

	if data, ok := configService[filename].([]byte); ok {
		dataLen := len(data)
		C.ftruncate(fd, C.off_t(dataLen))

		// Memory-map the file in the parent process
		mem := C.mmap(nil, C.size_t(dataLen), C.PROT_READ|C.PROT_WRITE, C.MAP_SHARED, fd, 0)
		if mem == C.MAP_FAILED {
			log.Fatal("Failed to mmap memfd")
		}
		defer C.munmap(mem, C.size_t(dataLen))

		// Write data into the memory file
		copy((*[4096]byte)(mem)[:len(data)], data)

		filePath := fmt.Sprintf("/proc/self/fd/%d", fd)

		// TODO: make symlink
	}

}

func exec(cmdMessage string) {
	cmd := exec.Command("java", cmdMessage)
	output, err := cmd.Output()
	if err != nil {
		log.Fatalf("Failed to execute Java process: %v", err)
	}
}

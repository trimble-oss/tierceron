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
	"archive/zip"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"

	"golang.org/x/sys/unix"
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

func ZigInit(configContext *tccore.ConfigContext) error {
	if err := unix.Unshare(unix.CLONE_NEWNS); err != nil {
		configContext.Log.Printf("Failed to unshare mount namespace: %v", err)
		return err
	}
	var statfs unix.Statfs_t

	err := unix.Statfs("/proc", &statfs)
	if err != nil {
		configContext.Log.Printf("Error getting /proc stat:", err)
		return err
	}
	if statfs.Type != unix.PROC_SUPER_MAGIC {
		if err := unix.Mount("proc", "/proc", "proc", unix.MS_PRIVATE|unix.MS_RDONLY, ""); err != nil {
			configContext.Log.Printf("Failed to mount /proc: %v", err)
			return err
		}
	} else {
		configContext.Log.Printf("Already mounted.  Insecure run...\n")
	}
	return nil
}

// Add this to the kernel when running....
// sudo setcap cap_sys_admin+ep /usr/bin/code
func WriteMemFile(configContext *tccore.ConfigContext, configService map[string]interface{}, filename string, pluginName string) error {
	if data, ok := configService[filename].([]byte); ok {
		dataLen := len(data)

		// Create a memory-backed file using memfd_create
		fd, err := unix.MemfdCreate(filename, 0)
		if err != nil {
			configContext.Log.Printf("Failed to create memfd: %v", err)
			return err
		}
		defer unix.Close(fd)

		if err := unix.Ftruncate(fd, int64(dataLen)); err != nil {
			configContext.Log.Printf("Failed to resize memfd: %v", err)
		}
		mem, err := unix.Mmap(fd, 0, dataLen, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
		if err != nil {
			configContext.Log.Printf("Failed to mmap: %v", err)
			return err
		}
		defer unix.Munmap(mem)

		// Write data into the memory file
		copy(mem, data)

		filePath := fmt.Sprintf("/proc/self/fd/%d", fd)
		filename = strings.Replace(filename, "./local_config/", "", 1)

		os.Symlink(filePath, fmt.Sprintf("./plugins/%s", filename))

		// TODO: Symlink new relic folder

		// TODO: How to specify jar file...

		zr, err := zip.OpenReader(fmt.Sprintf("./plugins/%s/", pluginName))
		if err != nil {
			return err
		}
		defer zr.Close()
		for _, file := range zr.File {
			if strings.Contains(file.Name, "startup-command") {
				r, err := file.Open()
				if err != nil {
					return err
				}
				var cmd bytes.Buffer
				_, err = io.Copy(bufio.NewWriter(&cmd), r)
				if err != nil {
					return err
				}
				execCmd(cmd.String())
			}
		}
	}

	return nil
}

func execCmd(cmdMessage string) {
	cmdTokens := strings.Fields(cmdMessage)
	if len(cmdTokens) <= 1 {
		return
	}
	cmdName := cmdTokens[0]
	cmdArgs := strings.Join(cmdTokens[1:], " ")
	cmd := exec.Command(cmdName, cmdArgs)
	output, err := cmd.Output()
	if err != nil {
		fmt.Println(err)
		log.Fatalf("Failed to execute Java process: %v", err)
	}
	fmt.Println(output)
}

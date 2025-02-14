package trcshzig

import (
	"archive/zip"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcsh/trcshio/trcshzigfs"
)

var zigPluginMap map[string]*trcshzigfs.TrcshZigRoot

func ZigInit(configContext *tccore.ConfigContext,
	pluginName string,
	pluginFiles *map[string]interface{}) error {
	if zigPluginMap == nil {
		zigPluginMap = make(map[string]*trcshzigfs.TrcshZigRoot)
	}
	zigPluginMap[pluginName] = trcshzigfs.NewTrcshZigRoot(pluginFiles)
	mntDir := "/usr/local/trcshk/plugins/zigfs"

	server, err := fs.Mount(mntDir, zigPluginMap[pluginName], &fs.Options{
		MountOptions: fuse.MountOptions{Debug: true},
	})
	if err != nil {
		configContext.Log.Printf("Error %v", err)
		return err
	}

	// Serve the file system, until unmounted by calling fusermount -u
	go server.Wait()

	return nil
}

// Add this to the kernel when running....
// sudo setcap cap_sys_admin+ep /usr/bin/code
func WriteMemFile(configContext *tccore.ConfigContext, configService map[string]interface{}, filename string, pluginName string) error {

	if _, ok := configService[filename].([]byte); ok {

		// TODO: Figure out pathing and symlink for child process

		// err = os.Symlink(filePath, symlinkPath)
		// if err != nil {
		// 	fmt.Println(err)
		// }
		// TODO: Symlink new relic folder

	}

	return nil
}

func ExecPlugin(pluginName string) error {
	// TODO: How to specify jar file... -- deploy path/root for plugin in certify
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
			// execCmd(zigPluginMap[pluginName], cmd.String())
		}
	}
	return nil
}

func execCmd(tzr *trcshzigfs.TrcshZigRoot, cmdMessage string) {
	cmdTokens := strings.Fields(cmdMessage)
	if len(cmdTokens) <= 1 {
		return
	}
	pid, err := syscall.ForkExec("/bin/true", nil, &syscall.ProcAttr{
		Env:   os.Environ(),
		Sys:   nil,
		Files: []uintptr{uintptr(syscall.Stdin), uintptr(syscall.Stdout), uintptr(syscall.Stderr)},
	})

	if err != nil {
		fmt.Println("Error forking process:", err)
		os.Exit(1)
	}

	if pid == 0 {
		err := syscall.Exec(cmdTokens[0], cmdTokens[1:], os.Environ())
		if err != nil {
			fmt.Println(err)
			// log.Fatalf("Failed to execute Java process: %v", err)
		}
	} else {
		tzr.SetPid(uint32(pid))

		// TODO: do we need to wait??? Idk...
		_, err = syscall.Wait4(pid, nil, 0, nil)
		if err != nil {
			fmt.Println("Error waiting for the child process:", err)
			os.Exit(1)
		}
	}

}

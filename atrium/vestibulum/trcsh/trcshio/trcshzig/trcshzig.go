package trcshzig

import (
	"archive/zip"
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	pluginFiles *map[string]interface{}) (string, error) {
	if zigPluginMap == nil {
		zigPluginMap = make(map[string]*trcshzigfs.TrcshZigRoot)
	}
	zigPluginMap[pluginName] = trcshzigfs.NewTrcshZigRoot(pluginFiles)
	mntDir := fmt.Sprintf("/usr/local/trcshk/plugins/zigfs/%s", pluginName)
	if _, err := os.Stat(mntDir); os.IsNotExist(err) {
		os.MkdirAll(mntDir, 0700)
	}
	exec.Command("fusermount", "-u", mntDir).Run()

	server, err := fs.Mount(mntDir, zigPluginMap[pluginName], &fs.Options{
		MountOptions: fuse.MountOptions{Debug: false},
	})
	if err != nil {
		configContext.Log.Printf("Error %v", err)
		return "", err
	}

	// Serve the file system, until unmounted by calling fusermount -u
	go server.Wait()

	return mntDir, nil
}

// Add this to the kernel when running....
// sudo setcap cap_sys_admin+ep /usr/bin/code
func LinkMemFile(configContext *tccore.ConfigContext, configService map[string]interface{}, filename string, pluginName string, mntDir string) error {

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
	zr, err := zip.OpenReader(fmt.Sprintf("/usr/local/trcshk/plugins/%s/", pluginName))
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
			execCmd(zigPluginMap[pluginName], cmd.String())
		}
	}
	return nil
}

func execCmd(tzr *trcshzigfs.TrcshZigRoot, cmdMessage string) error {
	cmdTokens := strings.Fields(cmdMessage)
	if len(cmdTokens) <= 1 {
		return errors.New("Not enough params")
	}
	pid, err := syscall.ForkExec("/bin/true", nil, &syscall.ProcAttr{
		Env:   os.Environ(),
		Sys:   nil,
		Files: []uintptr{uintptr(syscall.Stdin), uintptr(syscall.Stdout), uintptr(syscall.Stderr)},
	})

	if err != nil {
		fmt.Println("Error forking process:", err)
		return err
	}

	if pid == 0 {
		err := syscall.Exec(cmdTokens[0], cmdTokens[1:], os.Environ())
		if err != nil {
			fmt.Println(err)
			// log.Fatalf("Failed to execute Java process: %v", err)
		}
	} else {
		tzr.SetPid(uint32(pid))
	}
	return nil
}

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
	var mountDir string
	if certifyMap, ok := (*pluginFiles)["certify"].(map[string]interface{}); ok {
		if filePath, ok := certifyMap["trcdeployroot"].(string); ok {
			mountDir = strings.Replace(filePath, pluginName, "", 1)
		}
	}
	mntDir := fmt.Sprintf("%szigfs/%s", mountDir, pluginName)
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
		var filePath string
		if certifyMap, ok := configService["certify"].(map[string]interface{}); ok {
			if path, ok := certifyMap["trcdeployroot"].(string); ok {
				filePath = path
			}
		}
		filename = strings.Replace(filename, "./local_config/", "", 1)
		if strings.Contains(filename, "newrelic") {
			filename = fmt.Sprintf("newrelic/%s", filename)
		}
		filePath = fmt.Sprintf("%s/%s", filePath, filename)
		symlinkPath := fmt.Sprintf("%s/%s", mntDir, filename)
		if _, err := os.Lstat(filePath); err == nil {
			syscall.Unlink(filePath)
		}
		err := os.Symlink(symlinkPath, filePath)
		if err != nil {
			return err
		}
	}

	return nil
}

func ExecPlugin(pluginName string, properties map[string]interface{}, mntDir string) error {
	var filePath string
	if certifyMap, ok := properties["certify"].(map[string]interface{}); ok {
		if rootPath, ok := certifyMap["trcdeployroot"].(string); ok {
			if objectFile, ok := certifyMap["trccodebundle"].(string); ok {
				filePath = fmt.Sprintf("%s/%s", rootPath, objectFile)
			}
		}
	}
	zr, err := zip.OpenReader(filePath)
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
			execCmd(zigPluginMap[pluginName], cmd.String(), mntDir)
		}
	}
	return nil
}

func execCmd(tzr *trcshzigfs.TrcshZigRoot, cmdMessage string, mntDir string) error {
	cmdTokens := strings.Fields(cmdMessage)
	if len(cmdTokens) <= 1 {
		return errors.New("Not enough params")
	}
	tzr.SetPPid(uint32(os.Getpid()))
	pid, _, errno := syscall.RawSyscall(syscall.SYS_FORK, 0, 0, 0)
	if errno != 0 {
		fmt.Printf("Error forking process: %d\n", errno)
		return errors.New("fork failure")
	}

	if pid == 0 {
		err := syscall.Chdir(mntDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Exec failed: %v\n", err)
			return err
		}
		params := cmdTokens[1:]
		for i, param := range params {
			params[i] = strings.ReplaceAll(param, "\"", "")
		}

		err = syscall.Exec(cmdTokens[0], params, os.Environ())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Exec failed: %v\n", err)
			return err
		}
	}
	return nil
}

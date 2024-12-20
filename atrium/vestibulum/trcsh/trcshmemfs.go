package trcsh

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcsh/trcshio"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"

	"github.com/go-git/go-billy/v5"
)

type TrcshMemFs struct {
	BillyFs *billy.Filesystem
}

func (t *TrcshMemFs) WriteToMemFile(driverConfig *config.DriverConfig, byteData *[]byte, path string) {

	driverConfig.MemCacheLock.Lock()
	if _, err := (*t.BillyFs).Stat(path); errors.Is(err, os.ErrNotExist) {
		if strings.HasPrefix(path, "./") {
			path = strings.TrimLeft(path, "./")
		}
		memFile, err := driverConfig.MemFs.Create(path)
		if err != nil {
			eUtils.CheckError(driverConfig.CoreConfig, err, true)
		}
		memFile.Write(*byteData)
		memFile.Close()
		driverConfig.MemCacheLock.Unlock()
		eUtils.LogInfo(driverConfig.CoreConfig, "Wrote memfile:"+path)
	} else {
		driverConfig.MemCacheLock.Unlock()
		eUtils.LogInfo(driverConfig.CoreConfig, "Unexpected memfile exists:"+path)
		eUtils.CheckError(driverConfig.CoreConfig, err, true)
	}
}

func (t *TrcshMemFs) ReadDir(driverConfig *config.DriverConfig, path string) ([]os.FileInfo, error) {
	driverConfig.MemCacheLock.Lock()
	defer driverConfig.MemCacheLock.Unlock()
	return (*t.BillyFs).ReadDir(path)
}

func (t *TrcshMemFs) ClearCache(driverConfig *config.DriverConfig, path string) {
	driverConfig.MemCacheLock.Lock()
	defer driverConfig.MemCacheLock.Unlock()
	filestack := []string{path}
	var p string

summitatem:

	if len(filestack) == 0 {
		return
	}
	p, filestack = filestack[len(filestack)-1], filestack[:len(filestack)-1]

	if fileset, err := (*t.BillyFs).ReadDir(p); err == nil {
		for _, file := range fileset {
			if file.IsDir() {
				filestack = append(filestack, p)
				filestack = append(filestack, fmt.Sprintf("%s/%s", p, file.Name()))
				goto summitatem
			} else {
				driverConfig.MemFs.Remove(fmt.Sprintf("%s/%s", p, file.Name()))
			}
		}
	}
	driverConfig.MemFs.Remove(p)

	goto summitatem
}

func (t *TrcshMemFs) Create(filename string) (trcshio.TrcshReadWriteCloser, error) {
	return (*t.BillyFs).Create(filename)
}

func (t *TrcshMemFs) Open(filename string) (trcshio.TrcshReadWriteCloser, error) {
	return (*t.BillyFs).Open(filename)
}

func (t *TrcshMemFs) Stat(filename string) (os.FileInfo, error) {
	return (*t.BillyFs).Stat(filename)
}

func (t *TrcshMemFs) Remove(filename string) error {
	return (*t.BillyFs).Remove(filename)
}

func (t *TrcshMemFs) Lstat(filename string) (os.FileInfo, error) {
	return (*t.BillyFs).Lstat(filename)
}

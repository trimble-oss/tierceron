package trcsh

import (
	"errors"
	"fmt"
	"os"
	"strings"

	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"

	"github.com/go-git/go-billy/v5"
)

type TrcshMemFs struct {
	BillyFs billy.Filesystem
}

func (t *TrcshMemFs) WriteToMemFile(driverConfig *config.DriverConfig, byteData *[]byte, path string) {

	configMemFs := driverConfig.MemFs.(*TrcshMemFs)

	driverConfig.MemCacheLock.Lock()
	if _, err := configMemFs.BillyFs.Stat(path); errors.Is(err, os.ErrNotExist) {
		if strings.HasPrefix(path, "./") {
			path = strings.TrimLeft(path, "./")
		}
		memFile, err := configMemFs.BillyFs.Create(path)
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
	configMemFs := driverConfig.MemFs.(*TrcshMemFs)

	driverConfig.MemCacheLock.Lock()
	defer driverConfig.MemCacheLock.Unlock()
	return configMemFs.BillyFs.ReadDir(path)
}

func (t *TrcshMemFs) ClearCache(driverConfig *config.DriverConfig, path string) {
	configMemFs := driverConfig.MemFs.(*TrcshMemFs)

	driverConfig.MemCacheLock.Lock()
	defer driverConfig.MemCacheLock.Unlock()
	filestack := []string{path}
	var p string

summitatem:

	if len(filestack) == 0 {
		return
	}
	p, filestack = filestack[len(filestack)-1], filestack[:len(filestack)-1]

	if fileset, err := configMemFs.BillyFs.ReadDir(p); err == nil {
		for _, file := range fileset {
			if file.IsDir() {
				filestack = append(filestack, p)
				filestack = append(filestack, fmt.Sprintf("%s/%s", p, file.Name()))
				goto summitatem
			} else {
				configMemFs.BillyFs.Remove(fmt.Sprintf("%s/%s", p, file.Name()))
			}
		}
	}
	configMemFs.BillyFs.Remove(p)

	goto summitatem
}

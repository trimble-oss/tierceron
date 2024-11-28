package trcsh

import (
	"errors"
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

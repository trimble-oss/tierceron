package trcsh

import (
	"errors"
	"os"
	"strings"
	"sync"

	eUtils "github.com/trimble-oss/tierceron/pkg/utils"

	"github.com/go-git/go-billy/v5"
)

type TrcshMemFs struct {
	BillyFs billy.Filesystem
}

func (t *TrcshMemFs) WriteToMemFile(driverConfig *eUtils.DriverConfig, memCacheLocal *sync.Mutex, byteData *[]byte, path string) {

	configMemFs := driverConfig.MemFs.(*TrcshMemFs)

	memCacheLocal.Lock()
	if _, err := configMemFs.BillyFs.Stat(path); errors.Is(err, os.ErrNotExist) {
		if strings.HasPrefix(path, "./") {
			path = strings.TrimLeft(path, "./")
		}
		memFile, err := configMemFs.BillyFs.Create(path)
		if err != nil {
			eUtils.CheckError(&driverConfig.CoreConfig, err, true)
		}
		memFile.Write(*byteData)
		memFile.Close()
		memCacheLocal.Unlock()
		eUtils.LogInfo(&driverConfig.CoreConfig, "Wrote memfile:"+path)
	} else {
		memCacheLocal.Unlock()
		eUtils.LogInfo(&driverConfig.CoreConfig, "Unexpected memfile exists:"+path)
		eUtils.CheckError(&driverConfig.CoreConfig, err, true)
	}
}

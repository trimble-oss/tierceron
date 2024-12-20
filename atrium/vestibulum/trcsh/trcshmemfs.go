package trcsh

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcsh/trcshio"
	"github.com/trimble-oss/tierceron/pkg/core"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
)

type TrcshMemFs struct {
	BillyFs      *billy.Filesystem
	MemCacheLock sync.Mutex
}

func NewTrcshMemFs() *TrcshMemFs {
	billyFs := memfs.New()
	return &TrcshMemFs{
		BillyFs: &billyFs,
	}
}

func (t *TrcshMemFs) WriteToMemFile(coreConfig *core.CoreConfig, byteData *[]byte, path string) {

	t.MemCacheLock.Lock()
	if _, err := (*t.BillyFs).Stat(path); errors.Is(err, os.ErrNotExist) {
		if strings.HasPrefix(path, "./") {
			path = strings.TrimLeft(path, "./")
		}
		memFile, err := t.Create(path)
		if err != nil {
			eUtils.CheckError(coreConfig, err, true)
		}
		memFile.Write(*byteData)
		memFile.Close()
		t.MemCacheLock.Unlock()
		eUtils.LogInfo(coreConfig, "Wrote memfile:"+path)
	} else {
		t.MemCacheLock.Unlock()
		eUtils.LogInfo(coreConfig, "Unexpected memfile exists:"+path)
		eUtils.CheckError(coreConfig, err, true)
	}
}

func (t *TrcshMemFs) ReadDir(path string) ([]os.FileInfo, error) {
	t.MemCacheLock.Lock()
	defer t.MemCacheLock.Unlock()
	return (*t.BillyFs).ReadDir(path)
}

func (t *TrcshMemFs) ClearCache(path string) {
	t.MemCacheLock.Lock()
	defer t.MemCacheLock.Unlock()
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
				(*t.BillyFs).Remove(fmt.Sprintf("%s/%s", p, file.Name()))
			}
		}
	}
	(*t.BillyFs).Remove(p)

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

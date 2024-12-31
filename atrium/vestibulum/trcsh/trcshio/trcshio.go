package trcshio

import (
	"io"
	"os"

	"github.com/trimble-oss/tierceron/pkg/core"
)

type TrcshReadWriteCloser interface {
	io.ReadWriteCloser
	Name() string
}

type MemoryFileSystem interface {
	Remove(string) error
	Lstat(filename string) (os.FileInfo, error)
	Create(string) (TrcshReadWriteCloser, error)
	Open(string) (TrcshReadWriteCloser, error)
	Stat(string) (os.FileInfo, error)
	WriteToMemFile(coreConfig *core.CoreConfig, byteData *[]byte, path string)
	ReadDir(path string) ([]os.FileInfo, error)
	ClearCache(path string)
	SerializeToMap(path string, configCache map[string]interface{})
}

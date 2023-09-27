//go:build darwin
// +build darwin

package memprotectopts

import (
	"log"

	"github.com/trimble-oss/tierceron/utils/mlock"
)

func MemProtectInit(logger *log.Logger) error {
	return mlock.Mlock(logger)
}

func MemUnprotectAll(logger *log.Logger) error {
	return mlock.MunlockAll(logger)
}

func MemProtect(logger *log.Logger, sensitive *string) error {
	return mlock.Mlock2(logger, sensitive)
}

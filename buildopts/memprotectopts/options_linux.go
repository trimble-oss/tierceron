//go:build linux
// +build linux

package memprotectopts

import (
	"log"

	"github.com/trimble-oss/tierceron/pkg/utils/mlock"
)

// MemProtectInit initializes memory protection
func MemProtectInit(logger *log.Logger) error {
	return mlock.Mlock(logger)
}

// MemUnprotectAll unprotects all memory
func MemUnprotectAll(logger *log.Logger) error {
	return mlock.MunlockAll(logger)
}

// MemProtect protects sensitive memory
func MemProtect(logger *log.Logger, sensitive *string) error {
	return mlock.Mlock2(logger, sensitive)
}

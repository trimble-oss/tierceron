//go:build darwin
// +build darwin

package memprotectopts

import (
	"log"

	"github.com/trimble-oss/tierceron/pkg/utils/mlock"
	"golang.org/x/sys/unix"
)

// Not a lot of effort has been put into this Darwin implementation
// for memory protection.  Som parts may be incomplete or incorrect.
func MemProtectInit(logger *log.Logger) error {
	mlock.Mlock(logger)
	return nil
}

func MemUnprotectAll(logger *log.Logger) error {
	return unix.Munlockall()
}

func MemProtect(logger *log.Logger, sensitive *string) error {
	// TODO: is this correct?
	return mlock.Mlock(logger)
}

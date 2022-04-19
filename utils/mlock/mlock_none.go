//go:build android || darwin || nacl || netbsd || plan9 || windows
// +build android darwin nacl netbsd plan9 windows

package mlock

import (
        "log"
	"os"
)

// Mlock - provides locking hook for OS's that don't support mlock
func Mlock(logger *log.Logger) error {
	logger.Println("Mlock not supported.")
	os.Exit(1)
	return nil
}

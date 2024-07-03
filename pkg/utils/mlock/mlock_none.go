//go:build android || darwin || nacl || netbsd || plan9 || windows
// +build android darwin nacl netbsd plan9 windows

package mlock

import (
	"fmt"
	"log"
	"os"
)

// Mlock - provides locking hook for OS's that don't support mlock
func Mlock(logger *log.Logger) error {
	if logger != nil {
		logger.Println("Mlock not supported.")
	} else {
		fmt.Println("Mlock not supported.")
	}
	os.Exit(1)
	return nil
}

func Mlock2(logger *log.Logger, sensitive *string) error {
	if logger != nil {
		logger.Println("Mlock2 not supported.")
	} else {
		fmt.Println("Mlock2 not supported.")
	}
	os.Exit(1)
	return nil
}

func MunlockAll(logger *log.Logger) error {
	if logger != nil {
		logger.Println("MunlockAll not supported.")
	} else {
		fmt.Println("MunlockAll not supported.")
	}
	os.Exit(1)
	return nil
}

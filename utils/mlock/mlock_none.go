// +build android darwin nacl netbsd plan9 windows

package mlock

import (
	"fmt"
	"os"
)

// Mlock - provides locking hook for OS's that don't support mlock
func Mlock() error {
	fmt.Println("Mlock not supported.")
	os.Exit(1)
}

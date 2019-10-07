// +build android darwin nacl netbsd plan9 windows

package mlock

// Mlock - provides locking hook for OS's that don't support mlock
func Mlock() error {
	return nil
}

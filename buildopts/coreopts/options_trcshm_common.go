//go:build !trcshm
// +build !trcshm

package coreopts

func TapInit() {
}

func IsMessenger() bool {
	return false
}

func GetMessengerConfigPath() string {
	return "super-secrets/Restricted/TrcshAgent/config"
}

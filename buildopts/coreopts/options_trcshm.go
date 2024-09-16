//go:build trcshm
// +build trcshm

package coreopts

func TapInit() {
	cap.TapInit("/tmp/trcshm/")
}

// Pathing for the messenger
func IsMessenger() bool {
	return true
}

func GetMessengerConfigPath() string {
	return "super-secrets/Restricted/Trcshm/config"
}

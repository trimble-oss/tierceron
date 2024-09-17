//go:build trcshm
// +build trcshm

package coreopts

import (
	"github.com/trimble-oss/tierceron-hat/cap/tap"
)

func TapInit() {
	tap.TapInit("/tmp/trcshm/")
}

// Pathing for the messenger
func IsMessenger() bool {
	return true
}

func GetMessengerConfigPath() string {
	return "super-secrets/Restricted/Trcshm/config"
}

func GetTrcshBinPath() string {
	return "/home/azuredeploy/bin/trcshq"
}

func GetTrcshConfigPath() string {
	return "super-secrets/Index/TrcVault/trcplugin/trcshq/Certify"
}

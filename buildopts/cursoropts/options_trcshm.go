//go:build trcshm
// +build trcshm

package cursoropts

import (
	"github.com/trimble-oss/tierceron-hat/cap/tap"
)

func TapInit() {
	tap.TapInit("/tmp/trcshm/")
}

// Pathing for the cursor (runner)
func IsCursor() bool {
	return true
}

func GetCursorConfigPath() string {
	return "super-secrets/Restricted/Trcshm/config"
}

func GetTrcshBinPath() string {
	return "/home/azuredeploy/bin/trcshq"
}

func GetTrcshConfigPath() string {
	return "super-secrets/Index/TrcVault/trcplugin/trcshq/Certify"
}

//go:build trcshm && !trcshc
// +build trcshm,!trcshc

package cursoropts

import (
	"github.com/trimble-oss/tierceron-hat/cap/tap"
)

func TapInit() {
	tap.TapInit(GetCapPath())
}

func GetCapPath() string {
	return "/tmp/trcshm/"
}

func GetPluginName() string {
	return "trcshm"
}

func GetLogPath() string {
	return "/var/log/trcshm.log"
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

func GetCursorFields() map[string]string {
	return map[string]string{
		"configrole": "Read only role for specified environment.",
		"vaddress":   "Vault Url for plugin reference purposes.",
	}
}

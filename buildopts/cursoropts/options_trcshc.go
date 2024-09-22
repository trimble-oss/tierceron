//go:build trcshc && !trcshm
// +build trcshc,!trcshm

package cursoropts

import (
	"github.com/trimble-oss/tierceron-hat/cap/tap"
)

func TapInit() {
	tap.TapInit(GetCapPath())
}

func GetCapPath() string {
	return "/tmp/trccarrier/"
}

func GetPluginName() string {
	return "trcshc"
}

func GetLogPath() string {
	return "/var/log/trcshc.log"
}

func GetCursorConfigPath() string {
	return "super-secrets/Restricted/TrcshAgent/config"
}

func GetTrcshBinPath() string {
	return "/home/azuredeploy/bin/trcsh"
}

func GetTrcshConfigPath() string {
	return "super-secrets/Index/TrcVault/trcplugin/trcsh/Certify"
}

func GetCursorFields() map[string]string {
	return map[string]string{
		"pubrole":    "Pub role for specified environment.",
		"configrole": "Read only role for specified environment.",
		"kubeconfig": "kube config for specified environment.",
		"token":      "Token used for specified environment.",
		"vaddress":   "Vault Url for plugin reference purposes.",
		"caddress":   "Vault Url for plugin certification purposes.",
		"ctoken":     "Token for plugin certification purposes.",
		"plugin":     "Optional plugin name.",
	}
}

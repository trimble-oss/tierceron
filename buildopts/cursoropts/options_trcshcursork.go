//go:build trcshcursork && !trcshcursoraw
// +build trcshcursork,!trcshcursoraw

package cursoropts

import (
	"github.com/trimble-oss/tierceron-hat/cap/tap"
)

func TapInit() {
	tap.TapInit(GetCapPath())
}

func GetCapPath() string {
	return "/tmp/trcshk/"
}

func GetPluginName() string {
	return "trcshk"
}

func GetLogPath() string {
	return "/var/log/trcshcursork.log"
}

func GetCursorConfigPath() string {
	return "super-secrets/Restricted/Trcshm/config"
}

func GetTrusts() map[string][]string {
	return map[string][]string{
		"trcsh-cursor-k": []string{"trcshqk", "/home/azuredeploy/bin/trcshqk", "azuredeploy"},
	}
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

//go:build trcshcurator && !trcshcursoraw && !trcshcursork
// +build trcshcurator,!trcshcursoraw,!trcshcursork

package cursoropts

import (
	"github.com/trimble-oss/tierceron-hat/cap/tap"
)


func TapInit() {
	tap.TapInit(GetCapPath())
}

func GetCapPath() string {
	return "/tmp/trccurator/"
}

func GetPluginName() string {
	return "trcsh-curator"
}

func GetLogPath() string {
	return "/var/log/trcshcurator.log"
}

func GetCursorConfigPath() string {
	return "super-secrets/Restricted/TrcshCurator/config"
}

func GetTrusts() map[string][]string {
	return map[string][]string{
		"trcsh-cursor-aw": []string{"trcshaw", "/etc/opt/vault/plugins/trcsh-cursor-aw", "root"}, // original
		"trcsh-cursor-k":  []string{"trcshk", "/etc/opt/vault/plugins/trcsh-cursor-k", "root"},
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

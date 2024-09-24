//go:build trcshcursoraw && !trcshcursork
// +build trcshcursoraw,!trcshcursork

package cursoropts

import (
	"github.com/trimble-oss/tierceron-hat/cap/tap"
)

func GetCuratorConfig(pluginEnvConfig map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"env":            "dev",
		"exitOnFailure":  false,
		"regions":        []string{"west"},
		"pluginNameList": []string{""},
		"templatePath":   []string{"trc_templates/TrcVault/Certify/config.yml.tmpl"},
		"logNamespace":   "trcshcursoraw",
	}
}

func TapInit() {
	tap.TapInit(GetCapPath())
}

func GetCapPath() string {
	return "/tmp/trcsh-curator/"
}

func GetPluginName() string {
	return "trcsh-curator"
}

func GetLogPath() string {
	return "/var/log/trcshcursoraw.log"
}

func GetCursorConfigPath() string {
	return "super-secrets/Restricted/TrcshAgent/config"
}

func GetTrusts() map[string][]string {
	// TODO: when we retire carrier and switch to curator, move to trcsh instead of trcsh-curator
	return map[string][]string{
		"trcsh-cursor-aw": []string{"trcsh-curator", "/home/azuredeploy/bin/trcsh-curator", "azuredeploy"},
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

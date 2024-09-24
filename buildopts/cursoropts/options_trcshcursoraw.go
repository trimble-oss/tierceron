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
	return "/tmp/trcshqaw/"
}

func GetPluginName() string {
	return "trcshqaw"
}

func GetLogPath() string {
	return "/var/log/trcshcursoraw.log"
}

func GetCursorConfigPath() string {
	return "super-secrets/Restricted/TrcshAgent/config"
}

func GetTrusts() map[string][]string {
	// TODO: when we retire carrier and switch to curator, move to trcsh instead of trcshqaw
	return map[string][]string{
		"trcsh-cursor-aw": []string{"trcshqaw", "/home/azuredeploy/bin/trcshqaw", "azuredeploy"},
	}
}

func GetCursorFields() map[string]string {
	return map[string]string{
		"configrole": "Read only role for specified environment.",
		"kubeconfig": "kube config for specified environment.",
		"vaddress":   "Vault Url for plugin reference purposes.",
	}
}

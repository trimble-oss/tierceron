//go:build trcshcurator && !trcshcursoraw && !trcshcursork
// +build trcshcurator,!trcshcursoraw,!trcshcursork

package cursoropts

import (
	"github.com/trimble-oss/tierceron-hat/cap/tap"
)

// Utilized by trcsh curator to indicate the following map attributes:
//
//		exitOnFailure - if true, the plugin will exit on failure
//		regions - a list of regions to be supported by the carrier
//		pluginNameList - a list of plugins to be supported by the carrier
//		               the carrier is responsible for keeping the indicated plugins
//		               up to date and deployed with certified code...
//	          example values: trcsh, trc-vault-plugin
//
//		templatePath - a list of template paths (presently 1 template) to the certification
//		               template utilized by plugins.  This template references the published template
//		               originating from the source:
//		                  installation/trcdb/trc_templates/TrcVault/Certify/config.yml.tmpl
//		logNamespace - a log namespace to be used by the carrier in logging.
func GetCuratorConfig(pluginEnvConfig map[string]interface{}) map[string]interface{} {
	if pluginEnvConfig != nil {
		pluginEnvConfig["exitOnFailure"] = false
		pluginEnvConfig["regions"] = []string{"west"}
		pluginEnvConfig["pluginNameList"] = []string{"trc-vault-plugin", "trcsh-cursor-aw", "trcsh-cursor-ak"}
		pluginEnvConfig["templatePath"] = []string{"trc_templates/TrcVault/Certify/config.yml.tmpl"}
		pluginEnvConfig["logNamespace"] = "trcshcurator"
		return pluginEnvConfig
	} else {
		return map[string]interface{}{
			"env":            "dev",
			"exitOnFailure":  false,
			"regions":        []string{"west"},
			"pluginNameList": []string{"trc-vault-plugin", "trcsh-cursor-aw", "trcsh-cursor-ak"},
			"templatePath":   []string{"trc_templates/TrcVault/Certify/config.yml.tmpl"},
			"logNamespace":   "trcshcurator",
		}
	}
}

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

//go:build trcshcurator && !trcshcursoraw && !trcshcursork && !trcshcursorz
// +build trcshcurator,!trcshcursoraw,!trcshcursork,!trcshcursorz

package coreopts

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
func InitPluginConfig(pluginEnvConfig map[string]any) map[string]any {
	if pluginEnvConfig != nil {
		pluginEnvConfig["exitOnFailure"] = false
		pluginEnvConfig["regions"] = []string{"west"}
		pluginEnvConfig["pluginNameList"] = []string{"trc-vault-plugin", "trcsh-cursor-aw", "trcsh-cursor-ak"}
		pluginEnvConfig["templatePath"] = []string{"trc_templates/TrcVault/Certify/config.yml.tmpl"}
		pluginEnvConfig["logNamespace"] = "trcshcurator"
		return pluginEnvConfig
	} else {
		return map[string]any{
			"env":            "dev",
			"exitOnFailure":  false,
			"regions":        []string{"west"},
			"pluginNameList": []string{"trc-vault-plugin", "trcsh-cursor-aw", "trcsh-cursor-ak"},
			"templatePath":   []string{"trc_templates/TrcVault/Certify/config.yml.tmpl"},
			"logNamespace":   "trcshcurator",
		}
	}
}

// IsKubeRunnable returns true if this build variant is allowed to run in Kubernetes/AKS
func IsKubeRunnable() bool {
	return false
}

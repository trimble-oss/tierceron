//go:build trcshcursoraw && !trcshcursork
// +build trcshcursoraw,!trcshcursork

package cursoropts

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
}

func GetCapPath() string {
	return "/tmp/trcshcaw/"
}

func GetCapCuratorPath() string {
	return "/tmp/trccurator/"
}

func GetPluginName() string {
	return "trcsh-cursor-aw"
}

func GetLogPath() string {
	return "/var/log/trcshcursoraw.log"
}

func GetCursorConfigPath() string {
	return "super-secrets/Restricted/TrcshCurosorAW/config"
}

func GetTrusts() map[string][]string {
	// TODO: when we retire carrier and switch to curator, move to trcsh instead of trcsh-curator
	return map[string][]string{
		"trcshqaw": []string{
			"trcshqaw",                       // Certify pluginName,
			"/home/azuredeploy/bin/trcshqaw", // agent plugin path.
			"azuredeploy",                    // Group ownership of agent plugin.
		},
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

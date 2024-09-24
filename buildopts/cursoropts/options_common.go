//go:build !trcshcurator && !trcshcursoraw && !trcshcursork
// +build !trcshcurator,!trcshcursoraw,!trcshcursork

package cursoropts

func GetCuratorConfig(pluginEnvConfig map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"env":            "dev",
		"exitOnFailure":  false,
		"regions":        []string{"west"},
		"pluginNameList": []string{"trc-vault-plugin", "trcsh-cursor-aw", "trcsh-cursor-ak"},
		"templatePath":   []string{"trc_templates/TrcVault/Certify/config.yml.tmpl"},
		"logNamespace":   "trcshcurator",
	}
}

func TapInit() {
}
func GetCapPath() string {
	return ""
}

func GetPluginName() string {
	return ""
}

func GetLogPath() string {
	return ""
}

func GetTrusts() map[string][]string {
	return map[string][]string{}
}

func GetCursorConfigPath() string {
	return ""
}

func GetCursorFields() map[string]string {
	return map[string]string{}
}

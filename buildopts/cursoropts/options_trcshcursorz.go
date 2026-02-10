//go:build trcshcursorz && !trcshcursoraw && !trcshcursork
// +build trcshcursorz,!trcshcursoraw,!trcshcursork

package cursoropts

func GetCuratorConfig(pluginEnvConfig map[string]any) map[string]any {
	return map[string]any{
		"env":            "dev",
		"exitOnFailure":  false,
		"regions":        []string{"west"},
		"pluginNameList": []string{""},
		"templatePath":   []string{"trc_templates/TrcVault/Certify/config.yml.tmpl"},
		"logNamespace":   "trcshcursorz",
	}
}

func TapInit() {
}

func GetCapPath() string {
	return "/tmp/trcshzaw/"
}

func GetCapCuratorPath() string {
	return "/tmp/trccurator/"
}

func GetPluginName(vaultPlugin bool) string {
	if vaultPlugin {
		return "trcsh-cursor-z"
	} else {
		return "trcshz"
	}
}

func GetLogPath() string {
	return "/var/log/trcshcursorz.log"
}

func GetCursorConfigPath() string {
	return "super-secrets/Restricted/TrcshCursorZ/config"
}

func GetTrusts() map[string][]string {
	return map[string][]string{}
}

func GetCursorFields() map[string]CursorFieldAttributes {
	return map[string]CursorFieldAttributes{
		"pubrole": {
			Description: "Pub role for specified environment.",
			KeepSecret:  true,
		},
		"configrole": {
			Description: "Read only role for specified environment.",
			KeepSecret:  true,
		},
		"vaddress": {
			Description: "Vault Url for plugin reference purposes.",
			KeepSecret:  false,
		},
		"caddress": {
			Description: "Vault Url for plugin certification purposes.",
			KeepSecret:  false,
		},
		"token": {
			Description: "The restricted plugin readonly token.",
			KeepSecret:  true,
		},
		"ctoken": {
			Description: "Token for plugin certification purposes.",
			KeepSecret:  true,
		},
		"plugin": {
			Description: "Optional plugin name.",
			KeepSecret:  false,
		},
	}
}

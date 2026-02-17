//go:build trcshcurator && !trcshcursoraw && !trcshcursork && !trcshcursorz
// +build trcshcurator,!trcshcursoraw,!trcshcursork,!trcshcursorz

package cursoropts

func TapInit() {
}

func GetCapPath() string {
	return "/tmp/trccurator/"
}

func GetCapCuratorPath() string {
	return "/tmp/trccurator/"
}

func GetPluginName(vaultPlugin bool) string {
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
		"trcsh-cursor-aw": {
			"trcsh-cursor-aw",                        // Certify pluginName,
			"/etc/opt/vault/plugins/trcsh-cursor-aw", // vault plugin path.
			"root",                                   // Group ownership of vault plugin.
		}, // original
		"trcsh-cursor-k": {
			"trcsh-cursor-k",                        // Certify pluginName,
			"/etc/opt/vault/plugins/trcsh-cursor-k", // vault plugin path.
			"root",                                  // Group ownership of vault plugin.
		},
		"trcsh-cursor-z": {
			"trcsh-cursor-z",                        // Certify pluginName,
			"/etc/opt/vault/plugins/trcsh-cursor-z", // vault plugin path.
			"root",                                  // Group ownership of vault plugin.
		},
	}
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
		"kubeconfig": {
			Description: "kube config for specified environment.",
			KeepSecret:  true,
		},
		"token": {
			Description: "Token used for specified environment.",
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

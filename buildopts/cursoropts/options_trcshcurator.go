//go:build trcshcurator && !trcshcursoraw && !trcshcursork
// +build trcshcurator,!trcshcursoraw,!trcshcursork

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
		"trcsh-cursor-aw": []string{
			"trcsh-cursor-aw",                        // Certify pluginName,
			"/etc/opt/vault/plugins/trcsh-cursor-aw", // vault plugin path.
			"root",                                   // Group ownership of vault plugin.
		}, // original
		"trcsh-cursor-k": []string{
			"trcsh-cursor-k",                        // Certify pluginName,
			"/etc/opt/vault/plugins/trcsh-cursor-k", // vault plugin path.
			"root",                                  // Group ownership of vault plugin.
		},
	}
}

func GetCursorFields() map[string]CursorFieldAttributes {
	return map[string]CursorFieldAttributes{
		"pubrole": CursorFieldAttributes{
			Description: "Pub role for specified environment.",
			KeepSecret:  true,
		},
		"configrole": CursorFieldAttributes{
			Description: "Read only role for specified environment.",
			KeepSecret:  true,
		},
		"kubeconfig": CursorFieldAttributes{
			Description: "kube config for specified environment.",
			KeepSecret:  true,
		},
		"token": CursorFieldAttributes{
			Description: "Token used for specified environment.",
			KeepSecret:  true,
		},
		"vaddress": CursorFieldAttributes{
			Description: "Vault Url for plugin reference purposes.",
			KeepSecret:  false,
		},
		"caddress": CursorFieldAttributes{
			Description: "Vault Url for plugin certification purposes.",
			KeepSecret:  false,
		},
		"ctoken": CursorFieldAttributes{
			Description: "Token for plugin certification purposes.",
			KeepSecret:  true,
		},
		"plugin": CursorFieldAttributes{
			Description: "Optional plugin name.",
			KeepSecret:  false,
		},
	}
}

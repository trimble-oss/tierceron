//go:build trcshcursoraw && !trcshcursork && !trcshcursorz
// +build trcshcursoraw,!trcshcursork,!trcshcursorz

package cursoropts

import "runtime"

func GetCuratorConfig(pluginEnvConfig map[string]any) map[string]any {
	return map[string]any{
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

func GetPluginName(vaultPlugin bool) string {
	if runtime.GOOS == "windows" {
		return "trcsh.exe"
	} else {
		if vaultPlugin {
			return "trcsh-cursor-aw"
		} else {
			return "trcshqaw"
		}
	}
}

func GetLogPath() string {
	return "/var/log/trcshcursoraw.log"
}

func GetCursorConfigPath() string {
	return "super-secrets/Restricted/TrcshCursorAW/config"
}

func GetTrusts() map[string][]string {
	// TODO: when we retire carrier and switch to curator, move to trcsh instead of trcsh-curator
	return map[string][]string{
		"trcshqaw": {
			"trcshqaw",                       // Certify pluginName,
			"/home/azuredeploy/bin/trcshqaw", // agent plugin path.
			"azuredeploy",                    // Group ownership of agent plugin.
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
			Description: "The restricted plugin readonly token.",
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

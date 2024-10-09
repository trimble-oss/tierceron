//go:build trcshcursoraw && !trcshcursork
// +build trcshcursoraw,!trcshcursork

package cursoropts

import "runtime"

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
		"trcshqaw": []string{
			"trcshqaw",                       // Certify pluginName,
			"/home/azuredeploy/bin/trcshqaw", // agent plugin path.
			"azuredeploy",                    // Group ownership of agent plugin.
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
			Description: "The restricted plugin readonly token.",
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

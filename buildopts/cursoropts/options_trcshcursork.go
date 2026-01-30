//go:build trcshcursork && !trcshcursoraw && !trcshcursorz
// +build trcshcursork,!trcshcursoraw,!trcshcursorz

package cursoropts

import "github.com/trimble-oss/tierceron/buildopts/kernelopts"

func GetCuratorConfig(pluginEnvConfig map[string]any) map[string]any {
	return map[string]any{
		"env":            "dev",
		"exitOnFailure":  false,
		"regions":        []string{"west"},
		"pluginNameList": []string{""},
		"templatePath":   []string{"trc_templates/TrcVault/Certify/config.yml.tmpl"},
		"logNamespace":   "trcshcursork",
	}
}

func TapInit() {
}

func GetCapPath() string {
	return "/tmp/trcshkaw/"
}

func GetCapCuratorPath() string {
	return "/tmp/trccurator/"
}

func GetPluginName(vaultPlugin bool) string {
	if vaultPlugin {
		return "trcsh-cursor-k"
	} else {
		if kernelopts.BuildOptions.IsKernel() {
			return "trcshk"
		} else {
			return "trcshqk"
		}
	}
}

func GetLogPath() string {
	return "/var/log/trcshcursork.log"
}

func GetCursorConfigPath() string {
	return "super-secrets/Restricted/TrcshCursorK/config"
}

func GetTrusts() map[string][]string {
	return map[string][]string{
		"trcshqk": {
			"trcshqk",                       // Certify pluginName,
			"/home/azuredeploy/bin/trcshqk", // agent plugin path.
			"azuredeploy",                   // Group ownership of agent plugin.
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

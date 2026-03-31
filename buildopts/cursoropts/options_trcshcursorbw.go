//go:build trcshcursorbw && !trcshcursoraw && !trcshcursork && !trcshcursorz

package cursoropts

import (
	"log"
	"runtime"
)

func GetCuratorConfig(pluginEnvConfig map[string]any) map[string]any {
	return map[string]any{
		"env":            "dev",
		"exitOnFailure":  false,
		"regions":        []string{"west"},
		"pluginNameList": []string{"trcsh.exe"},
		"templatePath":   []string{"trc_templates/TrcVault/Certify/config.yml.tmpl"},
		"logNamespace":   "trcshcursorbw",
	}
}

func TapInit(config map[string]any, logger *log.Logger, initCapAuthFunc func(map[string]any, *log.Logger) error) {
	// No-op for cursorbw
}

func GetCapPath() string {
	return "/tmp/trcshqbw/"
}

func GetCapCuratorPath() string {
	return "/tmp/trccurator/"
}

func GetPluginName(vaultPlugin bool) string {
	if runtime.GOOS == "windows" {
		return "trcshb.exe"
	} else {
		if vaultPlugin {
			return "trcsh-cursor-bw"
		} else {
			return "trcshqbw"
		}
	}
}

func GetLogPath() string {
	return "/var/log/trcshcursorbw.log"
}

func GetCursorConfigPath() string {
	return "super-secrets/Restricted/TrcshCursorBW/config"
}

func GetTrusts() map[string][]string {
	if runtime.GOOS == "windows" {
		return map[string][]string{
			"trcshb.exe": {
				"trcshb.exe",                       // Certify pluginName
				"/home/azuredeploy/bin/trcshb.exe", // agent plugin path.
				"azuredeploy",                      // Group ownership of agent plugin.
			},
		}
	}
	return map[string][]string{
		"trcshqbw": {
			"trcshqbw",                       // Certify pluginName,
			"/home/azuredeploy/bin/trcshqbw", // agent plugin path.
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

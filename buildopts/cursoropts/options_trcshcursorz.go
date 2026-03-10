//go:build trcshcursorz && !trcshcursoraw && !trcshcursork
// +build trcshcursorz,!trcshcursoraw,!trcshcursork

package cursoropts

import (
	"fmt"
	"log"

	"github.com/trimble-oss/tierceron-core/v2/prod"
)

func GetCuratorConfig(pluginEnvConfig map[string]any) map[string]any {
	return map[string]any{
		"env":            "dev",
		"exitOnFailure":  false,
		"regions":        []string{"west"},
		"pluginNameList": []string{},
		"templatePath":   []string{"trc_templates/TrcVault/Certify/config.yml.tmpl"},
		"logNamespace":   "trcshcursorz",
	}
}

func TapInit(config map[string]any, logger *log.Logger, initCapAuthFunc func(map[string]any, *log.Logger) error) {
	// No-op for cursorz
}

func GetCapPath() string {
	return ""
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
	prodSuffix := ""
	if prod.IsProd() {
		prodSuffix = "-prod"
	}

	return map[string][]string{
		fmt.Sprintf("trcsh-cursor-z%s", prodSuffix): {
			fmt.Sprintf("trcsh-cursor-z%s", prodSuffix),                        // Certify pluginName,
			fmt.Sprintf("/etc/opt/vault/plugins/trcsh-cursor-z%s", prodSuffix), // vault plugin path.
			"root", // Group ownership of vault plugin.
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

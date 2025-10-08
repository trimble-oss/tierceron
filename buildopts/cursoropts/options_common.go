//go:build !trcshcurator && !trcshcursoraw && !trcshcursork
// +build !trcshcurator,!trcshcursoraw,!trcshcursork

package cursoropts

func TapInit() {
}

func GetCapPath() string {
	return ""
}

func GetCapCuratorPath() string {
	return ""
}

func GetPluginName(vaultPlugin bool) string {
	return ""
}

func GetLogPath() string {
	return ""
}

func GetTrusts() map[string][]string {
	return map[string][]string{
		"trusted_plugin_key": []string{
			"trusted_plugin_certify_name", // Certify pluginName,
			"trusted_plugin_path",         // vault plugin path.
			"trusted_plugin_group",        // Group ownership of vault plugin.
		},
	}
}

func GetCursorConfigPath() string {
	return ""
}

func GetCursorFields() map[string]CursorFieldAttributes {
	return map[string]CursorFieldAttributes{}
}

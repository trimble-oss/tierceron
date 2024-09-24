//go:build !trcshcurator && !trcshcursoraw && !trcshcursork
// +build !trcshcurator,!trcshcursoraw,!trcshcursork

package cursoropts

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

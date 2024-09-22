//go:build !trcshc && !trcshm
// +build !trcshc,!trcshm

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

func GetCursorConfigPath() string {
	return ""
}

func GetTrcshBinPath() string {
	return ""
}

func GetTrcshConfigPath() string {
	return ""
}

func GetCursorFields() map[string]string {
	return map[string]string{}
}

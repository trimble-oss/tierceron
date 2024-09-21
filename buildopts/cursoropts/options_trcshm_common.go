//go:build !trcshm
// +build !trcshm

package cursoropts

func TapInit() {
}

func IsCursor() bool {
	return false
}

func GetCursorConfigPath() string {
	return "super-secrets/Restricted/TrcshAgent/config"
}

func GetTrcshBinPath() string {
	return "/home/azuredeploy/bin/trcsh"
}

func GetTrcshConfigPath() string {
	return "super-secrets/Index/TrcVault/trcplugin/trcsh/Certify"
}

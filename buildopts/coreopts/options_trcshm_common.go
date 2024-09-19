//go:build !trcshm
// +build !trcshm

package coreopts

func TapInit() {
}

func IsMessenger() bool {
	return false
}

func GetMessengerConfigPath() string {
	return "super-secrets/Restricted/TrcshAgent/config"
}

func GetTrcshBinPath() string {
	return "/home/azuredeploy/bin/trcsh"
}

func GetTrcshConfigPath() string {
	return "super-secrets/Index/TrcVault/trcplugin/trcsh/Certify"
}

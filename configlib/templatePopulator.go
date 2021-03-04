package main

import (
	"C"

	"encoding/base64"
	"fmt"

	"Vault.Whoville/vaultconfig/utils"
	"Vault.Whoville/vaulthelper/kv"
)

//export ConfigTemplateLib
func ConfigTemplateLib(token string, address string, env string, templatePath string, configuredFilePath string, project string, service string) *C.char {
	fmt.Println("NCLib Version: " + "1.12")
	mod, err := kv.NewModifier(token, address, env, nil)
	mod.Env = env
	if err != nil {
		panic(err)
	}

	configuredTemplate, _ := utils.ConfigTemplate(mod, templatePath, configuredFilePath, true, project, service, false, true)

	mod.Close()

	return C.CString(configuredTemplate)
}

//export ConfigCertLib
func ConfigCertLib(token string, address string, env string, templatePath string, configuredFilePath string, project string, service string) *C.char {
	fmt.Println("NCLib Version: " + "1.12")
	mod, err := kv.NewModifier(token, address, env, nil)
	mod.Env = env
	if err != nil {
		panic(err)
	}

	_, configuredCert := utils.ConfigTemplate(mod, templatePath, configuredFilePath, true, project, service, true, true)

	mod.Close()

	certBase64 := base64.StdEncoding.EncodeToString([]byte(configuredCert[1]))

	return C.CString(certBase64)
}

func main() {}

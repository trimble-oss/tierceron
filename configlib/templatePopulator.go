package main

import (
	"C"

	"bitbucket.org/dexterchaney/whoville/vaultconfig/utils"
	"bitbucket.org/dexterchaney/whoville/vaulthelper/kv"
)

//export ConfigTemplateLib
func ConfigTemplateLib(token string, address string, env string, templatePath string, configuredFilePath string, project string, service string) *C.char {
	mod, err := kv.NewModifier(token, address, env, nil)
	mod.Env = env
	if err != nil {
		panic(err)
	}

	configuredTemplate, _ := utils.ConfigTemplate(mod, templatePath, configuredFilePath, true, project, service, false)

	return C.CString(configuredTemplate)
}
func main() {}

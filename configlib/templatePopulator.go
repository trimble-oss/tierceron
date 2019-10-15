package main

import "C"

import (
	"strings"

	"bitbucket.org/dexterchaney/whoville/vaultconfig/utils"
	"bitbucket.org/dexterchaney/whoville/vaulthelper/kv"
)

//export ConfigTemplateLib
func ConfigTemplateLib(token string, address string, env string, templatePath string, configuredFilePath string, secretMode bool, servicesWanted string) string {

	services := []string{}
	if servicesWanted != "" {
		services = strings.Split(servicesWanted, ",")
	}

	for _, service := range services {
		service = strings.TrimSpace(service)
	}

	mod, err := kv.NewModifier(token, address, env)
	mod.Env = env
	if err != nil {
		panic(err)
	}
	return utils.ConfigTemplate(mod, templatePath, configuredFilePath, secretMode, services...)
}
func main() {}

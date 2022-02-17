package utils

import (
	"log"
	"strings"
	"tierceron/vaulthelper/kv"
	helperkv "tierceron/vaulthelper/kv"
	sys "tierceron/vaulthelper/system"
)

// Helper to easiliy intialize a vault and a mod all at once.
func InitVaultMod(config *DriverConfig) (*helperkv.Modifier, *sys.Vault, error) {
	vault, err := sys.NewVault(config.Insecure, config.VaultAddress, config.Env, false, false, config.ExitOnFailure, config.Log)
	if err != nil {
		LogErrorObject(err, config.Log, false)
		return nil, nil, err
	}
	vault.SetToken(config.Token)

	mod, err := helperkv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, config.Regions, config.Log)
	if err != nil {
		LogErrorObject(err, config.Log, false)
		return nil, nil, err
	}
	mod.Env = config.Env
	mod.Version = "0"
	mod.VersionFilter = config.VersionFilter

	return mod, vault, nil
}

func GetAcceptedTemplatePaths(config *DriverConfig, modCheck *kv.Modifier, templatePaths []string) ([]string, error) {
	var acceptedTemplatePaths []string
	serviceMap := make(map[string]bool)

	if modCheck != nil {
		serviceInterface, err := modCheck.ListEnv("super-secrets/" + modCheck.Env)
		modCheck.Env = config.Env
		if err != nil {
			return nil, err
		}
		if serviceInterface == nil || serviceInterface.Data["keys"] == nil {
			return templatePaths, nil
		}

		serviceList := serviceInterface.Data["keys"]
		for _, data := range serviceList.([]interface{}) {
			serviceMap[data.(string)] = true
		}
	}
	for _, templatePath := range templatePaths {
		templatePathParts := strings.Split(templatePath, "/")
		service := templatePathParts[len(templatePathParts)-2]

		if _, ok := serviceMap[service]; ok {
			if config.SectionKey == "" || strings.Contains(templatePath, config.SectionKey) {
				acceptedTemplatePaths = append(acceptedTemplatePaths, templatePath)
			}
		}
	}

	if len(acceptedTemplatePaths) > 0 {
		templatePaths = acceptedTemplatePaths
	}

	return templatePaths, nil
}

// Helper to easiliy intialize a vault and a mod all at once.
func InitVaultModForPlugin(pluginConfig map[string]interface{}, logger *log.Logger) (*helperkv.Modifier, *sys.Vault, error) {
	exitOnFailure := false
	if _, ok := pluginConfig["exitOnFailure"]; ok {
		exitOnFailure = pluginConfig["exitOnFailure"].(bool)
	}
	config := DriverConfig{
		Insecure:       pluginConfig["insecure"].(bool),
		Token:          pluginConfig["token"].(string),
		VaultAddress:   pluginConfig["address"].(string),
		Env:            pluginConfig["env"].(string),
		Regions:        pluginConfig["regions"].([]string),
		SecretMode:     true, //  "Only override secret values in templates?"
		ServicesWanted: []string{},
		StartDir:       append([]string{}, ""),
		EndDir:         "",
		WantCerts:      false,
		GenAuth:        false,
		ExitOnFailure:  exitOnFailure,
		Log:            logger,
	}

	return InitVaultMod(&config)
}

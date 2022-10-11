package utils

import (
	"fmt"
	"log"
	"os"
	"strings"
	"tierceron/buildopts/coreopts"
	"tierceron/trcvault/opts/prod"
	helperkv "tierceron/vaulthelper/kv"
	sys "tierceron/vaulthelper/system"
)

// Helper to easiliy intialize a vault and a mod all at once.
func InitVaultMod(config *DriverConfig) (*DriverConfig, *helperkv.Modifier, *sys.Vault, error) {
	LogInfo(config, "InitVaultMod begins..")
	vault, err := sys.NewVault(config.Insecure, config.VaultAddress, config.Env, false, false, false, config.Log)
	if err != nil {
		LogInfo(config, "Failure to connect to vault..")
		LogErrorObject(config, err, false)
		return config, nil, nil, err
	}
	vault.SetToken(config.Token)

	mod, err := helperkv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, config.Regions, config.Log)
	if err != nil {
		LogErrorObject(config, err, false)
		return config, nil, nil, err
	}
	mod.Env = config.Env
	mod.Version = "0"
	mod.VersionFilter = config.VersionFilter
	LogInfo(config, "InitVaultMod complete..")

	return config, mod, vault, nil
}

func GetAcceptedTemplatePaths(config *DriverConfig, modCheck *helperkv.Modifier, templatePaths []string) ([]string, error) {
	var acceptedTemplatePaths []string
	serviceMap := make(map[string]bool)

	if strings.Contains(config.EnvRaw, "_") {
		config.EnvRaw = strings.Split(config.EnvRaw, "_")[0]
	}

	if modCheck != nil {
		envVersion := SplitEnv(config.Env)
		serviceInterface, err := modCheck.ListEnv("super-secrets/"+envVersion[0], config.Log)
		modCheck.Env = config.Env
		if err != nil {
			return nil, err
		}
		if serviceInterface == nil || serviceInterface.Data["keys"] == nil {
			return templatePaths, nil
		}

		serviceList := serviceInterface.Data["keys"]
		for _, data := range serviceList.([]interface{}) {
			if config.SectionName != "" {
				if strings.Contains(data.(string), config.SectionName) {
					serviceMap[data.(string)] = true
				}
			} else {
				serviceMap[data.(string)] = true
			}
		}

		for _, templatePath := range templatePaths {
			if len(config.ProjectSections) > 0 { //Filter by project
				for _, projectSection := range config.ProjectSections {
					if strings.Contains(templatePath, "/"+projectSection+"/") {
						listValues, err := modCheck.ListEnv("super-secrets/"+strings.Split(config.EnvRaw, ".")[0]+config.SectionKey+config.ProjectSections[0]+"/"+config.SectionName, config.Log)
						if err != nil || listValues == nil {
							listValues, err = modCheck.ListEnv("super-secrets/"+strings.Split(config.EnvRaw, ".")[0]+config.SectionKey+config.ProjectSections[0], config.Log)
							if listValues == nil {
								LogErrorObject(config, err, false)
								LogInfo(config, "Couldn't list services for project path")
								continue
							}
						}
						for _, valuesPath := range listValues.Data {
							for _, service := range valuesPath.([]interface{}) {
								serviceMap[service.(string)] = true
							}
						}
					}
				}
			}
		}
	}
	for _, templatePath := range templatePaths {
		templatePathRelativeParts := strings.Split(templatePath, coreopts.GetFolderPrefix()+"_templates/")
		templatePathParts := strings.Split(templatePathRelativeParts[1], "/")
		service := templatePathParts[1]

		if _, ok := serviceMap[service]; ok || templatePathParts[0] == "Common" {
			if config.SectionKey == "" || config.SectionKey == "/" {
				acceptedTemplatePaths = append(acceptedTemplatePaths, templatePath)
			} else {
				for _, sectionProject := range config.ProjectSections {
					if strings.Contains(templatePath, sectionProject) {
						acceptedTemplatePaths = append(acceptedTemplatePaths, templatePath)
					}
				}
			}
		} else {
			if config.SectionKey != "" && config.SectionKey != "/" {
				for _, sectionProject := range config.ProjectSections {
					if strings.Contains(templatePath, "/"+sectionProject+"/") {
						acceptedTemplatePaths = append(acceptedTemplatePaths, templatePath)
					}
				}
			}
		}
	}

	if len(acceptedTemplatePaths) > 0 {
		templatePaths = acceptedTemplatePaths
	}

	return templatePaths, nil
}

// Helper to easiliy intialize a vault and a mod all at once.
func InitVaultModForPlugin(pluginConfig map[string]interface{}, logger *log.Logger) (*DriverConfig, *helperkv.Modifier, *sys.Vault, error) {
	logPrefix := fmt.Sprintf("[trcplugin%s-%s]", pluginConfig["logNamespace"].(string), pluginConfig["env"].(string))
	var trcdbEnvLogger *log.Logger

	if logger.Prefix() != logPrefix {
		logFile := fmt.Sprintf("/var/log/trcplugin%s-%s.log", pluginConfig["logNamespace"].(string), pluginConfig["env"].(string))
		if !prod.IsProd() && coreopts.IsTestRunner() {
			logFile = fmt.Sprintf("trcplugin%s-%s.log", pluginConfig["logNamespace"].(string), pluginConfig["env"].(string))
		}
		f, logErr := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		trcdbEnvLogger = log.New(f, fmt.Sprintf("[trcplugin%s-%s]", pluginConfig["logNamespace"].(string), pluginConfig["env"].(string)), log.LstdFlags)
		CheckError(&DriverConfig{Insecure: true, Log: trcdbEnvLogger, ExitOnFailure: true}, logErr, true)
	} else {
		trcdbEnvLogger = logger
	}

	trcdbEnvLogger.Println("InitVaultModForPlugin begin..")
	exitOnFailure := false
	if _, ok := pluginConfig["exitOnFailure"]; ok {
		exitOnFailure = pluginConfig["exitOnFailure"].(bool)
	}

	trcdbEnvLogger.Println("InitVaultModForPlugin initialize DriverConfig.")

	config := DriverConfig{
		Insecure:       !exitOnFailure, // Plugin has exitOnFailure=false ...  always local, so this is ok...
		Token:          pluginConfig["token"].(string),
		VaultAddress:   pluginConfig["vaddress"].(string),
		Env:            pluginConfig["env"].(string),
		Regions:        pluginConfig["regions"].([]string),
		SecretMode:     true, //  "Only override secret values in templates?"
		ServicesWanted: []string{},
		StartDir:       append([]string{}, ""),
		EndDir:         "",
		WantCerts:      false,
		GenAuth:        false,
		ExitOnFailure:  exitOnFailure,
		Log:            trcdbEnvLogger,
	}
	trcdbEnvLogger.Println("InitVaultModForPlugin ends..")

	return InitVaultMod(&config)
}

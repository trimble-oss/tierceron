package utils

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/trcvault/opts/prod"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
	sys "github.com/trimble-oss/tierceron/vaulthelper/system"
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

	mod, err := helperkv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, config.Regions, false, config.Log)
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

var templateName string = coreopts.GetFolderPrefix() + "_templates"

func GetAcceptedTemplatePaths(config *DriverConfig, modCheck *helperkv.Modifier, templatePaths []string) ([]string, error) {
	var acceptedTemplatePaths []string

	if strings.Contains(config.EnvRaw, "_") {
		config.EnvRaw = strings.Split(config.EnvRaw, "_")[0]
	}
	var wantedTemplatePaths []string

	if len(config.DynamicPathFilter) > 0 {
		dynamicPathParts := strings.Split(config.DynamicPathFilter, "/")

		if len(dynamicPathParts) > 4 && dynamicPathParts[0] == "Restricted" || dynamicPathParts[0] == "Index" {
			projectFilter := "/" + dynamicPathParts[1] + "/"
			serviceFilter := "/" + dynamicPathParts[4] + "/"
			config.SectionName = serviceFilter

			// Now filter and grab the templates we want...
			for _, templateCandidate := range templatePaths {
				templateIndex := strings.Index(templateCandidate, templateName)
				projectIndex := strings.Index(templateCandidate, projectFilter)

				if projectIndex > templateIndex && strings.Index(templateCandidate, serviceFilter) > projectIndex {
					acceptedTemplatePaths = append(acceptedTemplatePaths, templateCandidate)
				}
			}
		}
	} else {
		// TODO: Deprecated...
		// 1-800-ROIT
		pathFilterBase := ""
		if config.SectionKey != "/Restricted/" {
			pathFilterBase = "/" + coreopts.GetFolderPrefix() + "_templates"
		}

		for _, projectSection := range config.ProjectSections {
			pathFilter := pathFilterBase + "/" + projectSection + "/"
			if len(config.ServiceFilter) > 0 {
				for _, serviceFilter := range config.ServiceFilter {
					endPathFilter := serviceFilter
					if config.SectionKey != "/Restricted/" {
						endPathFilter = endPathFilter + "/"
					}
					wantedTemplatePaths = append(wantedTemplatePaths, pathFilter+endPathFilter)
				}
			} else {
				wantedTemplatePaths = append(wantedTemplatePaths, pathFilter)
			}
		}

		// Now filter and grab the templates we want...
		for _, templateCandidate := range templatePaths {
			for _, wantedPath := range wantedTemplatePaths {
				if strings.Contains(templateCandidate, wantedPath) {
					acceptedTemplatePaths = append(acceptedTemplatePaths, templateCandidate)
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

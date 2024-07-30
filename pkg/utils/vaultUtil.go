package utils

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/pkg/core"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	sys "github.com/trimble-oss/tierceron/pkg/vaulthelper/system"
)

// Helper to easiliy intialize a vault and a mod all at once.
func InitVaultMod(driverConfig *DriverConfig) (*DriverConfig, *helperkv.Modifier, *sys.Vault, error) {
	LogInfo(&driverConfig.CoreConfig, "InitVaultMod begins..")
	if driverConfig == nil {
		LogInfo(&driverConfig.CoreConfig, "InitVaultMod failure.  driverConfig provided is nil")
		return driverConfig, nil, nil, errors.New("invalid nil driverConfig")
	}

	vault, err := sys.NewVault(driverConfig.CoreConfig.Insecure, driverConfig.CoreConfig.VaultAddress, driverConfig.CoreConfig.Env, false, false, false, driverConfig.CoreConfig.Log)
	if err != nil {
		LogInfo(&driverConfig.CoreConfig, "Failure to connect to vault..")
		LogErrorObject(&driverConfig.CoreConfig, err, false)
		return driverConfig, nil, nil, err
	}
	vault.SetToken(driverConfig.CoreConfig.Token)
	LogInfo(&driverConfig.CoreConfig, "InitVaultMod - Initializing Modifier")
	mod, err := helperkv.NewModifier(driverConfig.CoreConfig.Insecure, driverConfig.CoreConfig.Token, driverConfig.CoreConfig.VaultAddress, driverConfig.CoreConfig.Env, driverConfig.CoreConfig.Regions, false, driverConfig.CoreConfig.Log)
	if err != nil {
		LogErrorObject(&driverConfig.CoreConfig, err, false)
		return driverConfig, nil, nil, err
	}
	mod.Env = driverConfig.CoreConfig.Env
	mod.Version = "0"
	mod.VersionFilter = driverConfig.VersionFilter
	LogInfo(&driverConfig.CoreConfig, "InitVaultMod complete..")

	return driverConfig, mod, vault, nil
}

func GetAcceptedTemplatePaths(driverConfig *DriverConfig, modCheck *helperkv.Modifier, templatePaths []string) ([]string, error) {
	var acceptedTemplatePaths []string
	var templateName string = coreopts.BuildOptions.GetFolderPrefix(driverConfig.StartDir) + "_templates"

	if strings.Contains(driverConfig.CoreConfig.EnvBasis, "_") {
		driverConfig.CoreConfig.EnvBasis = strings.Split(driverConfig.CoreConfig.EnvBasis, "_")[0]
	}
	var wantedTemplatePaths []string

	if len(driverConfig.CoreConfig.DynamicPathFilter) > 0 {
		dynamicPathParts := strings.Split(driverConfig.CoreConfig.DynamicPathFilter, "/")

		if dynamicPathParts[0] == "Restricted" || dynamicPathParts[0] == "Index" || dynamicPathParts[0] == "PublicIndex" || dynamicPathParts[0] == "Protected" {
			projectFilter := "/" + dynamicPathParts[1] + "/"
			var serviceFilter string
			if len(dynamicPathParts) > 4 {
				serviceFilter = "/" + dynamicPathParts[4] + "/"
			} else if len(dynamicPathParts) < 4 && dynamicPathParts[0] == "Protected" {
				// Support shorter Protected paths.
				serviceFilter = "/" + dynamicPathParts[2]
			}
			driverConfig.SectionName = serviceFilter

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
		if driverConfig.SectionKey != "/Restricted/" {
			pathFilterBase = "/" + coreopts.BuildOptions.GetFolderPrefix(driverConfig.StartDir) + "_templates"
		}

		for _, projectSection := range driverConfig.ProjectSections {
			pathFilter := pathFilterBase + "/" + projectSection + "/"
			if len(driverConfig.ServiceFilter) > 0 {
				for _, serviceFilter := range driverConfig.ServiceFilter {
					endPathFilter := serviceFilter
					if driverConfig.SectionKey != "/Restricted/" {
						endPathFilter = endPathFilter + "/"
					}
					wantedTemplatePaths = append(wantedTemplatePaths, pathFilter+endPathFilter)
				}
			} else if len(driverConfig.SubPathFilter) > 0 {
				wantedTemplatePaths = driverConfig.SubPathFilter
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

var logMap sync.Map = sync.Map{}

// Helper to easiliy intialize a vault and a mod all at once.
func InitVaultModForPlugin(pluginConfig map[string]interface{}, logger *log.Logger) (*DriverConfig, *helperkv.Modifier, *sys.Vault, error) {
	logger.Println("InitVaultModForPlugin log setup: " + pluginConfig["env"].(string))
	var trcdbEnvLogger *log.Logger

	if _, nameSpaceOk := pluginConfig["logNamespace"]; nameSpaceOk {
		logPrefix := fmt.Sprintf("[trcplugin%s-%s]", pluginConfig["logNamespace"].(string), pluginConfig["env"].(string))

		if logger.Prefix() != logPrefix {
			logFile := fmt.Sprintf("/var/log/trcplugin%s-%s.log", pluginConfig["logNamespace"].(string), pluginConfig["env"].(string))
			if tLogger, logOk := logMap.Load(logFile); !logOk {
				logger.Printf("Checking log permissions for logfile: %s\n", logFile)

				f, logErr := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
				if logErr != nil {
					logFile = fmt.Sprintf("trcplugin%s-%s.log", pluginConfig["logNamespace"].(string), pluginConfig["env"].(string))
					f, logErr = os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
					if logErr != nil {
						logger.Println("Log permissions failure.  Will exit.")
					}
				}

				trcdbEnvLogger = log.New(f, fmt.Sprintf("[trcplugin%s-%s]", pluginConfig["logNamespace"].(string), pluginConfig["env"].(string)), log.LstdFlags)
				CheckError(&core.CoreConfig{ExitOnFailure: true, Log: trcdbEnvLogger}, logErr, true)
				logMap.Store(logFile, trcdbEnvLogger)
				logger.Println("InitVaultModForPlugin log setup complete")
			} else {
				logger.Printf("Utilizing existing logger for logfile: %s\n", logFile)
				trcdbEnvLogger = tLogger.(*log.Logger)
			}
		} else {
			trcdbEnvLogger = logger
		}
	} else {
		logger.Printf("Utilizing default logger invalid namespace\n")
		trcdbEnvLogger = logger
	}

	trcdbEnvLogger.Println("InitVaultModForPlugin begin..")
	exitOnFailure := false
	if _, ok := pluginConfig["exitOnFailure"]; ok {
		exitOnFailure = pluginConfig["exitOnFailure"].(bool)
	}

	trcdbEnvLogger.Println("InitVaultModForPlugin initialize DriverConfig.")

	var regions []string
	if _, regionsOk := pluginConfig["regions"]; regionsOk {
		regions = pluginConfig["regions"].([]string)
	}

	driverConfig := DriverConfig{
		CoreConfig: core.CoreConfig{
			WantCerts:     false,
			Insecure:      !exitOnFailure, // Plugin has exitOnFailure=false ...  always local, so this is ok...
			Token:         pluginConfig["token"].(string),
			VaultAddress:  pluginConfig["vaddress"].(string),
			Env:           pluginConfig["env"].(string),
			Regions:       regions,
			ExitOnFailure: exitOnFailure,
			Log:           trcdbEnvLogger,
		},
		SecretMode:     true, //  "Only override secret values in templates?"
		ServicesWanted: []string{},
		StartDir:       append([]string{}, ""),
		EndDir:         "",
		GenAuth:        false,
	}
	trcdbEnvLogger.Println("InitVaultModForPlugin ends..")

	return InitVaultMod(&driverConfig)
}

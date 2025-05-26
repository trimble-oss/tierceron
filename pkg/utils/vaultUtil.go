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
	"github.com/trimble-oss/tierceron/pkg/core/cache"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	sys "github.com/trimble-oss/tierceron/pkg/vaulthelper/system"
)

// Helper to easiliy intialize a vault and a mod all at once.
func InitVaultMod(driverConfig *config.DriverConfig) (*config.DriverConfig, *helperkv.Modifier, *sys.Vault, error) {
	if driverConfig == nil {
		fmt.Println("InitVaultMod failure.  driverConfig provided is nil")
		return driverConfig, nil, nil, errors.New("invalid nil driverConfig")
	}
	LogInfo(driverConfig.CoreConfig, "InitVaultMod begins..")

	vault, err := sys.NewVault(driverConfig.CoreConfig.Insecure, driverConfig.CoreConfig.TokenCache.VaultAddressPtr, driverConfig.CoreConfig.Env, false, false, false, driverConfig.CoreConfig.Log)
	if err != nil {
		LogInfo(driverConfig.CoreConfig, "Failure to connect to vault..")
		LogErrorObject(driverConfig.CoreConfig, err, false)
		return driverConfig, nil, nil, err
	}

	if RefLength(driverConfig.CoreConfig.CurrentTokenNamePtr) == 0 {
		return driverConfig, nil, nil, errors.New("missing required token name")
	}
	tokenName := *driverConfig.CoreConfig.CurrentTokenNamePtr
	tokenPtr := driverConfig.CoreConfig.TokenCache.GetToken(tokenName)
	if RefLength(tokenPtr) == 0 {
		return driverConfig, nil, nil, fmt.Errorf("token found nothing in token cache: %s", tokenName)
	}
	vault.SetToken(tokenPtr)
	LogInfo(driverConfig.CoreConfig, "InitVaultMod - Initializing Modifier")
	mod, err := helperkv.NewModifierFromCoreConfig(driverConfig.CoreConfig, tokenName, driverConfig.CoreConfig.Env, false)
	if err != nil {
		LogErrorObject(driverConfig.CoreConfig, err, false)
		return driverConfig, nil, nil, err
	}
	mod.Env = driverConfig.CoreConfig.Env
	mod.Version = "0"
	mod.VersionFilter = driverConfig.VersionFilter
	LogInfo(driverConfig.CoreConfig, "InitVaultMod complete..")

	return driverConfig, mod, vault, nil
}

func GetAcceptedTemplatePaths(driverConfig *config.DriverConfig, modCheck *helperkv.Modifier, templatePaths []string) ([]string, error) {
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

func InitPluginLogs(pluginConfig map[string]interface{}, logger *log.Logger) *log.Logger {
	logger.Println("InitPluginLogs log setup: " + pluginConfig["env"].(string))
	var trcdbEnvLogger *log.Logger

	if _, nameSpaceOk := pluginConfig["logNamespace"]; nameSpaceOk {
		logPrefix := fmt.Sprintf("[trcplugin-%s-%s]", pluginConfig["logNamespace"].(string), pluginConfig["env"].(string))

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

	return trcdbEnvLogger
}

// Helper to easiliy intialize a vault and a mod all at once.
func InitVaultModForPlugin(pluginConfig map[string]interface{}, tokenCache *cache.TokenCache, currentTokenName string, logger *log.Logger) (*config.DriverConfig, *helperkv.Modifier, *sys.Vault, error) {
	trcdbEnvLogger := InitPluginLogs(pluginConfig, logger)
	exitOnFailure := false

	trcdbEnvLogger.Println("InitVaultModForPlugin begin..")
	if _, ok := pluginConfig["exitOnFailure"]; ok {
		exitOnFailure = pluginConfig["exitOnFailure"].(bool)
	}
	trcdbEnvLogger.Println("InitVaultModForPlugin region init.")
	var regions []string
	if regionsSlice, regionsOk := pluginConfig["regions"].([]string); regionsOk {
		regions = regionsSlice
	}

	trcdbEnvLogger.Println("InitVaultModForPlugin initialize DriverConfig.")
	if tokenPtr, tokenOk := pluginConfig["tokenptr"].(*string); !tokenOk || RefLength(tokenPtr) < 5 {
		if tokenCache.GetToken(currentTokenName) == nil {
			trcdbEnvLogger.Println("Missing required token")
			return nil, nil, nil, errors.New("missing required token")
		}
	}
	if _, vaddressOk := pluginConfig["vaddress"].(string); !vaddressOk {
		trcdbEnvLogger.Println("Missing required vaddress")
		return nil, nil, nil, errors.New("missing required vaddress")
	}
	if _, envOk := pluginConfig["env"].(string); !envOk {
		trcdbEnvLogger.Println("Missing required env")
		return nil, nil, nil, errors.New("missing required env")
	}
	tokenCache.SetVaultAddress(RefMap(pluginConfig, "vaddress"))

	driverConfig := config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			WantCerts:           false,
			Insecure:            !exitOnFailure, // Plugin has exitOnFailure=false ...  always local, so this is ok...
			CurrentTokenNamePtr: &currentTokenName,
			TokenCache:          tokenCache,
			Env:                 pluginConfig["env"].(string),
			EnvBasis:            GetEnvBasis(pluginConfig["env"].(string)),
			Regions:             regions,
			ExitOnFailure:       exitOnFailure,
			Log:                 trcdbEnvLogger,
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

func InitDriverConfigForPlugin(pluginConfig map[string]interface{}, tokenCache *cache.TokenCache, currentTokenName string, logger *log.Logger) (*config.DriverConfig, error) {
	trcdbEnvLogger := InitPluginLogs(pluginConfig, logger)
	exitOnFailure := false

	trcdbEnvLogger.Println("InitVaultModForPlugin begin..")
	if _, ok := pluginConfig["exitOnFailure"]; ok {
		exitOnFailure = pluginConfig["exitOnFailure"].(bool)
	}
	trcdbEnvLogger.Println("InitVaultModForPlugin region init.")
	var regions []string
	if regionsSlice, regionsOk := pluginConfig["regions"].([]string); regionsOk {
		regions = regionsSlice
	}

	trcdbEnvLogger.Println("InitVaultModForPlugin initialize DriverConfig.")
	if tokenPtr, tokenOk := pluginConfig["tokenptr"].(*string); !tokenOk || RefLength(tokenPtr) < 5 {
		if tokenCache.GetToken(currentTokenName) == nil {
			trcdbEnvLogger.Println("Missing required token")
			return nil, errors.New("missing required token")
		}
	}
	if _, vaddressOk := pluginConfig["vaddress"].(string); !vaddressOk {
		trcdbEnvLogger.Println("Missing required vaddress")
		return nil, errors.New("missing required vaddress")
	}
	if _, envOk := pluginConfig["env"].(string); !envOk {
		trcdbEnvLogger.Println("Missing required env")
		return nil, errors.New("missing required env")
	}
	tokenCache.SetVaultAddress(RefMap(pluginConfig, "vaddress"))

	return &config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			WantCerts:           false,
			Insecure:            !exitOnFailure, // Plugin has exitOnFailure=false ...  always local, so this is ok...
			CurrentTokenNamePtr: &currentTokenName,
			TokenCache:          tokenCache,
			Env:                 pluginConfig["env"].(string),
			EnvBasis:            GetEnvBasis(pluginConfig["env"].(string)),
			Regions:             regions,
			ExitOnFailure:       exitOnFailure,
			Log:                 trcdbEnvLogger,
		},
		SecretMode:     true, //  "Only override secret values in templates?"
		ServicesWanted: []string{},
		StartDir:       append([]string{}, ""),
		EndDir:         "",
		GenAuth:        false,
	}, nil
}

// Helper to easiliy intialize a vault and a mod all at once.
func InitVaultModForTool(pluginConfig map[string]interface{}, driverConfig *config.DriverConfig) (*config.DriverConfig, *helperkv.Modifier, *sys.Vault, error) {
	exitOnFailure := false

	driverConfig.CoreConfig.Log.Println("InitVaultModForTool begin..")
	if _, ok := pluginConfig["exitOnFailure"]; ok {
		exitOnFailure = pluginConfig["exitOnFailure"].(bool)
	}

	driverConfig.CoreConfig.Log.Println("InitVaultModForTool initialize DriverConfig.")
	if _, vaddressOk := pluginConfig["vaddress"].(string); !vaddressOk {
		driverConfig.CoreConfig.Log.Println("Missing required vaddress")
		return nil, nil, nil, errors.New("missing required vaddress")
	}
	if _, envOk := pluginConfig["env"].(string); !envOk {
		driverConfig.CoreConfig.Log.Println("Missing required env")
		return nil, nil, nil, errors.New("missing required env")
	}

	if !driverConfig.CoreConfig.IsShell {
		driverConfig.CoreConfig.Log.Println("InitVaultModForTool region init.")
		var regions []string
		if regionsSlice, regionsOk := pluginConfig["regions"].([]string); regionsOk {
			regions = regionsSlice
		}

		driverConfig.CoreConfig.WantCerts = false
		driverConfig.CoreConfig.Insecure = !exitOnFailure // Plugin has exitOnFailure=false ...  always local, so this is ok...
		driverConfig.CoreConfig.TokenCache.SetVaultAddress(RefMap(pluginConfig, "vaddress"))
		driverConfig.CoreConfig.Env = pluginConfig["env"].(string)
		driverConfig.CoreConfig.EnvBasis = GetEnvBasis(pluginConfig["env"].(string))
		driverConfig.CoreConfig.Regions = regions
		driverConfig.CoreConfig.ExitOnFailure = exitOnFailure
	}

	driverConfig.SecretMode = true //  "Only override secret values in templates?"
	driverConfig.ServicesWanted = []string{}
	driverConfig.StartDir = append([]string{}, "")
	driverConfig.EndDir = ""
	driverConfig.GenAuth = false

	driverConfig.CoreConfig.Log.Println("InitVaultModForTool ends..")

	return InitVaultMod(driverConfig)
}

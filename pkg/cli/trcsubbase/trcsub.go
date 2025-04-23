package trcsubbase

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/pkg/core"
	"github.com/trimble-oss/tierceron/pkg/core/cache"
	il "github.com/trimble-oss/tierceron/pkg/trcinit/initlib"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

func PrintVersion() {
	fmt.Println("Version: " + "1.27")
}

// Reads in template files in specified directory
// Template directory should contain directories for each service
// Templates are uploaded to templates/<service>/<file name>/template-file
// The file is saved under the data key, and the extension under the ext key
// Vault automatically encodes the file into base64

func CommonMain(envDefaultPtr *string,
	envCtxPtr *string,
	tokenNamePtr *string,
	flagset *flag.FlagSet,
	argLines []string,
	driverConfig *config.DriverConfig) error {

	if driverConfig == nil || driverConfig.CoreConfig == nil || driverConfig.CoreConfig.TokenCache == nil {
		driverConfig = &config.DriverConfig{
			CoreConfig: &core.CoreConfig{
				ExitOnFailure: true,
				TokenCache:    cache.NewTokenCacheEmpty(),
			},
		}
	}

	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	var envPtr *string = nil
	var tokenPtr *string = nil
	var addrPtr *string = nil

	if flagset == nil {
		fmt.Println("Version: " + "1.6")
		flagset = flag.NewFlagSet(argLines[0], flag.ExitOnError)
		flagset.Usage = func() {
			fmt.Fprintf(flagset.Output(), "Usage of %s:\n", argLines[0])
			flagset.PrintDefaults()
		}
		if envDefaultPtr != nil {
			envPtr = flagset.String("env", *envDefaultPtr, "Environment to configure")
		} else {
			envPtr = flagset.String("env", "dev", "Environment to configure")
		}

		flagset.String("addr", "", "API endpoint for the vault")
		flagset.String("secretID", "", "Secret for app role ID")
		flagset.String("appRoleID", "", "Public app role ID")
		flagset.String("tokenName", "", "Token name used by this "+coreopts.BuildOptions.GetFolderPrefix(nil)+"pub to access the vault")
	} else {
		tokenPtr = flagset.String("token", "", "Vault access token")
		addrPtr = flagset.String("addr", "", "API endpoint for the vault")
	}
	endDirPtr := flagset.String("endDir", coreopts.BuildOptions.GetFolderPrefix(nil)+"_templates", "Directory to put configured templates into")
	pingPtr := flagset.Bool("ping", false, "Ping vault.")
	insecurePtr := flagset.Bool("insecure", false, "By default, every ssl connection this tool makes is verified secure.  This option allows to tool to continue with server connections considered insecure.")
	logFilePtr := flagset.String("log", "./"+coreopts.BuildOptions.GetFolderPrefix(nil)+"sub.log", "Output path for log files")
	projectInfoPtr := flagset.Bool("projectInfo", false, "Lists all project info")
	pluginInfoPtr := flagset.Bool("pluginInfo", false, "Lists all plugins")
	pluginNamePtr := flagset.String("pluginName", "", "Specifies which templates to filter")

	filterTemplatePtr := flagset.String("templateFilter", "", "Specifies which templates to filter")
	templatePathsPtr := flagset.String("templatePaths", "", "Specifies which specific templates to download.")

	flagset.Parse(argLines[1:])

	if envPtr == nil {
		if envDefaultPtr != nil {
			envPtr = envDefaultPtr
		} else {
			env := "dev"
			envPtr = &env
		}
	}
	envBasis := eUtils.GetEnvBasis(*envPtr)

	if len(*filterTemplatePtr) == 0 && len(*pluginNamePtr) == 0 && !*projectInfoPtr && !*pluginInfoPtr && *templatePathsPtr == "" {
		fmt.Printf("Must specify either -projectInfo, -fileTemplate, -pluginName, -pluginInfo, or -templateFilter flag \n")
		return errors.New("must specify either -projectInfo or -templateFilter flag")
	}
	var driverConfigBase *config.DriverConfig
	var currentRoleEntityPtr *string

	if driverConfig.CoreConfig.IsShell {
		driverConfigBase = driverConfig
		if len(driverConfigBase.EndDir) == 0 && len(*endDirPtr) != 0 {
			// Bad inputs... use default.
			driverConfigBase.EndDir = *endDirPtr
		}
		currentRoleEntityPtr = driverConfig.CoreConfig.CurrentRoleEntityPtr

	} else {
		// If logging production directory does not exist and is selected log to local directory
		if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/"+coreopts.BuildOptions.GetFolderPrefix(nil)+"sub.log" {
			*logFilePtr = "./" + coreopts.BuildOptions.GetFolderPrefix(nil) + "sub.log"
		}
		f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			fmt.Println("Log init failure")
			return err
		}

		logger := log.New(f, "[INIT]", log.LstdFlags)
		driverConfigBase = driverConfig
		driverConfigBase.CoreConfig.Insecure = *insecurePtr
		driverConfigBase.CoreConfig.Log = logger
		driverConfigBase.EndDir = *endDirPtr
		if eUtils.RefLength(tokenNamePtr) == 0 && eUtils.RefLength(tokenPtr) > 0 {
			tokenName := fmt.Sprintf("config_token_%s", envBasis)
			tokenNamePtr = &tokenName
		}
		driverConfigBase.CoreConfig.TokenCache.AddToken(*tokenNamePtr, tokenPtr)
		if eUtils.RefLength(addrPtr) > 0 {
			driverConfigBase.CoreConfig.TokenCache.SetVaultAddress(addrPtr)
		}
	}

	if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
		var err error
		*envPtr, err = eUtils.LoginToLocal()
		fmt.Println(*envPtr)
		eUtils.CheckError(driverConfigBase.CoreConfig, err, false)
		return err
	}

	wantedTokenName := fmt.Sprintf("config_token_%s", envBasis)
	autoErr := eUtils.AutoAuth(driverConfigBase,
		&wantedTokenName,
		&tokenPtr, // Token matching currentTokenNamePtr
		&envBasis,
		envCtxPtr,
		currentRoleEntityPtr,
		*pingPtr)
	if autoErr != nil {
		fmt.Println("Missing auth components.")
		return autoErr
	}
	fmt.Printf("Connecting to vault @ %s\n", *driverConfigBase.CoreConfig.TokenCache.VaultAddressPtr)

	mod, err := helperkv.NewModifierFromCoreConfig(driverConfigBase.CoreConfig,
		*tokenNamePtr,
		envBasis, true)
	if mod != nil {
		defer mod.Release()
	}
	if err != nil {
		fmt.Println("Failure to init to vault")
		driverConfigBase.CoreConfig.Log.Println("Failure to init to vault")
		return err
	}
	mod.Env = envBasis

	if len(*pluginNamePtr) > 0 {
		certifyMap, err := mod.ReadData(fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", *pluginNamePtr))
		if err != nil {
			fmt.Printf("Failure read plugin: %s\n", *pluginNamePtr)
			driverConfigBase.CoreConfig.Log.Printf("Failure read plugin: %s\n", *pluginNamePtr)
			return err
		}
		if eUtils.RefLength(filterTemplatePtr) == 0 {
			if projectFilter, ok := certifyMap["trcprojectservice"].(string); ok {
				filterTemplatePtr = &projectFilter
			} else {
				fmt.Printf("No additional secrets for plugin: %s\n", *pluginNamePtr)
				driverConfigBase.CoreConfig.Log.Printf("No additional secrets for plugin: %s\n", *pluginNamePtr)
				return err
			}
		}
	}

	if *templatePathsPtr != "" {
		fmt.Printf("Downloading templates from vault to %s\n", driverConfigBase.EndDir)
		// The actual download templates goes here.
		il.DownloadTemplates(driverConfigBase, mod, driverConfigBase.EndDir, driverConfigBase.CoreConfig.Log, templatePathsPtr)
	} else if *pluginInfoPtr {
		pluginList, err := mod.List("super-secrets/Index/TrcVault/trcplugin", driverConfigBase.CoreConfig.Log)
		if err != nil || pluginList == nil {
			fmt.Println("Failure read plugins")
			driverConfigBase.CoreConfig.Log.Println("Failure read plugins")
			return err
		}
		for _, pluginPath := range pluginList.Data {
			for _, pluginInterface := range pluginPath.([]interface{}) {
				plugin := pluginInterface.(string)
				fmt.Println(strings.TrimRight(plugin, "/"))
			}
		}

	} else if *projectInfoPtr {
		templateList, err := mod.List("templates/", driverConfigBase.CoreConfig.Log)
		if err != nil {
			fmt.Println("Failure read templates")
			driverConfigBase.CoreConfig.Log.Println("Failure read templates")
			return err
		}
		fmt.Printf("\nProjects available:\n")
		for _, templatePath := range templateList.Data {
			for _, projectInterface := range templatePath.([]interface{}) {
				project := projectInterface.(string)
				fmt.Println(strings.TrimRight(project, "/"))
			}
		}
		return nil
	} else {
		fmt.Printf("Downloading templates from vault to %s\n", driverConfigBase.EndDir)
		// The actual download templates goes here.
		warn, err := il.DownloadTemplateDirectory(driverConfigBase, mod, driverConfigBase.EndDir, driverConfigBase.CoreConfig.Log, filterTemplatePtr)
		if err != nil {
			fmt.Println(err)
			driverConfigBase.CoreConfig.Log.Printf("Failure to download: %s", err.Error())
			if strings.Contains(err.Error(), "x509: certificate") {
				return err
			}
		}
		eUtils.CheckError(driverConfigBase.CoreConfig, err, false)
		eUtils.CheckWarnings(driverConfigBase.CoreConfig, warn, false)
	}
	return nil
}

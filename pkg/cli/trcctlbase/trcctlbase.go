package trcctlbase

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	trcshMemFs "github.com/trimble-oss/tierceron/atrium/vestibulum/trcsh"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcshbase"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase"
	trcinitbase "github.com/trimble-oss/tierceron/pkg/cli/trcinitbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcpubbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcsubbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcxbase"
	"github.com/trimble-oss/tierceron/pkg/core"
	"github.com/trimble-oss/tierceron/pkg/core/cache"
	"github.com/trimble-oss/tierceron/pkg/core/util/hive"
	"github.com/trimble-oss/tierceron/pkg/trcx/xutil"
	"github.com/trimble-oss/tierceron/pkg/utils"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

func PrintVersion() {
	fmt.Println("Version: " + "1.37")
}

// This is a controller program that can act as any command line utility.
// The swiss army knife of tierceron if you will.
func CommonMain(envDefaultPtr *string,
	pluginNamePtr *string,
	tokenPtr *string,
	uploadCertPtr *bool,
	prodPtr *bool,
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
	var envCtxPtr *string = new(string)
	var logFilePtr *string = nil
	var addrPtr *string = nil

	if flagset == nil {
		fmt.Println("Version: " + "1.36")
		flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
		flagset.Usage = func() {
			fmt.Fprintf(flagset.Output(), "Usage of %s:\n", os.Args[0])
			flagset.PrintDefaults()
		}
		envPtr = flagset.String("env", "", "Environment to be seeded") //If this is blank -> use context otherwise override context.
		flagset.String("addr", "", "API endpoint for the vault")
		flagset.String("token", "", "Vault access token")
		flagset.Bool("certs", false, "Upload certs if provided")
		flagset.String("pluginName", "", "Specifies which templates to filter")
		flagset.Bool("prod", false, "Prod only seeds vault with staging environment")
		flagset.Bool("pluginInfo", false, "Lists all plugins")
		flagset.Bool("novault", false, "Don't pull configuration data from vault.")
		logFilePtr = flagset.String("log", "./"+coreopts.BuildOptions.GetFolderPrefix(nil)+"config.log", "Output path for log file")
	} else {
		logFilePtr = flagset.String("log", "./"+coreopts.BuildOptions.GetFolderPrefix(nil)+"config.log", "Output path for log file")
		addrPtr = flagset.String("addr", "", "API endpoint for the vault")
		flagset.Parse(argLines[2:])
		envPtr = envDefaultPtr
	}

	f, logErr := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if logErr != nil {
		return logErr
	}
	driverConfig.CoreConfig.Log = log.New(f, "["+coreopts.BuildOptions.GetFolderPrefix(nil)+"config]", log.LstdFlags)
	if utils.RefLength(addrPtr) == 0 {
		eUtils.ReadAuthParts(driverConfig, false)
	} else {
		driverConfig.CoreConfig.TokenCache.SetVaultAddress(addrPtr)
	}

	var ctl string
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") { //This pre checks arguments for ctl switch to allow parse to work with non "-" flags.
		ctl = os.Args[1]
		ctlSplit := strings.Split(ctl, " ")
		if len(ctlSplit) >= 2 {
			fmt.Println("Invalid arguments - only 1 non flag argument available at a time.")
			return errors.New("Invalid arguments - only 1 non flag argument available at a time.")
		}

		if len(os.Args) > 2 {
			os.Args = os.Args[1:]
		}
	}
	flagset.Parse(os.Args[1:])
	if flagset.NFlag() == 0 {
		flagset.Usage()
		os.Exit(0)
	}

	if ctl != "" {
		var err error
		if strings.Contains(ctl, "context") {
			contextSplit := strings.Split(ctl, "=")
			if len(contextSplit) == 1 {
				*envPtr, *envCtxPtr, err = eUtils.GetSetEnvContext(*envPtr, *envCtxPtr)
				if err != nil {
					fmt.Println(err.Error())
					return err
				}
				fmt.Println("Current context is set to " + *envCtxPtr)
			} else if len(contextSplit) == 2 {
				*envCtxPtr = contextSplit[1]
				*envPtr, *envCtxPtr, err = eUtils.GetSetEnvContext(*envPtr, *envCtxPtr)
				if err != nil {
					fmt.Println(err.Error())
					return err
				}
			}
		}
	}

	var err error

	*envPtr, *envCtxPtr, err = eUtils.GetSetEnvContext(*envPtr, *envCtxPtr)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	switch ctl {
	case "pub":
		tokenName := fmt.Sprintf("vault_pub_token_%s", eUtils.GetEnvBasis(*envPtr))
		flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
		trcpubbase.CommonMain(envPtr, envCtxPtr, &tokenName, flagset, os.Args, driverConfig)
	case "sub":
		tokenName := fmt.Sprintf("config_token_%s", eUtils.GetEnvBasis(*envPtr))
		flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
		trcsubbase.CommonMain(envPtr, envCtxPtr, &tokenName, flagset, os.Args, driverConfig)
	case "init":
		//tokenName := fmt.Sprintf("config_token_%s_unrestricted", eUtils.GetEnvBasis(*envPtr))
		flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
		flagset.String("env", "dev", "Environment to configure")
		flagset.String("addr", "", "API endpoint for the vault")
		trcinitbase.CommonMain(envPtr, envCtxPtr, nil, uploadCertPtr, flagset, os.Args, driverConfig)
	case "plugininit":
		//			tokenName := fmt.Sprintf("config_token_%s_unrestricted", eUtils.GetEnvBasis(*envPtr))
		os.Chdir(*pluginNamePtr)
		tokenName := fmt.Sprintf("vault_pub_token_%s", eUtils.GetEnvBasis(*envPtr))
		pubMappingInit := []string{""}

		if eUtils.RefLength(tokenPtr) > 0 {
			pubMappingInit = append(pubMappingInit, fmt.Sprintf("-token=%s", *tokenPtr))
		}

		flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
		trcpubbase.CommonMain(envPtr, envCtxPtr, &tokenName, flagset, pubMappingInit, driverConfig)
		retrictedMappingsMap := coreopts.BuildOptions.GetPluginRestrictedMappings()

		if pluginRestrictedMappings, ok := retrictedMappingsMap[*pluginNamePtr]; ok {
			for _, restrictedMapping := range pluginRestrictedMappings {
				restrictedMappingInit := []string{""}
				for _, restrictedMapEntry := range restrictedMapping {
					if strings.HasPrefix(restrictedMapEntry, "-restricted") {
						restrictedMappingInit = append(restrictedMappingInit, restrictedMapEntry)
					}
				}
				if eUtils.RefLength(tokenPtr) > 0 {
					//						restrictedMappingInit = append(restrictedMappingInit, fmt.Sprintf("-tokenName=%s", tokenName))
					restrictedMappingInit = append(restrictedMappingInit, fmt.Sprintf("-token=%s", *tokenPtr))
					restrictedMappingInit = append(restrictedMappingInit, fmt.Sprintf("-prod=%v", *prodPtr))
				}
				flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
				trcinitbase.CommonMain(envPtr, envCtxPtr, nil, uploadCertPtr, flagset, restrictedMappingInit, driverConfig)
			}
		}

		os.Chdir("..")
	case "config":
		tokenName := fmt.Sprintf("config_token_%s", eUtils.GetEnvBasis(*envPtr))
		flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
		trcconfigbase.CommonMain(envPtr, envCtxPtr, &tokenName, nil, nil, os.Args, driverConfig)
	case "pluginx":
		tokenName := fmt.Sprintf("config_token_%s", eUtils.GetEnvBasis(*envPtr))
		driverConfig.CoreConfig.TokenCache.AddToken(tokenName, tokenPtr)
		if len(*pluginNamePtr) == 0 {
			fmt.Printf("Must specify either -pluginName flag \n")
			return errors.New("Must specify either -pluginName flag \n")
		}

		os.Mkdir(*pluginNamePtr, 0700)
		os.Chdir(*pluginNamePtr)
		fmt.Printf("%s\n", *pluginNamePtr)
		flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
		retrictedMappingsMap := coreopts.BuildOptions.GetPluginRestrictedMappings()

		if pluginRestrictedMappings, ok := retrictedMappingsMap[*pluginNamePtr]; ok {

			os.Mkdir("trc_seeds", 0700)
			for _, restrictedMapping := range pluginRestrictedMappings {
				restrictedMappingSub := append([]string{"", os.Args[1]}, restrictedMapping[0])
				flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
				flagset.String("env", "dev", "Environment to configure")
				if eUtils.RefLength(tokenPtr) > 0 {
					restrictedMappingSub = append(restrictedMappingSub, fmt.Sprintf("-token=%s", *tokenPtr))
				}
				trcsubbase.CommonMain(envPtr, envCtxPtr, &tokenName, flagset, restrictedMappingSub, driverConfig)
				restrictedMappingX := append([]string{""}, restrictedMapping[1:]...)
				if eUtils.RefLength(tokenPtr) > 0 {
					restrictedMappingX = append(restrictedMappingX, fmt.Sprintf("-tokenName=%s", tokenName))
					restrictedMappingX = append(restrictedMappingX, fmt.Sprintf("-token=%s", *tokenPtr))
				}
				flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
				flagset.String("env", "dev", "Environment to configure")
				trcxbase.CommonMain(nil,
					xutil.GenerateSeedsFromVault,
					envPtr,
					driverConfig.CoreConfig.TokenCache.VaultAddressPtr,
					envCtxPtr,
					nil,
					flagset,
					restrictedMappingX)
			}
		} else {
			fmt.Printf("Plugin not registered with trcctl.\n")
		}
		os.Chdir("..")
	case "pluginrun":
		//		tokenName := fmt.Sprintf("config_token_%s", eUtils.GetEnvBasis(*envPtr))
		tokenName := "config_token_pluginany"
		driverConfig := config.DriverConfig{
			CoreConfig: &core.CoreConfig{
				IsShell:             true, // Pretent to be shell to keep things in memory
				TokenCache:          driverConfig.CoreConfig.TokenCache,
				ExitOnFailure:       true,
				CurrentTokenNamePtr: &tokenName,
				EnvBasis:            *envPtr,
				Env:                 *envPtr,
				Log:                 driverConfig.CoreConfig.Log,
			},
			SecretMode:        true,
			ZeroConfig:        true,
			SubOutputMemCache: true,
			ReadMemCache:      true,
			OutputMemCache:    true,
			MemFs:             trcshMemFs.NewTrcshMemFs(),
		}
		driverConfig.CoreConfig.TokenCache.AddToken(tokenName, tokenPtr)
		if len(*pluginNamePtr) == 0 {
			fmt.Printf("Must specify either -pluginName flag \n")
			return errors.New("Must specify either -pluginName flag \n")
		}

		os.Mkdir(*pluginNamePtr, 0700)
		os.Chdir(*pluginNamePtr)
		fmt.Printf("%s\n", *pluginNamePtr)
		flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
		GetPluginConfigs(&driverConfig, flagset, pluginNamePtr, ctl, envCtxPtr)
		os.Chdir("..")
		var pluginCompleteChan chan bool
		<-pluginCompleteChan

	case "edit":
		tokenName := fmt.Sprintf("config_token_%s_unrestricted", *envPtr)
		driverConfig := config.DriverConfig{
			CoreConfig: &core.CoreConfig{
				IsShell:             true, // Pretent to be shell to keep things in memory
				TokenCache:          driverConfig.CoreConfig.TokenCache,
				ExitOnFailure:       true,
				CurrentTokenNamePtr: &tokenName,
				EnvBasis:            *envPtr,
				Env:                 *envPtr,
				Log:                 driverConfig.CoreConfig.Log,
			},
			SecretMode:        true,
			ZeroConfig:        true,
			SubOutputMemCache: true,
			ReadMemCache:      true,
			OutputMemCache:    true,
			MemFs:             trcshMemFs.NewTrcshMemFs(),
		}
		driverConfig.CoreConfig.TokenCache.AddToken(tokenName, tokenPtr)

		// Services downstream several more limited tokens but all covered
		// by the scope of the unrestricted token.
		limitedTokenName := fmt.Sprintf("config_token_%s", *envPtr)
		driverConfig.CoreConfig.TokenCache.AddToken(limitedTokenName, tokenPtr)

		statTokenName := "config_token_pluginany"
		driverConfig.CoreConfig.TokenCache.AddToken(statTokenName, tokenPtr)

		configMap := map[string]interface{}{
			"plugin_name": "trcctl",
			"token_name":  tokenName,
			"vault_token": *tokenPtr,
			"vault_addr":  *driverConfig.CoreConfig.TokenCache.VaultAddressPtr,
			"agent_env":   *envPtr,
			//			"deployments": "fenestra",
			"deployments": "trcdb",
			//			"deployments": "fenestra,rosea,spiralis,trcdb",
			"region": "west",
		}
		trcshArgs := []string{}
		for i := 0; i < len(os.Args); i++ {
			if !strings.HasPrefix(os.Args[i], "-token=") {
				trcshArgs = append(trcshArgs, os.Args[i])
			}
		}
		flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
		err := trcshbase.CommonMain(envPtr, nil, flagset, trcshArgs, &configMap, &driverConfig)
		if err != nil {
			fmt.Println(err.Error())
			return err
		}

	case "x":
		flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
		trcxbase.CommonMain(nil, xutil.GenerateSeedsFromVault, envPtr, driverConfig.CoreConfig.TokenCache.VaultAddressPtr, envCtxPtr, nil, flagset, os.Args)
	}

	return nil
}

func GetPluginConfigs(driverConfig *config.DriverConfig, flagset *flag.FlagSet, pluginNamePtr *string, ctl string, envCtxPtr *string) {
	retrictedMappingsMap := coreopts.BuildOptions.GetPluginRestrictedMappings()

	if pluginRestrictedMappings, ok := retrictedMappingsMap[*pluginNamePtr]; ok {
		for _, restrictedMapping := range pluginRestrictedMappings {
			restrictedMappingSub := append([]string{"", os.Args[1]}, restrictedMapping[0])
			flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
			flagset.String("env", "dev", "Environment to configure")

			wantedTokenName := fmt.Sprintf("config_token_%s", eUtils.GetEnvBasis(driverConfig.CoreConfig.Env))

			trcsubbase.CommonMain(&driverConfig.CoreConfig.Env,
				envCtxPtr,
				&wantedTokenName,
				flagset,
				restrictedMappingSub,
				driverConfig)

			driverConfig.EndDir = "."
			restrictedMappingConfig := []string{"", os.Args[1]}
			flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
			flagset.String("env", "dev", "Environment to configure")

			// Get certs...
			driverConfig.CoreConfig.WantCerts = true
			trcconfigbase.CommonMain(&driverConfig.CoreConfig.Env,
				envCtxPtr,
				&wantedTokenName, // wantedTokenName
				nil,              // regionPtr
				flagset,
				restrictedMappingConfig,
				driverConfig)

			if strings.HasPrefix(restrictedMapping[0], "-templateFilter=") {
				filter := restrictedMapping[0][strings.Index(restrictedMapping[0], "=")+1:]
				filterParts := strings.Split(filter, ",")
				for _, filterPart := range filterParts {
					if !strings.HasPrefix(filterPart, "Common") {
						restrictedMappingConfig = append(restrictedMappingConfig, fmt.Sprintf("-servicesWanted=%s", filterPart))
						break
					}
				}
			}

			driverConfig.CoreConfig.WantCerts = false
			flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
			flagset.String("env", "dev", "Environment to configure")
			trcconfigbase.CommonMain(&driverConfig.CoreConfig.Env,
				envCtxPtr,
				&wantedTokenName, // tokenName
				nil,              // regionPtr
				flagset,
				restrictedMappingConfig,
				driverConfig)

			driverConfig.MemFs.ClearCache("./trc_templates")
			driverConfig.MemFs.ClearCache("./deploy")
			serviceConfig := map[string]interface{}{}
			driverConfig.MemFs.SerializeToMap(".", serviceConfig)
			pluginRestart := make(chan tccore.KernelCmd)
			chatReceiverChan := make(chan *tccore.ChatMsg)
			pluginHandler := &hive.PluginHandler{
				Name: *pluginNamePtr,
				ConfigContext: &tccore.ConfigContext{
					ChatReceiverChan: &chatReceiverChan,
					Log:              driverConfig.CoreConfig.Log,
				},
				KernelCtx: &hive.KernelCtx{
					PluginRestartChan: &pluginRestart,
				},
			}

			pluginHandler.RunPlugin(driverConfig, *pluginNamePtr, &serviceConfig)
		}
	} else {
		fmt.Printf("Plugin not registered with trcctl.\n")
	}
}

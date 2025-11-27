package trcctlbase

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/faiface/mainthread"
	trcshMemFs "github.com/trimble-oss/tierceron-core/v2/trcshfs"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/memonly"
	"github.com/trimble-oss/tierceron-core/v2/buildopts/memprotectopts"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig"
	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig/cache"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcshbase"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/pluginopts"
	"github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase"
	trcinitbase "github.com/trimble-oss/tierceron/pkg/cli/trcinitbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcpubbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcsubbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcxbase"
	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"
	"github.com/trimble-oss/tierceron/pkg/core/util/hive"
	"github.com/trimble-oss/tierceron/pkg/trcx/xutil"
	"github.com/trimble-oss/tierceron/pkg/utils"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

func PrintVersion() {
	fmt.Fprintln(os.Stderr, "Version: "+"1.39")
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
	driverConfig *config.DriverConfig,
) error {
	if driverConfig == nil || driverConfig.CoreConfig == nil || driverConfig.CoreConfig.TokenCache == nil {
		driverConfig = &config.DriverConfig{
			CoreConfig: &coreconfig.CoreConfig{
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
		fmt.Fprintln(os.Stderr, "Version: "+"1.37")
		flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
		flagset.Usage = func() {
			fmt.Fprintf(flagset.Output(), "Usage of %s:\n", os.Args[0])
			flagset.PrintDefaults()
		}
		envPtr = flagset.String("env", "", "Environment to be seeded") // If this is blank -> use context otherwise override context.
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

	f, logErr := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if logErr != nil {
		return logErr
	}
	driverConfig.CoreConfig.Log = log.New(f, "["+coreopts.BuildOptions.GetFolderPrefix(nil)+"config]", log.LstdFlags)
	if utils.RefLength(addrPtr) == 0 {
		eUtils.ReadAuthParts(driverConfig, false)
		if utils.RefLength(driverConfig.CoreConfig.TokenCache.VaultAddressPtr) == 0 {
			fmt.Fprintln(os.Stderr, "Missing required vault address")
			return errors.New("Missing required vault address")
		}
	} else {
		driverConfig.CoreConfig.TokenCache.SetVaultAddress(addrPtr)
	}

	var ctl string
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") { // This pre checks arguments for ctl switch to allow parse to work with non "-" flags.
		ctl = os.Args[1]
		ctlSplit := strings.Split(ctl, " ")
		if len(ctlSplit) >= 2 {
			fmt.Fprintln(os.Stderr, "Invalid arguments - only 1 non flag argument available at a time.")
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
					fmt.Fprintln(os.Stderr, err.Error())
					return err
				}
				fmt.Fprintln(os.Stderr, "Current context is set to "+*envCtxPtr)
			} else if len(contextSplit) == 2 {
				*envCtxPtr = contextSplit[1]
				*envPtr, *envCtxPtr, err = eUtils.GetSetEnvContext(*envPtr, *envCtxPtr)
				if err != nil {
					fmt.Fprintln(os.Stderr, err.Error())
					return err
				}
			}
		}
	}

	var err error

	*envPtr, *envCtxPtr, err = eUtils.GetSetEnvContext(*envPtr, *envCtxPtr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
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
		// tokenName := fmt.Sprintf("config_token_%s_unrestricted", eUtils.GetEnvBasis(*envPtr))
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
			fmt.Fprintf(os.Stderr, "Must specify either -pluginName flag \n")
			return errors.New("Must specify either -pluginName flag \n")
		}

		os.Mkdir(*pluginNamePtr, 0o700)
		os.Chdir(*pluginNamePtr)
		fmt.Fprintf(os.Stderr, "%s\n", *pluginNamePtr)
		flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
		GeneratePluginSeedData(pluginNamePtr, flagset, ctl, tokenPtr, envPtr, envCtxPtr, tokenName, driverConfig)
		os.Chdir("..")
	case "pluginrun":
		//		tokenName := fmt.Sprintf("config_token_%s", eUtils.GetEnvBasis(*envPtr))
		tokenName := "config_token_pluginany"
		driverConfig := config.DriverConfig{
			CoreConfig: &coreconfig.CoreConfig{
				IsShell:             true, // Pretent to be shell to keep things in memory
				IsEditor:            true, // Pretend to be editor.
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
			fmt.Fprintf(os.Stderr, "Must specify either -pluginName flag \n")
			return errors.New("Must specify either -pluginName flag \n")
		}

		os.Mkdir(*pluginNamePtr, 0o700)
		os.Chdir(*pluginNamePtr)
		fmt.Fprintf(os.Stderr, "%s\n", *pluginNamePtr)
		flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
		GetPluginConfigs(&driverConfig, flagset, pluginNamePtr, ctl, envCtxPtr)
		os.Chdir("..")
		var pluginCompleteChan chan bool
		<-pluginCompleteChan

	case "edit":
		coreopts.BuildOptions.IsSupportedFlow = coreopts.IsSupportedFlow
		var tokenName string
		if eUtils.RefLength(tokenPtr) > 0 {
			tokenName = fmt.Sprintf("config_token_%s_unrestricted", eUtils.GetEnvBasis(*envPtr))
		}
		editDriverConfig := config.DriverConfig{
			ShellRunner: func(dc *config.DriverConfig, pluginName string, scriptPath string) {
				if dc.CoreConfig.TokenCache.GetRole("hivekernel") == nil {
					deploy_role := os.Getenv("DEPLOY_ROLE")
					deploy_secret := os.Getenv("DEPLOY_SECRET")
					if len(deploy_role) > 0 && len(deploy_secret) > 0 {
						azureDeployRole := []string{deploy_role, deploy_secret}
						dc.CoreConfig.TokenCache.AddRole("hivekernel", &azureDeployRole)
					}
				}

				switch scriptPath {
				case "/edit/load.trc.tmpl":
					if len(tokenName) == 0 {
						tokenName = fmt.Sprintf("config_token_%s", *envPtr)
					}
					GeneratePluginSeedData(&pluginName, nil, ctl, tokenPtr, envPtr, envCtxPtr, tokenName, dc)
				case "/edit/save.trc.tmpl":
					uploadCert := false
					trcinitbase.CommonMain(&dc.CoreConfig.Env, envPtr, dc.CoreConfig.CurrentTokenNamePtr, &uploadCert, nil, []string{"", "deploy.trc.tmpl"}, dc)
				}
			},
			CoreConfig: &coreconfig.CoreConfig{
				IsShell:             true, // Pretend to be shell to keep things in memory
				IsEditor:            true,
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
		}
		if eUtils.RefLength(tokenPtr) == 0 {
			role := "bamboo"
			tokenWanted := fmt.Sprintf("config_token_%s", eUtils.GetEnvBasis(*envPtr))
			autoErr := eUtils.AutoAuth(&editDriverConfig, &tokenWanted, &tokenPtr, &editDriverConfig.CoreConfig.Env, &editDriverConfig.CoreConfig.EnvBasis, &role, false)
			if autoErr != nil {
				fmt.Fprintln(os.Stderr, autoErr.Error())
				return autoErr
			}
			tokenName = tokenWanted
		} else {
			editDriverConfig.CoreConfig.TokenCache.AddToken(tokenName, tokenPtr)
			// Services downstream several more limited tokens but all covered
			// by the scope of the unrestricted token.
			limitedTokenName := fmt.Sprintf("config_token_%s", *envPtr)
			editDriverConfig.CoreConfig.TokenCache.AddToken(limitedTokenName, tokenPtr)
		}
		statTokenName := "config_token_pluginany"
		editDriverConfig.CoreConfig.TokenCache.AddToken(statTokenName, tokenPtr)

		configMap := map[string]any{
			"plugin_name": "trcctl",
			"token_name":  tokenName,
			"vault_token": *tokenPtr,
			"vault_addr":  *editDriverConfig.CoreConfig.TokenCache.VaultAddressPtr,
			"agent_env":   *envPtr,
			//			"deployments": "fenestra",
			//			"deployments": "trcdb",
			"deployments": "rosea,trcdb",
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
		mainthread.Run(func() {
			err := trcshbase.CommonMain(envPtr, nil, flagset, trcshArgs, &configMap, &editDriverConfig)
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				return
			}
		})

	case "x":
		flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
		trcxbase.CommonMain(nil, xutil.GenerateSeedsFromVault, envPtr, driverConfig.CoreConfig.TokenCache.VaultAddressPtr, envCtxPtr, nil, flagset, os.Args, nil)
	}

	return nil
}

func GeneratePluginSeedData(pluginNamePtr *string, flagset *flag.FlagSet, ctl string, tokenPtr *string, envPtr *string, envCtxPtr *string, tokenName string, driverConfig *config.DriverConfig) {
	retrictedMappingsMap := coreopts.BuildOptions.GetPluginRestrictedMappings()

	if pluginRestrictedMappings, ok := retrictedMappingsMap[*pluginNamePtr]; ok {

		if driverConfig.OutputMemCache {
			if _, err := driverConfig.MemFs.Create(fmt.Sprintf("trc_seeds/%s/TODO.txt", *envPtr)); err != nil {
				fmt.Fprintf(os.Stderr, "Error setting up utility directory: %v\n", err)
				return
			}
		} else {
			os.Mkdir("trc_seeds", 0o700)
		}
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
				restrictedMappingX,
				driverConfig)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Plugin not registered with trcctl.\n")
	}
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
			var projServ []string

			if strings.HasPrefix(restrictedMapping[0], "-templateFilter=") {
				filter := restrictedMapping[0][strings.Index(restrictedMapping[0], "=")+1:]
				filterParts := strings.Split(filter, ",")
				for _, filterPart := range filterParts {
					if !strings.HasPrefix(filterPart, "Common") && !strings.HasSuffix(filterPart, "Build") {
						restrictedMappingConfig = append(restrictedMappingConfig, fmt.Sprintf("-servicesWanted=%s", filterPart))
						projServ = strings.Split(filterPart, "/")
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
			serviceConfig := map[string]any{}
			driverConfig.MemFs.SerializeToMap(".", serviceConfig)

			paths := pluginopts.BuildOptions.GetConfigPaths(*pluginNamePtr)
			path := paths[0]
			pluginConfig := make(map[string]any)
			pluginConfig["vaddress"] = *driverConfig.CoreConfig.TokenCache.VaultAddressPtr
			currentTokenName := fmt.Sprintf("config_token_%s", driverConfig.CoreConfig.EnvBasis)
			pluginConfig["tokenptr"] = driverConfig.CoreConfig.TokenCache.GetToken(currentTokenName)
			pluginConfig["env"] = driverConfig.CoreConfig.EnvBasis

			_, mod, vault, err := eUtils.InitVaultModForPlugin(pluginConfig,
				driverConfig.CoreConfig.TokenCache,
				wantedTokenName,
				driverConfig.CoreConfig.Log)
			if err != nil {
				driverConfig.CoreConfig.Log.Printf("Problem initializing mod: %s\n", err)
				return
			}
			properties, err := trcvutils.NewProperties(driverConfig.CoreConfig, vault, mod, mod.Env, projServ[0], projServ[1])
			if err != nil && !strings.Contains(err.Error(), "no data paths found when initing CDS") {
				driverConfig.CoreConfig.Log.Println("Couldn't create properties for regioned certify:" + err.Error())
				return
			}
			sc, ok := properties.GetRegionConfigValues(projServ[1], path)
        	if !ok {
				driverConfig.CoreConfig.Log.Printf("Unable to access configuration data for %s\n", *pluginNamePtr)
				return
			}
			serviceConfig[path] = &sc

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
		fmt.Fprintf(os.Stderr, "Plugin not registered with trcctl.\n")
	}
}

package trcdescartesbase

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcsubbase"
	"github.com/trimble-oss/tierceron/pkg/core"
	"github.com/trimble-oss/tierceron/pkg/core/cache"
	"github.com/trimble-oss/tierceron/pkg/core/util/hive"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

func PrintVersion() {
	fmt.Println("Version: " + "1.31")
}

func CommonMain(envDefaultPtr *string,
	addrPtr *string,
	pluginNamePtr *string,
	envCtxPtr *string,
	secretIDPtr *string,
	appRoleIDPtr *string,
	tokenNamePtr *string,
	regionPtr *string,
	flagset *flag.FlagSet,
	argLines []string,
	driverConfig *config.DriverConfig) error {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}

	// configCtx := &config.ConfigContext{
	// 	ResultMap:     make(map[string]*string),
	// 	EnvSlice:      make([]string, 0),
	// 	ResultChannel: make(chan *config.ResultData, 5),
	// 	FileSysIndex:  -1,
	// 	ConfigWg:      sync.WaitGroup{},
	// 	Mutex:         &sync.Mutex{},
	// }
	var envPtr *string = nil
	var tokenPtr *string = nil

	if flagset == nil {
		PrintVersion() // For trcsh
		flagset = flag.NewFlagSet(argLines[0], flag.ExitOnError)
		flagset.Usage = func() {
			fmt.Fprintf(flagset.Output(), "Usage of %s:\n", argLines[0])
			flagset.PrintDefaults()
		}

		envPtr = flagset.String("env", "", "Environment to configure")
		flagset.String("addr", "", "API endpoint for the vault")
		flagset.String("token", "", "Vault access token")
		flagset.String("secretID", "", "Secret for app role ID")
		flagset.String("region", "", "Region to be processed") //If this is blank -> use context otherwise override context.
		flagset.String("appRoleID", "", "Public app role ID")
		flagset.String("tokenName", "", "Token name used by this"+coreopts.BuildOptions.GetFolderPrefix(nil)+"config to access the vault")
	} else {
		tokenPtr = flagset.String("token", "", "Vault access token")
	}
	logFilePtr := flagset.String("log", "./"+coreopts.BuildOptions.GetFolderPrefix(nil)+"config.log", "Output path for log file")
	pingPtr := flagset.Bool("ping", false, "Ping vault.")
	insecurePtr := flagset.Bool("insecure", false, "By default, every ssl connection this tool makes is verified secure.  This option allows to tool to continue with server connections considered insecure.")
	noVaultPtr := flagset.Bool("novault", false, "Don't pull configuration data from vault.")

	isShell := false

	if driverConfig != nil {
		isShell = driverConfig.CoreConfig.IsShell
	}
	var cmd string
	if len(argLines) > 1 && !strings.HasPrefix(argLines[1], "-") { //This pre checks arguments for ctl switch to allow parse to work with non "-" flags.
		cmd = argLines[1]
		cmdSplit := strings.Split(cmd, " ")
		if len(cmdSplit) >= 2 {
			fmt.Println("Invalid arguments - only 1 non flag argument available at a time.")
			return errors.New("Invalid arguments - only 1 non flag argument available at a time.")
		}

		if len(argLines) > 2 {
			argLines = argLines[1:]
		}
	}
	if driverConfig == nil || !isShell {
		args := argLines[1:]
		for i := 0; i < len(args); i++ {
			s := args[i]
			if s[0] != '-' {
				fmt.Println("Wrong flag syntax: ", s)
				return fmt.Errorf("wrong flag syntax: %s", s)
			}
		}
		flagset.Parse(argLines[1:])
	} else {
		// TODO: rework to support standard arg parsing...
		for _, args := range argLines {
			if strings.HasPrefix(args, "-env") {
				envArgs := strings.Split(args, "=")
				if len(envArgs) > 1 {
					if envPtr == nil {
						env := ""
						envPtr = &env
					}
					*envPtr = envArgs[1]
				}
			} else if strings.HasPrefix(args, "-pluginName") {
				pluginArgs := strings.Split(args, "=")
				if len(pluginArgs) > 1 {
					if envPtr == nil {
						plugin := ""
						pluginNamePtr = &plugin
					}
					*pluginNamePtr = pluginArgs[1]
				}
			}
		}
		flagset.Parse(nil)
	}
	if envPtr == nil || len(*envPtr) == 0 || strings.HasPrefix(*envPtr, "$") {
		envPtr = envDefaultPtr
	}

	if strings.Contains(*envPtr, "*") {
		fmt.Println("* is not available as an environment suffix.")
		return errors.New("* is not available as an environment suffix")
	}

	var appRoleConfigPtr *string
	var driverConfigBase *config.DriverConfig
	if driverConfig.CoreConfig.IsShell {
		driverConfigBase = driverConfig
		*insecurePtr = driverConfigBase.CoreConfig.Insecure
		appRoleConfigPtr = driverConfigBase.CoreConfig.AppRoleConfigPtr

		if driverConfigBase.CoreConfig.Log == nil {
			f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
			if err != nil {
				fmt.Println("Error creating log file: " + *logFilePtr)
				return errors.New("Error creating log file: " + *logFilePtr)
			}
			logger := log.New(f, "["+coreopts.BuildOptions.GetFolderPrefix(nil)+"config]", log.LstdFlags)
			driverConfigBase.CoreConfig.Log = logger
			driverConfigBase.CoreConfig.Insecure = *insecurePtr
			driverConfigBase.NoVault = *noVaultPtr
		}
	} else {
		f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		logger := log.New(f, "["+coreopts.BuildOptions.GetFolderPrefix(nil)+"config]", log.LstdFlags)
		driverConfigBase = &config.DriverConfig{
			CoreConfig: &core.CoreConfig{Env: *envPtr, ExitOnFailure: true, Insecure: *insecurePtr, Log: logger},
			NoVault:    *noVaultPtr,
		}
		if eUtils.RefLength(tokenNamePtr) == 0 && eUtils.RefLength(tokenPtr) > 0 {
			tokenName := fmt.Sprintf("config_token_%s", eUtils.GetEnvBasis(driverConfig.CoreConfig.Env))
			tokenNamePtr = &tokenName
		}
		driverConfigBase.CoreConfig.TokenCache = cache.NewTokenCache(*tokenNamePtr, tokenPtr)
		driverConfig.CoreConfig.CurrentTokenNamePtr = tokenNamePtr

		appRoleConfigPtr = new(string)
		eUtils.CheckError(driverConfigBase.CoreConfig, err, true)
	}

	envVersion := strings.Split(*envPtr, "_") //Break apart env+version for token
	*envPtr = envVersion[0]

	if !*noVaultPtr {
		wantedTokenName := fmt.Sprintf("config_token_%s", eUtils.GetEnvBasis(driverConfigBase.CoreConfig.Env))
		autoErr := eUtils.AutoAuth(driverConfigBase,
			secretIDPtr,
			appRoleIDPtr,
			&wantedTokenName,
			&tokenPtr,
			envPtr,
			addrPtr,
			envCtxPtr,
			appRoleConfigPtr,
			*pingPtr)
		if autoErr != nil {
			if isShell {
				driverConfig.CoreConfig.Log.Printf("auth error: %s  Trcsh expecting <roleid>:<secretid>", autoErr)
			} else {
				driverConfigBase.CoreConfig.Log.Printf("auth error: %s", autoErr)
			}
			fmt.Println("Missing auth components.")
			return errors.New("missing auth components")
		}
		if *pingPtr {
			return nil
		}
	} else {
		token := "novault"
		driverConfigBase.CoreConfig.TokenCache.AddToken(fmt.Sprintf("config_token_%s", *envPtr), &token)
	}

	if len(envVersion) >= 2 { //Put back env+version together
		*envPtr = envVersion[0] + "_" + envVersion[1]
		if envVersion[1] == "" {
			fmt.Println("Must declare desired version number after '_' : -env=env1_ver1")
			return errors.New("must declare desired version number after '_' : -env=env1_ver1")
		}
	} else {
		*envPtr = envVersion[0] + "_0"
	}

	switch cmd {
	case "train":
		//		tokenName := fmt.Sprintf("config_token_%s", eUtils.GetEnvBasis(*envPtr))
		tokenName := "config_token_pluginany"
		driverConfig.CoreConfig.TokenCache = cache.NewTokenCache(tokenName, tokenPtr)
		driverConfig.CoreConfig.CurrentTokenNamePtr = &tokenName
		// driverConfig := config.DriverConfig{
		// 	CoreConfig: &core.CoreConfig{
		// 		IsShell:             true, // Pretent to be shell to keep things in memory
		// 		TokenCache:          cache.NewTokenCache(tokenName, tokenPtr),
		// 		ExitOnFailure:       true,
		// 		CurrentTokenNamePtr: &tokenName,
		// 		VaultAddressPtr:     addrPtr,
		// 		EnvBasis:            *envPtr,
		// 		Env:                 *envPtr,
		// 		// Log:                 logger,
		// 	},
		// 	SecretMode:        true,
		// 	ZeroConfig:        true,
		// 	SubOutputMemCache: true,
		// 	ReadMemCache:      true,
		// 	OutputMemCache:    true,
		// }
		if len(*pluginNamePtr) == 0 {
			fmt.Printf("Must specify either -pluginName flag \n")
			return errors.New("Must specify either -pluginName flag \n")
		}

		os.Mkdir(*pluginNamePtr, 0700)
		os.Chdir(*pluginNamePtr)
		fmt.Printf("%s\n", *pluginNamePtr)
		flagset = flag.NewFlagSet(cmd, flag.ExitOnError)
		GetPluginData(driverConfig, flagset, pluginNamePtr, cmd, envCtxPtr)
		os.Chdir("..")
		var pluginCompleteChan chan bool
		<-pluginCompleteChan

		// TODO: run the plugin...
	case "test":
		fmt.Println("yay!")
	}

	return nil
}

func GetPluginData(driverConfig *config.DriverConfig, flagset *flag.FlagSet, pluginNamePtr *string, ctl string, envCtxPtr *string) {
	retrictedMappingsMap := coreopts.BuildOptions.GetPluginRestrictedMappings()

	if pluginRestrictedMappings, ok := retrictedMappingsMap[*pluginNamePtr]; ok {
		for _, restrictedMapping := range pluginRestrictedMappings {
			restrictedMappingSub := append([]string{"", os.Args[1]}, restrictedMapping[0])
			flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
			flagset.String("env", "dev", "Environment to configure")

			wantedTokenName := fmt.Sprintf("config_token_%s", eUtils.GetEnvBasis(driverConfig.CoreConfig.Env))

			trcsubbase.CommonMain(&driverConfig.CoreConfig.Env,
				driverConfig.CoreConfig.VaultAddressPtr,
				envCtxPtr,
				new(string),
				new(string),
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
				driverConfig.CoreConfig.VaultAddressPtr,
				envCtxPtr,
				new(string),      // secretId
				new(string),      // approleId
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
				driverConfig.CoreConfig.VaultAddressPtr,
				envCtxPtr,
				new(string),      // secretId
				new(string),      // approleId
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
					Log: driverConfig.CoreConfig.Log,
				},
				KernelCtx: &hive.KernelCtx{
					PluginRestartChan: &pluginRestart,
				},
			}

			pluginHandler.RunPlugin(driverConfig, *pluginNamePtr, &serviceConfig, &chatReceiverChan)
		}
	} else {
		fmt.Printf("Plugin not registered with trcdescartes.\n")
	}
}

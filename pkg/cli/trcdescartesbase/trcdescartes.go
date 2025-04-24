package trcdescartesbase

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
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

var STARTDIR_DEFAULT string

var (
	ENDDIR_DEFAULT = "."
)

func PrintVersion() {
	fmt.Println("Version: " + "1.31")
}

func CommonMain(envDefaultPtr *string,
	addrPtr *string,
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
	STARTDIR_DEFAULT = coreopts.BuildOptions.GetFolderPrefix(nil) + "_templates"

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
	startDirPtr := flagset.String("startDir", STARTDIR_DEFAULT, "Template directory")
	endDirPtr := flagset.String("endDir", ENDDIR_DEFAULT, "Directory to put configured templates into")
	logFilePtr := flagset.String("log", "./"+coreopts.BuildOptions.GetFolderPrefix(nil)+"config.log", "Output path for log file")
	pingPtr := flagset.Bool("ping", false, "Ping vault.")
	insecurePtr := flagset.Bool("insecure", false, "By default, every ssl connection this tool makes is verified secure.  This option allows to tool to continue with server connections considered insecure.")
	noVaultPtr := flagset.Bool("novault", false, "Don't pull configuration data from vault.")

	isShell := false

	if driverConfig != nil {
		isShell = driverConfig.CoreConfig.IsShell
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
			}
		}
		flagset.Parse(nil)
	}
	if envPtr == nil || len(*envPtr) == 0 || strings.HasPrefix(*envPtr, "$") {
		envPtr = envDefaultPtr
	}
	if !isShell {
		if _, err := os.Stat(*startDirPtr); os.IsNotExist(err) {
			fmt.Println("Missing required template folder: " + *startDirPtr)
			return fmt.Errorf("missing required template folder: %s", *startDirPtr)
		}
	}

	if strings.Contains(*envPtr, "*") {
		fmt.Println("* is not available as an environment suffix.")
		return errors.New("* is not available as an environment suffix")
	}

	var appRoleConfigPtr *string
	var driverConfigBase *config.DriverConfig
	if driverConfig.CoreConfig.IsShell {
		driverConfigBase = driverConfig
		if len(driverConfigBase.EndDir) == 0 || *endDirPtr != ENDDIR_DEFAULT {
			// Honor inputs if provided...
			driverConfigBase.EndDir = *endDirPtr
		}
		if len(driverConfigBase.StartDir) == 0 || len(driverConfigBase.StartDir[0]) == 0 || *startDirPtr != STARTDIR_DEFAULT {
			// Bad inputs... use default.
			driverConfigBase.StartDir = append([]string{}, *startDirPtr)
		}
		*insecurePtr = driverConfigBase.CoreConfig.Insecure

		if driverConfigBase.CoreConfig.Log == nil {
			f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
			if err != nil {
				fmt.Println("Error creating log file: " + *logFilePtr)
				return errors.New("Error creating log file: " + *logFilePtr)
			}
			logger := log.New(f, "["+coreopts.BuildOptions.GetFolderPrefix(nil)+"config]", log.LstdFlags)
			driverConfigBase.CoreConfig.Log = logger
			driverConfigBase.CoreConfig.Insecure = *insecurePtr
			driverConfigBase.StartDir = append([]string{}, *startDirPtr)
			driverConfigBase.EndDir = *endDirPtr
			driverConfigBase.NoVault = *noVaultPtr
		}
	} else {
		f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		logger := log.New(f, "["+coreopts.BuildOptions.GetFolderPrefix(nil)+"config]", log.LstdFlags)
		driverConfigBase = &config.DriverConfig{
			CoreConfig: &core.CoreConfig{Env: *envPtr, ExitOnFailure: true, Insecure: *insecurePtr, Log: logger},
			StartDir:   append([]string{}, *startDirPtr),
			EndDir:     *endDirPtr,
			NoVault:    *noVaultPtr,
		}
		if eUtils.RefLength(tokenNamePtr) == 0 && eUtils.RefLength(tokenPtr) > 0 {
			tokenName := fmt.Sprintf("config_token_%s", eUtils.GetEnvBasis(driverConfig.CoreConfig.Env))
			tokenNamePtr = &tokenName
		}
		driverConfigBase.CoreConfig.TokenCache = cache.NewTokenCache(*tokenNamePtr, tokenPtr, addrPtr)
		driverConfig.CoreConfig.CurrentTokenNamePtr = tokenNamePtr

		appRoleConfigPtr = new(string)
		eUtils.CheckError(driverConfigBase.CoreConfig, err, true)
	}

	envVersion := strings.Split(*envPtr, "_") //Break apart env+version for token
	*envPtr = envVersion[0]

	if !*noVaultPtr {
		wantedTokenName := fmt.Sprintf("config_token_%s", eUtils.GetEnvBasis(driverConfigBase.CoreConfig.Env))
		autoErr := eUtils.AutoAuth(driverConfigBase,
			&wantedTokenName,
			&tokenPtr,
			envPtr,
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

	return nil
}

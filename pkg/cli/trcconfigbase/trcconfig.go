package trcconfigbase

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	vcutils "github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase/utils"
	"github.com/trimble-oss/tierceron/pkg/core"
	"github.com/trimble-oss/tierceron/pkg/utils"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"

	"github.com/google/go-cmp/cmp"
)

func messenger(configCtx *utils.ConfigContext, inData *string, inPath string) {
	var data utils.ResultData
	data.InData = inData
	data.InPath = inPath
	inPathSplit := strings.Split(inPath, "||.")
	configCtx.Mutex.Lock()
	_, present := configCtx.ResultMap["filesys||."+inPathSplit[1]]
	configCtx.Mutex.Unlock()
	//If data is filesys - skip fileSys loop
	if strings.Contains(inPath, "filesys") {
		goto skipSwitch
	}

	//Read file from filesys once per new file
	if configCtx.FileSysIndex != -1 && !present {
		path, err := os.Getwd()
		fileData, err1 := os.ReadFile(filepath.FromSlash(path + inPathSplit[1]))
		if err != nil || err1 != nil {
			fmt.Println("Error reading file: " + inPathSplit[1])
			return
		}
		dataStr := string(fileData)
		messenger(configCtx, &dataStr, "filesys||."+inPathSplit[1])
	}

skipSwitch:
	configCtx.ResultChannel <- &data
}

func receiver(configCtx *utils.ConfigContext) {

	for data := range configCtx.ResultChannel {
		if data != nil && data.Done {
			return
		}
		if data != nil && data.InData != nil && data.InPath != "" {
			configCtx.Mutex.Lock()
			configCtx.ResultMap[data.InPath] = data.InData
			configCtx.Mutex.Unlock()
		}
	}

}

var STARTDIR_DEFAULT string

var (
	ENDDIR_DEFAULT = "."
)

func CommonMain(envDefaultPtr *string,
	addrPtr *string,
	tokenPtr *string,
	envCtxPtr *string,
	secretIDPtr *string,
	appRoleIDPtr *string,
	tokenNamePtr *string,
	regionPtr *string,
	flagset *flag.FlagSet,
	argLines []string,
	driverConfig *eUtils.DriverConfig) error {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	STARTDIR_DEFAULT = coreopts.BuildOptions.GetFolderPrefix(nil) + "_templates"

	configCtx := &utils.ConfigContext{
		ResultMap:     make(map[string]*string),
		EnvSlice:      make([]string, 0),
		ResultChannel: make(chan *utils.ResultData, 5),
		FileSysIndex:  -1,
		ConfigWg:      sync.WaitGroup{},
		Mutex:         &sync.Mutex{},
	}
	var envPtr *string = nil

	if flagset == nil {
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
	}
	startDirPtr := flagset.String("startDir", STARTDIR_DEFAULT, "Template directory")
	endDirPtr := flagset.String("endDir", ENDDIR_DEFAULT, "Directory to put configured templates into")
	secretMode := flagset.Bool("secretMode", true, "Only override secret values in templates?")
	servicesWanted := flagset.String("servicesWanted", "", "Services to pull template values for, in the form 'service1,service2' (defaults to all services)")
	wantCertsPtr := flagset.Bool("certs", false, "Pull certificates into directory specified by endDirPtr")
	certDestPathPtr := flagset.String("certDestPath", "", "Override templated cert destination paths. Format of tmplFileName:certDirPath/file.pfx")
	keyStorePtr := flagset.String("keystore", "", "Put certificates into this keystore file.")
	logFilePtr := flagset.String("log", "./"+coreopts.BuildOptions.GetFolderPrefix(nil)+"config.log", "Output path for log file")
	pingPtr := flagset.Bool("ping", false, "Ping vault.")
	zcPtr := flagset.Bool("zc", false, "Zero config (no configuration option).")
	diffPtr := flagset.Bool("diff", false, "Diff files")
	fileFilterPtr := flagset.String("filter", "", "Filter files for diff")
	templateInfoPtr := flagset.Bool("templateInfo", false, "Version information about templates")
	versionInfoPtr := flagset.Bool("versions", false, "Version information about values")
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
			if args == "-certs" {
				driverConfig.CoreConfig.WantCerts = true
			} else if strings.HasPrefix(args, "-keystore") {
				storeArgs := strings.Split(args, "=")
				if len(storeArgs) > 1 {
					*keyStorePtr = storeArgs[1]
				}
			} else if strings.HasPrefix(args, "-endDir") {
				endDir := strings.Split(args, "=")
				if len(endDir) > 1 {
					*endDirPtr = endDir[1]
				}
			} else if strings.HasPrefix(args, "-certDestPath") {
				certDestPath := strings.Split(args, "=")
				if len(certDestPath) > 1 {
					*certDestPathPtr = certDestPath[1]
				}
			} else if strings.HasPrefix(args, "-env") {
				envArgs := strings.Split(args, "=")
				if len(envArgs) > 1 {
					*envPtr = envArgs[1]
				}
			}
		}
		flagset.Parse(nil)
		if driverConfig.CoreConfig.WantCerts {
			*wantCertsPtr = true
		}
	}
	if envPtr == nil || len(*envPtr) == 0 {
		envPtr = envDefaultPtr
	}
	if !isShell {
		if _, err := os.Stat(*startDirPtr); os.IsNotExist(err) {
			fmt.Println("Missing required template folder: " + *startDirPtr)
			return fmt.Errorf("missing required template folder: %s", *startDirPtr)
		}
	}

	if *zcPtr {
		*wantCertsPtr = false
	}

	if strings.Contains(*envPtr, "*") {
		fmt.Println("* is not available as an environment suffix.")
		return errors.New("* is not available as an environment suffix")
	}

	var appRoleConfigPtr *string
	var driverConfigBase *eUtils.DriverConfig
	if driverConfig != nil {
		driverConfigBase = driverConfig
		if len(driverConfigBase.EndDir) == 0 || *endDirPtr != ENDDIR_DEFAULT {
			// Honor inputs if provided...
			driverConfigBase.EndDir = *endDirPtr
		}
		if len(driverConfigBase.StartDir) == 0 || len(driverConfigBase.StartDir[0]) == 0 || *startDirPtr != STARTDIR_DEFAULT {
			// Bad inputs... use default.
			driverConfigBase.StartDir = append([]string{}, *startDirPtr)
		}
		*insecurePtr = driverConfigBase.Insecure
		appRoleConfigPtr = &(driverConfigBase.AppRoleConfig)
		if driverConfigBase.FileFilter != nil {
			fileFilterPtr = &(driverConfigBase.FileFilter[0])
		}
	} else {
		f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		logger := log.New(f, "["+coreopts.BuildOptions.GetFolderPrefix(nil)+"config]", log.LstdFlags)
		driverConfigBase = &eUtils.DriverConfig{
			CoreConfig: core.CoreConfig{ExitOnFailure: true, Log: logger},
			Insecure:   *insecurePtr,
			StartDir:   append([]string{}, *startDirPtr),
			EndDir:     *endDirPtr,
			ZeroConfig: *zcPtr,
		}

		appRoleConfigPtr = new(string)
		eUtils.CheckError(&driverConfigBase.CoreConfig, err, true)
	}

	//Dont allow these combinations of flags
	if *templateInfoPtr && *diffPtr {
		fmt.Println("Cannot use -diff flag and -templateInfo flag together")
		return errors.New("cannot use -diff flag and -templateInfo flag together")
	} else if *versionInfoPtr && *diffPtr {
		fmt.Println("Cannot use -diff flag and -versionInfo flag together")
		return errors.New("cannot use -diff flag and -versionInfo flag together")
	} else if *wantCertsPtr && *diffPtr {
		fmt.Println("Cannot use -diff flag and -certs flag together")
		return errors.New("cannot use -diff flag and -certs flag together")
	} else if *certDestPathPtr != "" && !*wantCertsPtr {
		fmt.Println("Cannot use -certDestPath flag without including -certs flag")
		return errors.New("Cannot use -certDestPath flag without including -certs flag")
	} else if *versionInfoPtr && *templateInfoPtr {
		fmt.Println("Cannot use -templateInfo flag and -versionInfo flag together")
		return errors.New("cannot use -templateInfo flag and -versionInfo flag together")
	} else if *diffPtr {
		if strings.ContainsAny(*envPtr, ",") { //Multiple environments
			*envPtr = strings.ReplaceAll(*envPtr, "latest", "0")
			configCtx.EnvSlice = strings.Split(*envPtr, ",")
			configCtx.EnvLength = len(configCtx.EnvSlice)
			if len(configCtx.EnvSlice) > 4 {
				fmt.Println("Unsupported number of environments - Maximum: 4")
				return errors.New("unsupported number of environments - Maximum: 4")
			}
			for i, env := range configCtx.EnvSlice {
				if env == "local" {
					fmt.Println("Unsupported env: local not available with diff flag")
					return errors.New("unsupported env: local not available with diff flag")
				}
				if !strings.Contains(env, "_") && env != "filesys" {
					configCtx.EnvSlice[i] = env + "_0"
				}
			}
			for i, env := range configCtx.EnvSlice {
				if env == "filesys" {
					configCtx.FileSysIndex = i
					configCtx.EnvSlice = append(configCtx.EnvSlice[:i], configCtx.EnvSlice[i+1:]...)
				}
			}
		} else {
			fmt.Println("Incorrect format for diff: -env=env1,env2,...")
			return errors.New("incorrect format for diff: -env=env1,env2")
		}
	} else {
		if strings.ContainsAny(*envPtr, ",") {
			fmt.Println("-diff flag is required for multiple environments - env: -env=env1,env2,...")
			return errors.New("-diff flag is required for multiple environments - env: -env=env1,env2")
		}
		if strings.Contains(*envPtr, "filesys") {
			fmt.Println("Unsupported env: filesys only available with diff flag")
			return errors.New("unsupported env: filesys only available with diff flag")
		}
		envVersion := strings.Split(*envPtr, "_") //Break apart env+version for token
		*envPtr = envVersion[0]

		if !*noVaultPtr {
			autoErr := eUtils.AutoAuth(driverConfigBase, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, envCtxPtr, *appRoleConfigPtr, *pingPtr)
			if autoErr != nil {
				if driverConfig != nil {
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
			*tokenPtr = "novault"
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
	}

	//Check if version is added on, process without it for versions & templateInfo flag
	if *versionInfoPtr || *templateInfoPtr {
		envVersion := strings.Split(*envPtr, "_")
		if len(envVersion) > 1 && envVersion[1] != "" && envVersion[1] != "0" {
			Yellow := "\033[33m"
			Reset := "\033[0m"
			if utils.IsWindows() {
				Reset = ""
				Yellow = ""
			}
			fmt.Println(Yellow + "Specified versioning not available, using " + envVersion[0] + " as environment" + Reset)
		}
	}

	if len(configCtx.EnvSlice) > 1 {
		removeDuplicateValuesSlice := eUtils.RemoveDuplicateValues(configCtx.EnvSlice)
		if !cmp.Equal(configCtx.EnvSlice, removeDuplicateValuesSlice) {
			fmt.Println("There is a duplicate environment in the -env flag")
			return errors.New("there is a duplicate environment in the -env flag")
		}
	}

	if !*diffPtr && (driverConfig == nil || !driverConfig.CoreConfig.IsShell) {
		if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
			var err error
			*envPtr, err = eUtils.LoginToLocal()
			fmt.Println(*envPtr)
			if err != nil {
				return err
			}
		}
	}

	if *servicesWanted != "" {
		driverConfigBase.ServicesWanted = strings.Split(*servicesWanted, ",")
	}

	regions := []string{}

	if strings.HasPrefix(*envPtr, "staging") || strings.HasPrefix(*envPtr, "prod") || strings.HasPrefix(*envPtr, "dev") {
		supportedRegions := eUtils.GetSupportedProdRegions()
		if regionPtr != nil && *regionPtr != "" {
			for _, supportedRegion := range supportedRegions {
				if *regionPtr == supportedRegion {
					regions = append(regions, *regionPtr)
					break
				}
			}
			if len(regions) == 0 {
				fmt.Println("Unsupported region: " + *regionPtr)
				return fmt.Errorf("unsupported region: %s", *regionPtr)
			}
		}
	}

	fileFilterSlice := make([]string, strings.Count(*fileFilterPtr, ",")+1)
	if strings.ContainsAny(*fileFilterPtr, ",") {
		fileFilterSlice = strings.Split(*fileFilterPtr, ",")
	} else if *fileFilterPtr != "" {
		fileFilterSlice[0] = *fileFilterPtr
	}

	certOverrides := make(map[string]string, strings.Count(*certDestPathPtr, ",")+1)
	if *certDestPathPtr != "" {
		for _, rebind := range strings.Split(*certDestPathPtr, ",") {
			split := strings.Split(rebind, ":")
			if len(split) != 2 {
				fmt.Println("Incorrect format for certDestPath: " + rebind)
				return fmt.Errorf("Incorrect format for certDestPath: " + rebind)
			}
			certFileName, certFileDest := split[0], split[1]
			if split[0] == "" || split[1] == "" {
				fmt.Println("Incorrect format for certDestPath: " + rebind)
				return fmt.Errorf("Incorrect format for certDestPath: " + rebind)
			}
			certOverrides[certFileName] = certFileDest
		}
	}

	//channel receiver
	go receiver(configCtx)
	if *diffPtr {
		configSlice := make([]eUtils.DriverConfig, 0, len(configCtx.EnvSlice)-1)
		for _, env := range configCtx.EnvSlice {
			envVersion := eUtils.SplitEnv(env)
			*envPtr = envVersion[0]
			*tokenPtr = ""
			if !*noVaultPtr {
				autoErr := eUtils.AutoAuth(driverConfigBase, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, envCtxPtr, *appRoleConfigPtr, *pingPtr)
				if autoErr != nil {
					fmt.Println("Missing auth components.")
					return errors.New("missing auth components")
				}
				if *pingPtr {
					return nil
				}
			} else {
				*tokenPtr = "novault"
			}
			if len(envVersion) >= 2 { //Put back env+version together
				*envPtr = envVersion[0] + "_" + envVersion[1]
				if envVersion[1] == "" {
					fmt.Println("Must declare desired version number after '_' : -env=env1_ver1,env2_ver2")
					return errors.New("must declare desired version number after '_' : -env=env1_ver1,env2_ver2")
				}
			} else {
				*envPtr = envVersion[0] + "_0"
			}
			if memonly.IsMemonly() {
				memprotectopts.MemUnprotectAll(nil)
				memprotectopts.MemProtect(nil, tokenPtr)
			}

			driverConfig := eUtils.DriverConfig{
				CoreConfig: core.CoreConfig{
					IsShell:       isShell,
					WantCerts:     *wantCertsPtr,
					ExitOnFailure: driverConfigBase.CoreConfig.ExitOnFailure,
					Log:           driverConfigBase.CoreConfig.Log,
				},
				IsShellSubProcess: driverConfigBase.IsShellSubProcess,
				Insecure:          *insecurePtr,
				Token:             *tokenPtr,
				VaultAddress:      *addrPtr,
				Env:               *envPtr,
				EnvBasis:          eUtils.GetEnvBasis(*envPtr),
				Regions:           regions,
				SecretMode:        *secretMode,
				ServicesWanted:    driverConfigBase.ServicesWanted,
				StartDir:          driverConfigBase.StartDir,
				EndDir:            driverConfigBase.EndDir,
				WantKeystore:      *keyStorePtr,
				ZeroConfig:        *zcPtr,
				GenAuth:           false,
				OutputMemCache:    driverConfigBase.OutputMemCache,
				MemFs:             driverConfigBase.MemFs,
				CertPathOverrides: certOverrides,
				Diff:              *diffPtr,
				Update:            messenger,
				FileFilter:        fileFilterSlice,
			}

			configSlice = append(configSlice, driverConfig)
			configCtx.ConfigWg.Add(1)
			go func(cs *[]eUtils.DriverConfig) {
				defer configCtx.ConfigWg.Done()
				eUtils.ConfigControl(nil, configCtx, &(*cs)[len(*cs)-1], vcutils.GenerateConfigsFromVault)
				if int(configCtx.GetDiffFileCount()) < (*cs)[len(*cs)-1].DiffCounter { //Without this, resultMap may be missing data when diffing.
					configCtx.SetDiffFileCount((*cs)[len(*cs)-1].DiffCounter) //This counter helps the diff wait for results
				}
			}(&configSlice)
		}
	} else {
		if memonly.IsMemonly() {
			memprotectopts.MemUnprotectAll(nil)
			memprotectopts.MemProtect(nil, tokenPtr)
		}

		if *templateInfoPtr {
			envVersion := strings.Split(*envPtr, "_")
			*envPtr = envVersion[0] + "_templateInfo"
		} else if *versionInfoPtr {
			envVersion := strings.Split(*envPtr, "_")
			*envPtr = envVersion[0] + "_versionInfo"
		}
		envVersion := strings.Split(*envPtr, "_")
		if len(envVersion) < 2 {
			*envPtr = envVersion[0] + "_0"
		}
		dConfig := eUtils.DriverConfig{
			CoreConfig: core.CoreConfig{
				IsShell:       isShell,
				WantCerts:     *wantCertsPtr,
				ExitOnFailure: driverConfigBase.CoreConfig.ExitOnFailure,
				Log:           driverConfigBase.CoreConfig.Log,
			},
			IsShellSubProcess: driverConfigBase.IsShellSubProcess,
			Insecure:          *insecurePtr,
			Token:             *tokenPtr,
			VaultAddress:      *addrPtr,
			Env:               *envPtr,
			EnvBasis:          eUtils.GetEnvBasis(*envPtr),
			Regions:           regions,
			SecretMode:        *secretMode,
			ServicesWanted:    driverConfigBase.ServicesWanted,
			StartDir:          driverConfigBase.StartDir,
			EndDir:            driverConfigBase.EndDir,
			WantKeystore:      *keyStorePtr,
			ZeroConfig:        driverConfigBase.ZeroConfig,
			GenAuth:           false,
			OutputMemCache:    driverConfigBase.OutputMemCache,
			MemFs:             driverConfigBase.MemFs,
			CertPathOverrides: certOverrides,
			Diff:              *diffPtr,
			FileFilter:        fileFilterSlice,
			VersionInfo:       eUtils.VersionHelper,
		}

		if len(driverConfigBase.DeploymentConfig) > 0 {
			dConfig.DeploymentConfig = driverConfigBase.DeploymentConfig
		}
		configCtx.ConfigWg.Add(1)
		go func(dc *eUtils.DriverConfig) {
			defer configCtx.ConfigWg.Done()
			eUtils.ConfigControl(nil, configCtx, dc, vcutils.GenerateConfigsFromVault)
		}(&dConfig)
	}
	configCtx.ConfigWg.Wait() //Wait for templates
	if driverConfig == nil {
		configCtx.ResultChannel <- &eUtils.ResultData{Done: true}
		close(configCtx.ResultChannel)
	} else if driverConfig.CoreConfig.IsShell {
		// Just shut down result channel since not really used in shell..
		configCtx.ResultChannel <- &eUtils.ResultData{Done: true}
		select {
		case _, ok := <-configCtx.ResultChannel:
			if ok {
				close(configCtx.ResultChannel)
			}
		case <-time.NewTicker(200 * time.Millisecond).C:
			close(configCtx.ResultChannel)
			break
		}
	}
	if *diffPtr { //Diff if needed
		if configCtx.FileSysIndex != -1 {
			configCtx.EnvSlice = append(configCtx.EnvSlice, "filesys")
			configCtx.EnvLength = len(configCtx.EnvSlice)
		}
		configCtx.ConfigWg.Add(1)
		go func() {
			defer configCtx.ConfigWg.Done()
			eUtils.DiffHelper(configCtx, true)
		}()
	}
	configCtx.ConfigWg.Wait() //Wait for diff
	return nil
}

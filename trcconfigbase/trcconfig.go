package trcconfigbase

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	vcutils "github.com/trimble-oss/tierceron/trcconfigbase/utils"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
	eUtils "github.com/trimble-oss/tierceron/utils"
	"github.com/trimble-oss/tierceron/utils/mlock"

	"github.com/google/go-cmp/cmp"
)

type ResultData struct {
	inData *string
	inPath string
}

var resultMap = make(map[string]*string)
var envDiffSlice = make([]string, 0)
var resultChannel = make(chan *ResultData, 5)
var fileSysIndex = -1
var envLength int
var wg sync.WaitGroup
var mutex = &sync.Mutex{}

func messenger(inData *string, inPath string) {
	var data ResultData
	data.inData = inData
	data.inPath = inPath
	inPathSplit := strings.Split(inPath, "||.")
	mutex.Lock()
	_, present := resultMap["filesys||."+inPathSplit[1]]
	mutex.Unlock()
	//If data is filesys - skip fileSys loop
	if strings.Contains(inPath, "filesys") {
		goto skipSwitch
	}

	//Read file from filesys once per new file
	if fileSysIndex != -1 && !present {
		path, err := os.Getwd()
		fileData, err1 := ioutil.ReadFile(filepath.FromSlash(path + inPathSplit[1]))
		if err != nil || err1 != nil {
			fmt.Println("Error reading file: " + inPathSplit[1])
			return
		}
		dataStr := string(fileData)
		messenger(&dataStr, "filesys||."+inPathSplit[1])
	}

skipSwitch:
	resultChannel <- &data
}

func reciever() {
	for {
		select {
		case data := <-resultChannel:
			if data != nil && data.inData != nil && data.inPath != "" {
				mutex.Lock()
				resultMap[data.inPath] = data.inData
				mutex.Unlock()
			}
		}
	}
}

func CommonMain(envPtr *string,
	addrPtr *string,
	tokenPtr *string,
	envCtxPtr *string,
	secretIDPtr *string,
	appRoleIDPtr *string,
	tokenNamePtr *string,
	regionPtr *string,
	c *eUtils.DriverConfig) {
	if memonly.IsMemonly() {
		mlock.Mlock(nil)
	}

	startDirPtr := flag.String("startDir", coreopts.GetFolderPrefix(nil)+"_templates", "Template directory")
	endDirPtr := flag.String("endDir", ".", "Directory to put configured templates into")
	secretMode := flag.Bool("secretMode", true, "Only override secret values in templates?")
	servicesWanted := flag.String("servicesWanted", "", "Services to pull template values for, in the form 'service1,service2' (defaults to all services)")
	wantCertsPtr := flag.Bool("certs", false, "Pull certificates into directory specified by endDirPtr")
	keyStorePtr := flag.String("keystore", "", "Put certificates into this keystore file.")
	logFilePtr := flag.String("log", "./"+coreopts.GetFolderPrefix(nil)+"config.log", "Output path for log file")
	pingPtr := flag.Bool("ping", false, "Ping vault.")
	zcPtr := flag.Bool("zc", false, "Zero config (no configuration option).")
	diffPtr := flag.Bool("diff", false, "Diff files")
	fileFilterPtr := flag.String("filter", "", "Filter files for diff")
	templateInfoPtr := flag.Bool("templateInfo", false, "Version information about templates")
	versionInfoPtr := flag.Bool("versions", false, "Version information about values")
	insecurePtr := flag.Bool("insecure", false, "By default, every ssl connection is secure.  Allows to continue with server connections considered insecure.")
	noVaultPtr := flag.Bool("novault", false, "Don't pull configuration data from vault.")

	if c == nil || !c.IsShellSubProcess {
		args := os.Args[1:]
		for i := 0; i < len(args); i++ {
			s := args[i]
			if s[0] != '-' {
				fmt.Println("Wrong flag syntax: ", s)
				os.Exit(1)
			}
		}
		flag.Parse()
	} else {
		// TODO: rework to support standard arg parsing...
		for _, args := range os.Args {
			if args == "-certs" {
				c.WantCerts = true
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
			}
		}
		flag.CommandLine.Parse(nil)
		if c.WantCerts {
			*wantCertsPtr = true
		}
	}

	if _, err := os.Stat(*startDirPtr); os.IsNotExist(err) {
		fmt.Println("Missing required template folder: " + *startDirPtr)
		os.Exit(1)
	}

	if *zcPtr {
		*wantCertsPtr = false
	}

	if strings.Contains(*envPtr, "*") {
		fmt.Println("* is not available as an environment suffix.")
		os.Exit(1)
	}

	var appRoleConfigPtr *string
	var configBase *eUtils.DriverConfig
	if c != nil {
		configBase = c
		*insecurePtr = configBase.Insecure
		appRoleConfigPtr = &(configBase.AppRoleConfig)
		if configBase.FileFilter != nil {
			fileFilterPtr = &(configBase.FileFilter[0])
		}
	} else {
		f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		logger := log.New(f, "["+coreopts.GetFolderPrefix(nil)+"config]", log.LstdFlags)
		configBase = &eUtils.DriverConfig{Insecure: true, Log: logger, ExitOnFailure: true}
		appRoleConfigPtr = new(string)
		eUtils.CheckError(configBase, err, true)
	}

	//Dont allow these combinations of flags
	if *templateInfoPtr && *diffPtr {
		fmt.Println("Cannot use -diff flag and -templateInfo flag together")
		os.Exit(1)
	} else if *versionInfoPtr && *diffPtr {
		fmt.Println("Cannot use -diff flag and -versionInfo flag together")
		os.Exit(1)
	} else if *versionInfoPtr && *templateInfoPtr {
		fmt.Println("Cannot use -templateInfo flag and -versionInfo flag together")
		os.Exit(1)
	} else if *diffPtr {
		if strings.ContainsAny(*envPtr, ",") { //Multiple environments
			*envPtr = strings.ReplaceAll(*envPtr, "latest", "0")
			envDiffSlice = strings.Split(*envPtr, ",")
			envLength = len(envDiffSlice)
			if len(envDiffSlice) > 4 {
				fmt.Println("Unsupported number of environments - Maximum: 4")
				os.Exit(1)
			}
			for i, env := range envDiffSlice {
				if env == "local" {
					fmt.Println("Unsupported env: local not available with diff flag")
					os.Exit(1)
				}
				if !strings.Contains(env, "_") && env != "filesys" {
					envDiffSlice[i] = env + "_0"
				}
			}
			for i, env := range envDiffSlice {
				if env == "filesys" {
					fileSysIndex = i
					envDiffSlice = append(envDiffSlice[:i], envDiffSlice[i+1:]...)
				}
			}
		} else {
			fmt.Println("Incorrect format for diff: -env=env1,env2,...")
			os.Exit(1)
		}
	} else {
		if strings.ContainsAny(*envPtr, ",") {
			fmt.Println("-diff flag is required for multiple environments - env: -env=env1,env2,...")
			os.Exit(1)
		}
		if strings.Contains(*envPtr, "filesys") {
			fmt.Println("Unsupported env: filesys only available with diff flag")
			os.Exit(1)
		}
		envVersion := strings.Split(*envPtr, "_") //Break apart env+version for token
		*envPtr = envVersion[0]

		if !*noVaultPtr {
			autoErr := eUtils.AutoAuth(configBase, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, envCtxPtr, *appRoleConfigPtr, *pingPtr)
			if autoErr != nil {
				fmt.Println("Missing auth components.")
				os.Exit(1)
			}
			if *pingPtr {
				os.Exit(0)
			}
		} else {
			*tokenPtr = "novault"
		}

		if len(envVersion) >= 2 { //Put back env+version together
			*envPtr = envVersion[0] + "_" + envVersion[1]
			if envVersion[1] == "" {
				fmt.Println("Must declare desired version number after '_' : -env=env1_ver1")
				os.Exit(1)
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
			if runtime.GOOS == "windows" {
				Reset = ""
				Yellow = ""
			}
			fmt.Println(Yellow + "Specified versioning not available, using " + envVersion[0] + " as environment" + Reset)
		}
	}

	if len(envDiffSlice) > 1 {
		removeDuplicateValuesSlice := eUtils.RemoveDuplicateValues(envDiffSlice)
		if !cmp.Equal(envDiffSlice, removeDuplicateValuesSlice) {
			fmt.Println("There is a duplicate environment in the -env flag")
			os.Exit(1)
		}
	}

	if !*diffPtr {
		if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
			var err error
			*envPtr, err = eUtils.LoginToLocal()
			fmt.Println(*envPtr)
			eUtils.CheckError(configBase, err, true)
		}
	}

	services := []string{}
	if *servicesWanted != "" {
		services = strings.Split(*servicesWanted, ",")
	}

	// TODO: This wasn't doing anything useful...  possibly remove?
	//	for _, service := range services {
	//		service = strings.TrimSpace(service)
	//	}
	regions := []string{}

	if strings.HasPrefix(*envPtr, "staging") || strings.HasPrefix(*envPtr, "prod") || strings.HasPrefix(*envPtr, "dev") {
		supportedRegions := eUtils.GetSupportedProdRegions()
		if *regionPtr != "" {
			for _, supportedRegion := range supportedRegions {
				if *regionPtr == supportedRegion {
					regions = append(regions, *regionPtr)
					break
				}
			}
			if len(regions) == 0 {
				fmt.Println("Unsupported region: " + *regionPtr)
				os.Exit(1)
			}
		}
	}

	fileFilterSlice := make([]string, strings.Count(*fileFilterPtr, ",")+1)
	if strings.ContainsAny(*fileFilterPtr, ",") {
		fileFilterSlice = strings.Split(*fileFilterPtr, ",")
	} else if *fileFilterPtr != "" {
		fileFilterSlice[0] = *fileFilterPtr
	}

	//channel reciever
	go reciever()
	var diffFileCount int
	if *diffPtr {
		configSlice := make([]eUtils.DriverConfig, 0, len(envDiffSlice)-1)
		for _, env := range envDiffSlice {
			envVersion := eUtils.SplitEnv(env)
			*envPtr = envVersion[0]
			*tokenPtr = ""
			if !*noVaultPtr {
				autoErr := eUtils.AutoAuth(configBase, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, envCtxPtr, *appRoleConfigPtr, *pingPtr)
				if autoErr != nil {
					fmt.Println("Missing auth components.")
					os.Exit(1)
				}
				if *pingPtr {
					os.Exit(0)
				}
			} else {
				*tokenPtr = "novault"
			}
			if len(envVersion) >= 2 { //Put back env+version together
				*envPtr = envVersion[0] + "_" + envVersion[1]
				if envVersion[1] == "" {
					fmt.Println("Must declare desired version number after '_' : -env=env1_ver1,env2_ver2")
					os.Exit(1)
				}
			} else {
				*envPtr = envVersion[0] + "_0"
			}
			if memonly.IsMemonly() {
				mlock.MunlockAll(nil)
				mlock.Mlock2(nil, tokenPtr)
			}

			config := eUtils.DriverConfig{
				Insecure:       *insecurePtr,
				Token:          *tokenPtr,
				VaultAddress:   *addrPtr,
				Env:            *envPtr,
				EnvRaw:         eUtils.GetRawEnv(*envPtr),
				Regions:        regions,
				SecretMode:     *secretMode,
				ServicesWanted: services,
				StartDir:       append([]string{}, *startDirPtr),
				EndDir:         *endDirPtr,
				WantCerts:      *wantCertsPtr,
				WantKeystore:   *keyStorePtr,
				ZeroConfig:     *zcPtr,
				GenAuth:        false,
				OutputMemCache: configBase.OutputMemCache,
				MemFs:          configBase.MemFs,
				Log:            configBase.Log,
				ExitOnFailure:  configBase.ExitOnFailure,
				Diff:           *diffPtr,
				Update:         messenger,
				FileFilter:     fileFilterSlice,
			}
			configSlice = append(configSlice, config)
			wg.Add(1)
			go func() {
				defer wg.Done()
				eUtils.ConfigControl(nil, &configSlice[len(configSlice)-1], vcutils.GenerateConfigsFromVault)
				if diffFileCount < configSlice[len(configSlice)-1].DiffCounter { //Without this, resultMap may be missing data when diffing.
					diffFileCount = configSlice[len(configSlice)-1].DiffCounter //This counter helps the diff wait for results
				}
			}()
		}
	} else {
		if memonly.IsMemonly() {
			mlock.MunlockAll(nil)
			mlock.Mlock2(nil, tokenPtr)
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
		config := eUtils.DriverConfig{
			Insecure:       *insecurePtr,
			Token:          *tokenPtr,
			VaultAddress:   *addrPtr,
			Env:            *envPtr,
			EnvRaw:         eUtils.GetRawEnv(*envPtr),
			Regions:        regions,
			SecretMode:     *secretMode,
			ServicesWanted: services,
			StartDir:       append([]string{}, *startDirPtr),
			EndDir:         *endDirPtr,
			WantCerts:      *wantCertsPtr,
			WantKeystore:   *keyStorePtr,
			ZeroConfig:     *zcPtr,
			GenAuth:        false,
			OutputMemCache: configBase.OutputMemCache,
			MemFs:          configBase.MemFs,
			ExitOnFailure:  configBase.ExitOnFailure,
			Log:            configBase.Log,
			Diff:           *diffPtr,
			FileFilter:     fileFilterSlice,
			VersionInfo:    eUtils.VersionHelper,
		}
		wg.Add(1)
		go func(c *eUtils.DriverConfig) {
			defer wg.Done()
			eUtils.ConfigControl(nil, c, vcutils.GenerateConfigsFromVault)
		}(&config)
	}
	wg.Wait() //Wait for templates
	if c == nil {
		close(resultChannel)
	} else if c.EndDir != "deploy" {
		close(resultChannel)
	}
	if *diffPtr { //Diff if needed
		if fileSysIndex != -1 {
			envDiffSlice = append(envDiffSlice, "filesys")
			envLength = len(envDiffSlice)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			eUtils.DiffHelper(resultMap, envLength, envDiffSlice, fileSysIndex, true, mutex, diffFileCount)
		}()
	}
	wg.Wait() //Wait for diff
}

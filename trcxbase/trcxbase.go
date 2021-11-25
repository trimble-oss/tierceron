package trcxbase

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"

	eUtils "tierceron/utils"
	"tierceron/vaulthelper/kv"
)

type ResultData struct {
	inData *string
	inPath string
}

var resultMap = make(map[string]*string)
var envSlice = make([]string, 0)
var resultChannel = make(chan *ResultData, 5)
var envLength int
var mutex = &sync.Mutex{}

func messenger(inData *string, inPath string) {
	var data ResultData
	data.inData = inData
	data.inPath = inPath
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
		default:
		}
	}
}

// CommonMain This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func CommonMain(ctx eUtils.ProcessContext, configDriver eUtils.ConfigDriver, envPtr *string, addrPtrIn *string, insecurePtrIn *bool) {
	// Executable input arguments(flags)
	addrPtr := flag.String("addr", "", "API endpoint for the vault")
	if addrPtrIn != nil && *addrPtrIn != "" {
		addrPtr = addrPtrIn
	}
	startDirPtr := flag.String("startDir", "trc_templates", "Pull templates from this directory")
	endDirPtr := flag.String("endDir", "./trc_seeds/", "Write generated seed files to this directory")
	logFilePtr := flag.String("log", "./var/log/trcx.log", "Output path for log file")
	helpPtr := flag.Bool("h", false, "Provide options for trcx")
	tokenPtr := flag.String("token", "", "Vault access token")
	secretMode := flag.Bool("secretMode", true, "Only override secret values in templates?")
	genAuth := flag.Bool("genAuth", false, "Generate auth section of seed data?")
	cleanPtr := flag.Bool("clean", false, "Cleans seed files locally")
	secretIDPtr := flag.String("secretID", "", "Secret app role ID")
	appRoleIDPtr := flag.String("appRoleID", "", "Public app role ID")
	tokenNamePtr := flag.String("tokenName", "", "Token name used by this trcx to access the vault")
	noVaultPtr := flag.Bool("novault", false, "Don't pull configuration data from vault.")
	pingPtr := flag.Bool("ping", false, "Ping vault.")

	var insecurePtr *bool
	if insecurePtrIn == nil {
		insecurePtr = flag.Bool("insecure", false, "By default, every ssl connection is secure.  Allows to continue with server connections considered insecure.")
	} else {
		insecurePtr = insecurePtrIn
	}

	diffPtr := flag.Bool("diff", false, "Diff files")
	versionPtr := flag.Bool("versions", false, "Gets version metadata information")
	wantCertsPtr := flag.Bool("certs", false, "Pull certificates into directory specified by endDirPtr")

	// Checks for proper flag input
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		s := args[i]
		if s[0] != '-' {
			fmt.Println("Wrong flag syntax: ", s)
			os.Exit(1)
		}
	}

	flag.Parse()

	Yellow := "\033[33m"
	Reset := "\033[0m"
	if runtime.GOOS == "windows" {
		Reset = ""
		Yellow = ""
	}

	//check for clean + env flag
	cleanPresent := false
	envPresent := false
	for _, arg := range args {
		if strings.Contains(arg, "clean") {
			cleanPresent = true
		}
		if strings.Contains(arg, "env") {
			envPresent = true
		}
	}

	if cleanPresent && !envPresent {
		fmt.Println("Environment must be defined with -env=env1,... for -clean usage")
		os.Exit(1)
	} else if *diffPtr && *versionPtr {
		fmt.Println("-version flag cannot be used with -diff flag")
		os.Exit(1)
	} else if (*envPtr == "staging" || *envPtr == "prod") && *addrPtr == "" {
		fmt.Println("The -addr flag must be used with staging/prod environment")
		os.Exit(1)
	}

	keysCheck := make(map[string]bool)
	listCheck := []string{}

	if *versionPtr {
		if strings.Contains(*envPtr, ",") {
			fmt.Println(Yellow + "Invalid environment, please specify one environment." + Reset)
			os.Exit(1)
		}
		envVersion := strings.Split(*envPtr, "_")
		if len(envVersion) > 1 && envVersion[1] != "" && envVersion[1] != "0" {

			fmt.Println(Yellow + "Specified versioning not available, using " + envVersion[0] + " as environment" + Reset)
		}
		envSlice = append(envSlice, *envPtr+"_versionInfo")
		goto skipDiff
	}

	//Diff flag parsing check
	if *diffPtr {
		if strings.ContainsAny(*envPtr, ",") { //Multiple environments
			*envPtr = strings.ReplaceAll(*envPtr, "latest", "0")
			envSlice = strings.Split(*envPtr, ",")
			envLength = len(envSlice)
			if len(envSlice) > 4 {
				fmt.Println("Unsupported number of environments - Maximum: 4")
				os.Exit(1)
			}
			for i, env := range envSlice {
				if env == "local" {
					fmt.Println("Unsupported env: local not available with diff flag")
					os.Exit(1)
				}
				if !strings.Contains(env, "_") {
					envSlice[i] = env + "_0"
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
		envSlice = append(envSlice, (*envPtr))
		envVersion := strings.Split(*envPtr, "_") //Break apart env+version for token
		*envPtr = envVersion[0]
		eUtils.AutoAuth(*insecurePtr, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, *pingPtr)
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

	//Duplicate env check
	for _, entry := range envSlice {
		if _, value := keysCheck[entry]; !value {
			keysCheck[entry] = true
			listCheck = append(listCheck, entry)
		}
	}

	if len(listCheck) != len(envSlice) {
		fmt.Printf("Cannot diff an environment against itself.\n")
		os.Exit(1)
	}

skipDiff:
	// Prints usage if no flags are specified
	if *helpPtr {
		flag.Usage()
		os.Exit(1)
	}
	if ctx == nil {
		if _, err := os.Stat(*startDirPtr); os.IsNotExist(err) {
			fmt.Println("Missing required start template folder: " + *startDirPtr)
			os.Exit(1)
		}
		if _, err := os.Stat(*endDirPtr); os.IsNotExist(err) {
			fmt.Println("Missing required start seed folder: " + *endDirPtr)
			os.Exit(1)
		}
	}

	// If logging production directory does not exist and is selected log to local directory
	if _, err := os.Stat("./var/log/"); os.IsNotExist(err) && *logFilePtr == "./var/log/trcx.log" {
		*logFilePtr = "./trcx.log"
	}

	regions := []string{}

	if len(envSlice) == 1 && !*noVaultPtr {
		if strings.HasPrefix(*envPtr, "staging") || strings.HasPrefix(*envPtr, "prod") {
			secretIDPtr = nil
			appRoleIDPtr = nil
		}
		if strings.HasPrefix(*envPtr, "staging") || strings.HasPrefix(*envPtr, "prod") || strings.HasPrefix(*envPtr, "dev") {
			regions = eUtils.GetSupportedProdRegions()
		}
		eUtils.AutoAuth(*insecurePtr, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, *pingPtr)
	}

	if (tokenPtr == nil || *tokenPtr == "") && !*noVaultPtr && len(envSlice) == 1 {
		fmt.Println("Missing required auth token.")
		os.Exit(1)
	}

	if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
		var err error
		*envPtr, err = eUtils.LoginToLocal()
		fmt.Println(*envPtr)
		eUtils.CheckError(err, true)
	}

	// Initialize logging
	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	eUtils.CheckError(err, true)
	logger := log.New(f, "[trcx]", log.LstdFlags)
	logger.Println("=============== Initializing Seed Generator ===============")

	logger.SetPrefix("[trcx]")
	logger.Printf("Looking for template(s) in directory: %s\n", *startDirPtr)

	var waitg sync.WaitGroup
	if len(envSlice) == 1 {
		if strings.Contains(envSlice[0], "*") {
			//Ask vault for list of dev.* environments, add to envSlice
			testMod, err := kv.NewModifier(*insecurePtr, *tokenPtr, *addrPtr, *envPtr, regions)
			testMod.Env = strings.Split(envSlice[0], ".")[0]
			listValues, err := testMod.ListEnv("values/")
			if err != nil {
				logger.Printf(err.Error())
			}
			newEnvSlice := make([]string, 0)
			if listValues == nil {
				fmt.Println("No enterprise IDs were found.")
				os.Exit(1)
			}
			for _, valuesPath := range listValues.Data {
				for _, envInterface := range valuesPath.([]interface{}) {
					env := envInterface.(string)
					if strings.Contains(env, ".") && strings.Contains(env, testMod.Env) {
						env = strings.ReplaceAll(env, "/", "")
						newEnvSlice = append(newEnvSlice, env)
					}
				}
			}
			envSlice = newEnvSlice
		}
	}
	go reciever() //Channel reciever
	for _, env := range envSlice {
		envVersion := strings.Split(env, "_") //Break apart env+version for token
		*envPtr = envVersion[0]
		if strings.Contains(*envPtr, ".") {
			*envPtr = strings.Split(*envPtr, ".")[0]
		}
		if secretIDPtr != nil && *secretIDPtr != "" && appRoleIDPtr != nil && *appRoleIDPtr != "" {
			*tokenPtr = ""
		}
		if !*noVaultPtr {
			eUtils.AutoAuth(*insecurePtr, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, *pingPtr)
		} else {
			*tokenPtr = "novault"
		}

		if len(envVersion) >= 2 { //Put back env+version together
			*envPtr = envVersion[0] + "_" + envVersion[1]
		} else {
			*envPtr = envVersion[0] + "_0"
		}
		config := eUtils.DriverConfig{
			Context:        ctx,
			Insecure:       *insecurePtr,
			Token:          *tokenPtr,
			VaultAddress:   *addrPtr,
			Env:            *envPtr,
			Regions:        regions,
			SecretMode:     *secretMode,
			ServicesWanted: []string{},
			StartDir:       append([]string{}, *startDirPtr),
			EndDir:         *endDirPtr,
			WantCerts:      *wantCertsPtr,
			GenAuth:        *genAuth,
			Log:            logger,
			Clean:          *cleanPtr,
			Diff:           *diffPtr,
			Update:         messenger,
			VersionInfo:    eUtils.VersionHelper,
		}
		waitg.Add(1)
		go func() {
			defer waitg.Done()
			eUtils.ConfigControl(ctx, config, configDriver)
		}()
	}
	waitg.Wait()
	close(resultChannel)
	if *diffPtr { //Diff if needed
		waitg.Add(1)
		go func() {
			defer waitg.Done()
			eUtils.DiffHelper(resultMap, envLength, envSlice, -1, false, mutex)
		}()
	}
	waitg.Wait() //Wait for diff

	logger.SetPrefix("[trcx]")
	logger.Println("=============== Terminating Seed Generator ===============")
	logger.SetPrefix("[END]")
	logger.Println()

	// Terminate logging
	f.Close()
}

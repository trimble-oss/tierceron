package trcxbase

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"

	"tierceron/trcx/xutil"
	eUtils "tierceron/utils"
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

func diffHelper() {
	keys := []string{}

	//Arranges keys for ordered output
	for _, env := range envSlice {
		keys = append(keys, env+"||"+env+"_seed.yml")
	}

	Reset := "\033[0m"
	Red := "\033[31m"
	Green := "\033[32m"
	Yellow := "\033[0;33m"

	if runtime.GOOS == "windows" {
		Reset = ""
		Red = ""
		Green = ""
		Yellow = ""
	}

	keyA := keys[0]
	keyB := keys[1]
	keySplitA := strings.Split(keyA, "||")
	keySplitB := strings.Split(keyB, "||")
	mutex.Lock()
	envFileKeyA := resultMap[keyA]
	envFileKeyB := resultMap[keyB]
	mutex.Unlock()

	//Seperator
	if runtime.GOOS == "windows" {
		fmt.Printf("\n======================================================================================")
	} else {
		fmt.Printf("\n\033[1;35m======================================================================================\033[0m")
	}
	switch envLength {
	case 4:
		keyC := keys[2]
		keyD := keys[3]
		keySplitC := strings.Split(keyC, "||")
		keySplitD := strings.Split(keyD, "||")
		mutex.Lock()
		envFileKeyC := resultMap[keyC]
		envFileKeyD := resultMap[keyD]
		mutex.Unlock()

		fmt.Print("\n" + Yellow + " (" + Reset + Red + "-Env-" + keySplitA[0] + Reset + Green + " +Env-" + keySplitB[0] + Reset + Yellow + ")" + Reset + "\n")
		fmt.Println(eUtils.LineByLineDiff(envFileKeyB, envFileKeyA))
		fmt.Print("\n" + Yellow + " (" + Reset + Red + "-Env-" + keySplitA[0] + Reset + Green + " +Env-" + keySplitC[0] + Reset + Yellow + ")" + Reset + "\n")
		fmt.Println(eUtils.LineByLineDiff(envFileKeyC, envFileKeyA))
		fmt.Print("\n" + Yellow + " (" + Reset + Red + "-Env-" + keySplitA[0] + Reset + Green + " +Env-" + keySplitD[0] + Reset + Yellow + ")" + Reset + "\n")
		fmt.Println(eUtils.LineByLineDiff(envFileKeyD, envFileKeyA))
		fmt.Print("\n" + Yellow + " (" + Reset + Red + "-Env-" + keySplitB[0] + Reset + Green + " +Env-" + keySplitC[0] + Reset + Yellow + ")" + Reset + "\n")
		fmt.Println(eUtils.LineByLineDiff(envFileKeyC, envFileKeyB))
		fmt.Print("\n" + Yellow + " (" + Reset + Red + "-Env-" + keySplitB[0] + Reset + Green + " +Env-" + keySplitD[0] + Reset + Yellow + ")" + Reset + "\n")
		fmt.Println(eUtils.LineByLineDiff(envFileKeyD, envFileKeyB))
		fmt.Print("\n" + Yellow + " (" + Reset + Red + "-Env-" + keySplitC[0] + Reset + Green + " +Env-" + keySplitD[0] + Reset + Yellow + ")" + Reset + "\n")
		fmt.Println(eUtils.LineByLineDiff(envFileKeyD, envFileKeyC))
	case 3:
		keyC := keys[2]
		keySplitC := strings.Split(keyC, "||")
		mutex.Lock()
		envFileKeyC := resultMap[keyC]
		mutex.Unlock()

		fmt.Print("\n" + Yellow + " (" + Reset + Red + "-Env-" + keySplitA[0] + Reset + Green + " +Env-" + keySplitB[0] + Reset + Yellow + ")" + Reset + "\n")
		fmt.Println(eUtils.LineByLineDiff(envFileKeyB, envFileKeyA))
		fmt.Print("\n" + Yellow + " (" + Reset + Red + "-Env-" + keySplitA[0] + Reset + Green + " +Env-" + keySplitC[0] + Reset + Yellow + ")" + Reset + "\n")
		fmt.Println(eUtils.LineByLineDiff(envFileKeyC, envFileKeyA))
		fmt.Print("\n" + Yellow + " (" + Reset + Red + "-Env-" + keySplitB[0] + Reset + Green + " +Env-" + keySplitC[0] + Reset + Yellow + ")" + Reset + "\n")
		fmt.Println(eUtils.LineByLineDiff(envFileKeyC, envFileKeyB))
	default:
		fmt.Print("\n" + Yellow + " (" + Reset + Red + "-Env-" + keySplitA[0] + Reset + Green + " +Env-" + keySplitB[0] + Reset + Yellow + ")" + Reset + "\n")
		fmt.Println(eUtils.LineByLineDiff(envFileKeyB, envFileKeyA))
	}

	//Seperator
	if runtime.GOOS == "windows" {
		fmt.Println("======================================================================================")
	} else {
		fmt.Println("\033[1;35m======================================================================================\033[0m")
	}
	keys = keys[:0] //Cleans keys for next file
}

// CommonMain This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func CommonMain(envPtr *string, addrPtrIn *string) {
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
	insecurePtr := flag.Bool("insecure", false, "By default, every ssl connection is secure.  Allows to continue with server connections considered insecure.")
	diffPtr := flag.Bool("diff", false, "Diff files")

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
	}

	//Diff flag parsing check
	if *diffPtr {
		if strings.ContainsAny(*envPtr, ",") { //Multiple environments
			envSlice = strings.Split(*envPtr, ",")
			envLength = len(envSlice)
			if len(envSlice) > 4 {
				fmt.Println("Unsupported number of environments - Maximum: 4")
				os.Exit(1)
			}
			for _, env := range envSlice {
				if env == "local" {
					fmt.Println("Unsupported env: local not available with diff flag")
					os.Exit(1)
				}
			}
		} else {
			fmt.Println("Incorrect format for diff: -env=env1,env2,...")
			os.Exit(1)
		}
	} else { //non diff
		if strings.ContainsAny(*envPtr, ",") { //Multiple environments
			envSlice = strings.Split(*envPtr, ",")
			envLength = len(envSlice)
		} else {
			envSlice = append(envSlice, *envPtr) //For single env
		}
	}

	// Prints usage if no flags are specified
	if *helpPtr {
		flag.Usage()
		os.Exit(1)
	}
	if _, err := os.Stat(*startDirPtr); os.IsNotExist(err) {
		fmt.Println("Missing required start template folder: " + *startDirPtr)
		os.Exit(1)
	}
	if _, err := os.Stat(*endDirPtr); os.IsNotExist(err) {
		fmt.Println("Missing required start seed folder: " + *endDirPtr)
		os.Exit(1)
	}

	// If logging production directory does not exist and is selected log to local directory
	if _, err := os.Stat("./var/log/"); os.IsNotExist(err) && *logFilePtr == "./var/log/trcx.log" {
		*logFilePtr = "./trcx.log"
	}

	regions := []string{}

	if len(envSlice) == 1 && !*noVaultPtr {
		if *envPtr == "staging" || *envPtr == "prod" {
			secretIDPtr = nil
			appRoleIDPtr = nil
			regions = eUtils.GetSupportedProdRegions()
		}
		eUtils.AutoAuth(*insecurePtr, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, *pingPtr)
	}

	if tokenPtr == nil || *tokenPtr == "" && !*noVaultPtr && len(envSlice) == 1 {
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
	go reciever() //Channel reciever
	for _, env := range envSlice {
		*envPtr = env
		if secretIDPtr != nil && *secretIDPtr != "" && appRoleIDPtr != nil && *appRoleIDPtr != "" {
			*tokenPtr = ""
		}
		if !*noVaultPtr {
			eUtils.AutoAuth(*insecurePtr, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, *pingPtr)
		} else {
			*tokenPtr = "novault"
		}
		config := eUtils.DriverConfig{
			Insecure:       *insecurePtr,
			Token:          *tokenPtr,
			VaultAddress:   *addrPtr,
			Env:            *envPtr,
			Regions:        regions,
			SecretMode:     *secretMode,
			ServicesWanted: []string{},
			StartDir:       append([]string{}, *startDirPtr),
			EndDir:         *endDirPtr,
			WantCert:       false,
			GenAuth:        *genAuth,
			Log:            logger,
			Clean:          *cleanPtr,
			Diff:           *diffPtr,
			Update:         messenger,
		}
		waitg.Add(1)
		go func() {
			defer waitg.Done()
			eUtils.ConfigControl(config, xutil.GenerateSeedsFromVault)
		}()
	}
	waitg.Wait()
	close(resultChannel)
	if *diffPtr { //Diff if needed
		waitg.Add(1)
		go func() {
			defer waitg.Done()
			diffHelper()
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

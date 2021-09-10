package main

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

	eUtils "Vault.Whoville/utils"
	"Vault.Whoville/vaultconfig/utils"
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
		default:
		}
	}
}

func diffHelper() {
	fileIndex := 0
	keys := []string{}
	mutex.Lock()
	fileList := make([]string, len(resultMap)/envLength)
	mutex.Unlock()

	//Make fileList
	for key := range resultMap {
		found := false
		keySplit := strings.Split(key, "||")

		for _, fileName := range fileList {
			if fileName == keySplit[1] {
				found = true
			}
		}

		if !found && len(fileList) > 0 {
			fileList[fileIndex] = keySplit[1]
			fileIndex++
		}
	}

	//Diff resultMap using fileList
	for _, fileName := range fileList {
		//Arranges keys for ordered output
		for i, env := range envDiffSlice {
			if i == fileSysIndex {
				keys = append(keys, "filesys||"+fileName)
			}
			keys = append(keys, env+"||"+fileName)
		}
		if fileSysIndex == len(envDiffSlice) {
			keys = append(keys, "filesys||"+fileName)
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

		keySplitA[0] = strings.ReplaceAll(keySplitA[0], "0", "latest")
		keySplitB[0] = strings.ReplaceAll(keySplitB[0], "0", "latest")
		switch envLength {
		case 4:
			keyC := keys[2]
			keyD := keys[3]
			keySplitC := strings.Split(keyC, "||")
			keySplitD := strings.Split(keyD, "||")
			mutex.Lock()
			envFileKeyC := resultMap[keyC]
			envFileKeyD := resultMap[keyD]
			keySplitC[0] = strings.ReplaceAll(keySplitC[0], "0", "latest")
			keySplitD[0] = strings.ReplaceAll(keySplitD[0], "0", "latest")
			mutex.Unlock()

			fmt.Print("\n" + Yellow + keySplitA[1] + " (" + Reset + Red + "-Env-" + keySplitA[0] + Reset + Green + " +Env-" + keySplitB[0] + Reset + Yellow + ")" + Reset + "\n")
			fmt.Println(eUtils.LineByLineDiff(envFileKeyB, envFileKeyA))
			fmt.Print("\n" + Yellow + keySplitA[1] + " (" + Reset + Red + "-Env-" + keySplitA[0] + Reset + Green + " +Env-" + keySplitC[0] + Reset + Yellow + ")" + Reset + "\n")
			fmt.Println(eUtils.LineByLineDiff(envFileKeyC, envFileKeyA))
			fmt.Print("\n" + Yellow + keySplitA[1] + " (" + Reset + Red + "-Env-" + keySplitA[0] + Reset + Green + " +Env-" + keySplitD[0] + Reset + Yellow + ")" + Reset + "\n")
			fmt.Println(eUtils.LineByLineDiff(envFileKeyD, envFileKeyA))
			fmt.Print("\n" + Yellow + keySplitA[1] + " (" + Reset + Red + "-Env-" + keySplitB[0] + Reset + Green + " +Env-" + keySplitC[0] + Reset + Yellow + ")" + Reset + "\n")
			fmt.Println(eUtils.LineByLineDiff(envFileKeyC, envFileKeyB))
			fmt.Print("\n" + Yellow + keySplitA[1] + " (" + Reset + Red + "-Env-" + keySplitB[0] + Reset + Green + " +Env-" + keySplitD[0] + Reset + Yellow + ")" + Reset + "\n")
			fmt.Println(eUtils.LineByLineDiff(envFileKeyD, envFileKeyB))
			fmt.Print("\n" + Yellow + keySplitA[1] + " (" + Reset + Red + "-Env-" + keySplitC[0] + Reset + Green + " +Env-" + keySplitD[0] + Reset + Yellow + ")" + Reset + "\n")
			fmt.Println(eUtils.LineByLineDiff(envFileKeyD, envFileKeyC))
		case 3:
			keyC := keys[2]
			keySplitC := strings.Split(keyC, "||")
			mutex.Lock()
			envFileKeyC := resultMap[keyC]
			mutex.Unlock()
			keySplitC[0] = strings.ReplaceAll(keySplitC[0], "0", "latest")

			fmt.Print("\n" + Yellow + keySplitA[1] + " (" + Reset + Red + "-Env-" + keySplitA[0] + Reset + Green + " +Env-" + keySplitB[0] + Reset + Yellow + ")" + Reset + "\n")
			fmt.Println(eUtils.LineByLineDiff(envFileKeyB, envFileKeyA))
			fmt.Print("\n" + Yellow + keySplitA[1] + " (" + Reset + Red + "-Env-" + keySplitA[0] + Reset + Green + " +Env-" + keySplitC[0] + Reset + Yellow + ")" + Reset + "\n")
			fmt.Println(eUtils.LineByLineDiff(envFileKeyC, envFileKeyA))
			fmt.Print("\n" + Yellow + keySplitA[1] + " (" + Reset + Red + "-Env-" + keySplitB[0] + Reset + Green + " +Env-" + keySplitC[0] + Reset + Yellow + ")" + Reset + "\n")
			fmt.Println(eUtils.LineByLineDiff(envFileKeyC, envFileKeyB))
		default:
			fmt.Print("\n" + Yellow + keySplitA[1] + " (" + Reset + Red + "-Env-" + keySplitA[0] + Reset + Green + " +Env-" + keySplitB[0] + Reset + Yellow + ")" + Reset + "\n")
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
}

func versionHelper(versionData map[string]interface{}) {
	for filename, versionMap := range versionData {
		fmt.Println("======================================================================================")
		fmt.Println(filename)
		fmt.Println("======================================================================================")
		for versionNumber, versionMetadata := range versionMap.(map[string]interface{}) {
			fmt.Println("Version " + versionNumber + " Metadata:")
			for field, fieldData := range versionMetadata.(map[string]interface{}) {
				fmt.Printf(field + ": ")
				fmt.Println(fieldData)
			}
		}
	}
	fmt.Println("======================================================================================")
}

func main() {
	fmt.Println("Version: " + "1.19")
	addrPtr := flag.String("addr", "", "API endpoint for the vault")
	tokenPtr := flag.String("token", "", "Vault access token")
	startDirPtr := flag.String("startDir", "vault_templates", "Template directory")
	endDirPtr := flag.String("endDir", ".", "Directory to put configured templates into")
	envPtr := flag.String("env", "dev", "Environment to configure")
	regionPtr := flag.String("region", "", "Region to configure")
	secretMode := flag.Bool("secretMode", true, "Only override secret values in templates?")
	servicesWanted := flag.String("servicesWanted", "", "Services to pull template values for, in the form 'service1,service2' (defaults to all services)")
	secretIDPtr := flag.String("secretID", "", "Secret app role ID")
	appRoleIDPtr := flag.String("appRoleID", "", "Public app role ID")
	tokenNamePtr := flag.String("tokenName", "", "Token name used by this vaultconfig to access the vault")
	wantCertPtr := flag.Bool("cert", false, "Pull certificate into directory specified by endDirPtr")
	logFilePtr := flag.String("log", "./vaultconfig.log", "Output path for log file")
	pingPtr := flag.Bool("ping", false, "Ping vault.")
	zcPtr := flag.Bool("zc", false, "Zero config (no configuration option).")
	diffPtr := flag.Bool("diff", false, "Diff files")
	fileFilterPtr := flag.String("filter", "", "Filter files for diff")
	versionInfoPtr := flag.Bool("versionInfo", false, "Version information about environment")

	args := os.Args[1:]

	for i := 0; i < len(args); i++ {
		s := args[i]
		if s[0] != '-' {
			fmt.Println("Wrong flag syntax: ", s)
			os.Exit(1)
		}
	}

	flag.Parse()

	if _, err := os.Stat(*startDirPtr); os.IsNotExist(err) {
		fmt.Println("Missing required template folder: " + *startDirPtr)
		os.Exit(1)
	}

	if *zcPtr {
		*wantCertPtr = false
	}

	if *versionInfoPtr && *diffPtr {
		fmt.Println("Cannot use -diff flag and -versionInfo flag together")
		os.Exit(1)
	}

	if *diffPtr {
		if strings.ContainsAny(*envPtr, ",") { //Multiple environments
			*envPtr = strings.ReplaceAll(*envPtr, "latest", "0")
			envDiffSlice = strings.Split(*envPtr, ",")
			envLength = len(envDiffSlice)
			if len(envDiffSlice) > 4 {
				fmt.Println("Unsupported number of environments - Maximum: 4")
				os.Exit(1)
			}
			for i, env := range envDiffSlice {
				if env == "filesys" {
					fileSysIndex = i
					envDiffSlice = append(envDiffSlice[:i], envDiffSlice[i+1:]...)
				}
				if env == "local" {
					fmt.Println("Unsupported env: local not available with diff flag")
					os.Exit(1)
				}
				if !strings.Contains(env, "_") {
					envDiffSlice[i] = env + "_0"
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
		eUtils.AutoAuth(secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, *pingPtr)
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

	if !*diffPtr {
		if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
			var err error
			*envPtr, err = eUtils.LoginToLocal()
			fmt.Println(*envPtr)
			eUtils.CheckError(err, true)
		}
	}

	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	eUtils.CheckError(err, true)
	logger := log.New(f, "[vaultconfig]", log.LstdFlags)
	services := []string{}
	if *servicesWanted != "" {
		services = strings.Split(*servicesWanted, ",")
	}

	for _, service := range services {
		service = strings.TrimSpace(service)
	}
	regions := []string{}

	if *envPtr == "staging" || *envPtr == "prod" {
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
	if *diffPtr {
		configSlice := make([]eUtils.DriverConfig, 0, len(envDiffSlice)-1)
		for _, env := range envDiffSlice {
			envVersion := strings.Split(env, "_") //Break apart env+version for token
			*envPtr = envVersion[0]
			*tokenPtr = ""
			eUtils.AutoAuth(secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, *pingPtr)
			if len(envVersion) >= 2 { //Put back env+version together
				*envPtr = envVersion[0] + "_" + envVersion[1]
				if envVersion[1] == "" {
					fmt.Println("Must declare desired version number after '_' : -env=env1_ver1,env2_ver2")
					os.Exit(1)
				}
			} else {
				*envPtr = envVersion[0] + "_0"
			}
			config := eUtils.DriverConfig{
				Token:          *tokenPtr,
				VaultAddress:   *addrPtr,
				Env:            *envPtr,
				Regions:        regions,
				SecretMode:     *secretMode,
				ServicesWanted: services,
				StartDir:       append([]string{}, *startDirPtr),
				EndDir:         *endDirPtr,
				WantCert:       *wantCertPtr,
				ZeroConfig:     *zcPtr,
				GenAuth:        false,
				Log:            logger,
				Diff:           *diffPtr,
				Update:         messenger,
				FileFilter:     fileFilterSlice,
			}
			configSlice = append(configSlice, config)
			wg.Add(1)
			go func() {
				defer wg.Done()
				eUtils.ConfigControl(configSlice[len(configSlice)-1], utils.GenerateConfigsFromVault)
			}()
		}
	} else {
		if *versionInfoPtr {
			*envPtr = *envPtr + "_versionInfo"
		}
		envVersion := strings.Split(*envPtr, "_")
		if len(envVersion) < 2 {
			*envPtr = envVersion[0] + "_0"
		}
		config := eUtils.DriverConfig{
			Token:          *tokenPtr,
			VaultAddress:   *addrPtr,
			Env:            *envPtr,
			Regions:        regions,
			SecretMode:     *secretMode,
			ServicesWanted: services,
			StartDir:       append([]string{}, *startDirPtr),
			EndDir:         *endDirPtr,
			WantCert:       *wantCertPtr,
			ZeroConfig:     *zcPtr,
			GenAuth:        false,
			Log:            logger,
			Diff:           *diffPtr,
			FileFilter:     fileFilterSlice,
			VersionInfo:    versionHelper,
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			eUtils.ConfigControl(config, utils.GenerateConfigsFromVault)
		}()
	}
	wg.Wait() //Wait for templates
	close(resultChannel)
	if *diffPtr { //Diff if needed
		if fileSysIndex != -1 {
			envDiffSlice = append(envDiffSlice, "filesys")
			envLength = len(envDiffSlice)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			diffHelper()
		}()
	}
	wg.Wait() //Wait for diff
}

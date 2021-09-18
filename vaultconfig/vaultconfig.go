package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	eUtils "Vault.Whoville/utils"
	"Vault.Whoville/vaultconfig/utils"
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

		latestVersionACheck := strings.Split(keySplitA[0], "_")
		if latestVersionACheck[1] == "0" {
			keySplitA[0] = strings.ReplaceAll(keySplitA[0], "0", "latest")
		}
		latestVersionBCheck := strings.Split(keySplitB[0], "_")
		if latestVersionBCheck[1] == "0" {
			keySplitB[0] = strings.ReplaceAll(keySplitB[0], "0", "latest")
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
			latestVersionCCheck := strings.Split(keySplitC[0], "_")
			if latestVersionCCheck[1] == "0" {
				keySplitC[0] = strings.ReplaceAll(keySplitC[0], "0", "latest")
			}
			latestVersionDCheck := strings.Split(keySplitD[0], "_")
			if latestVersionDCheck[1] == "0" {
				keySplitD[0] = strings.ReplaceAll(keySplitD[0], "0", "latest")
			}
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
			latestVersionCCheck := strings.Split(keySplitC[0], "_")
			if latestVersionCCheck[1] == "0" {
				keySplitC[0] = strings.ReplaceAll(keySplitC[0], "0", "latest")
			}

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

func versionHelper(versionData map[string]interface{}, templateOrValues bool, valuePath string) {
	Reset := "\033[0m"
	Cyan := "\033[36m"
	Red := "\033[31m"
	if runtime.GOOS == "windows" {
		Reset = ""
		Cyan = ""
		Red = ""
	}

	//template == true
	if templateOrValues {
		for _, versionMap := range versionData {
			for _, versionMetadata := range versionMap.(map[string]interface{}) {
				for field, data := range versionMetadata.(map[string]interface{}) {
					if field == "destroyed" && !data.(bool) {
						goto printOutput1
					}
				}
			}
		}
		return

	printOutput1:
		for filename, versionMap := range versionData {
			fmt.Println(Cyan + "======================================================================================")
			fmt.Println(filename)
			fmt.Println("======================================================================================" + Reset)
			keys := make([]int, 0, len(versionMap.(map[string]interface{})))
			for versionNumber, _ := range versionMap.(map[string]interface{}) {
				versionNo, err := strconv.Atoi(versionNumber)
				if err != nil {
					fmt.Println()
				}
				keys = append(keys, versionNo)
			}
			sort.Ints(keys)
			for i, key := range keys {
				versionNumber := fmt.Sprint(key)
				versionMetadata := versionMap.(map[string]interface{})[fmt.Sprint(key)]
				fmt.Println("Version " + string(versionNumber) + " Metadata:")

				fields := make([]string, 0, len(versionMetadata.(map[string]interface{})))
				for field, _ := range versionMetadata.(map[string]interface{}) {
					fields = append(fields, field)
				}
				sort.Strings(fields)
				for _, field := range fields {
					fmt.Printf(field + ": ")
					fmt.Println(versionMetadata.(map[string]interface{})[field])
				}
				if i != len(keys)-1 {
					fmt.Println(Red + "-------------------------------------------------------------------------------" + Reset)
				}
			}
		}
		fmt.Println(Cyan + "======================================================================================" + Reset)
	} else {
		for _, versionMetadata := range versionData {
			for field, data := range versionMetadata.(map[string]interface{}) {
				if field == "destroyed" && !data.(bool) {
					goto printOutput
				}
			}
		}
		return

	printOutput:
		fmt.Println(Cyan + "======================================================================================" + Reset)
		fmt.Println(Cyan + "ValuePath: " + valuePath)
		fmt.Println("======================================================================================" + Reset)
		keys := make([]int, 0, len(versionData))
		for versionNumber, _ := range versionData {
			versionNo, _ := strconv.ParseInt(versionNumber, 10, 64)
			keys = append(keys, int(versionNo))
		}
		sort.Ints(keys)
		for _, key := range keys {
			versionNumber := key
			versionMetadata := versionData[fmt.Sprint(key)]
			fields := make([]string, 0)
			fieldData := make(map[string]interface{}, 0)
			for field, data := range versionMetadata.(map[string]interface{}) {
				fields = append(fields, field)
				fieldData[field] = data
			}
			sort.Strings(fields)
			fmt.Println("Version " + fmt.Sprint(versionNumber) + " Metadata:")
			for _, field := range fields {
				fmt.Printf(field + ": ")
				fmt.Println(fieldData[field])
			}
			if keys[len(keys)-1] != versionNumber {
				fmt.Println(Red + "-------------------------------------------------------------------------------" + Reset)
			}
		}
	}
}

func removeDuplicateValues(intSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}

	for _, entry := range intSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
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
	templateInfoPtr := flag.Bool("templateInfo", false, "Version information about templates")
	valueInfoPtr := flag.Bool("valueInfo", false, "Version information about values")
	insecurePtr := flag.Bool("insecure", false, "By default, every ssl connection is secure.  Allows to continue with server connections considered insecure.")

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

	if *templateInfoPtr && *diffPtr {
		fmt.Println("Cannot use -diff flag and -templateInfo flag together")
		os.Exit(1)
	} else if *valueInfoPtr && *diffPtr {
		fmt.Println("Cannot use -diff flag and -valueInfo flag together")
		os.Exit(1)
	} else if *valueInfoPtr && *templateInfoPtr {
		fmt.Println("Cannot use -templateInfo flag and -valueInfo flag together")
		os.Exit(1)
	} else if *valueInfoPtr || *templateInfoPtr {
		envVersion := strings.Split(*envPtr, "_")
		if len(envVersion) > 1 && envVersion[1] != "" {
			Yellow := "\033[33m"
			Reset := "\033[0m"
			if runtime.GOOS == "windows" {
				Reset = ""
				Yellow = ""
			}
			fmt.Println(Yellow + "Specified versioning not available, using " + envVersion[0] + " as environment" + Reset)
		}
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

	if len(envDiffSlice) > 1 {
		removeDuplicateValuesSlice := removeDuplicateValues(envDiffSlice)
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
			eUtils.AutoAuth(*insecurePtr, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, *pingPtr)
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
				Insecure:       *insecurePtr,
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
		if *templateInfoPtr {
			envVersion := strings.Split(*envPtr, "_")
			*envPtr = envVersion[0] + "_templateInfo"
		} else if *valueInfoPtr {
			envVersion := strings.Split(*envPtr, "_")
			*envPtr = envVersion[0] + "_valueInfo"
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

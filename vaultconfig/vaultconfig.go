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

func messenger(inData *string, inPath string) {
	var data ResultData
	data.inData = inData
	data.inPath = inPath
	inPathSplit := strings.Split(inPath, "||.")
	_, present := resultMap["filesys||."+inPathSplit[1]]
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
				resultMap[data.inPath] = data.inData
			}
		default:
		}
	}
}

func diffHelper() {
	fileIndex := 0
	keys := []string{}
	fileList := make([]string, len(resultMap)/envLength)

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
		envFileKeyA := resultMap[keyA]
		envFileKeyB := resultMap[keyB]

		switch envLength {
		case 4:
			keyC := keys[2]
			keyD := keys[3]
			keySplitC := strings.Split(keyC, "||")
			keySplitD := strings.Split(keyD, "||")
			envFileKeyC := resultMap[keyC]
			envFileKeyD := resultMap[keyD]

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
			envFileKeyC := resultMap[keyC]

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

func main() {
	fmt.Println("Version: " + "1.18")
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

	if *diffPtr {
		if strings.ContainsAny(*envPtr, ",") {
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
			}
		} else {
			fmt.Println("Incorrect format for diff: -env=env1,env2,...")
			os.Exit(1)
		}
	} else {
		if strings.ContainsAny(*envPtr, ",") {
			fmt.Println("Incorrect format for env: -env=env1")
			os.Exit(1)
		}
		if strings.Contains(*envPtr, "filesys") {
			fmt.Println("Unsupported env: filesys only available with diff flag")
			os.Exit(1)
		}
		eUtils.AutoAuth(secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, *pingPtr)
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
			*envPtr = env
			*tokenPtr = ""
			eUtils.AutoAuth(secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, *pingPtr)
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
		wg.Add(1)
		go func() {
			defer wg.Done()
			diffHelper()
		}()
	}
	wg.Wait() //Wait for diff
}

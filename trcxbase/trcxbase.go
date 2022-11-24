package trcxbase

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"tierceron/buildopts/coreopts"
	"tierceron/trcvault/opts/memonly"
	eUtils "tierceron/utils"
	"tierceron/utils/mlock"
	helperkv "tierceron/vaulthelper/kv"

	"github.com/hashicorp/vault/api"
)

type ResultData struct {
	inData *string
	inPath string
}

var resultMap = make(map[string]*string)
var envSlice = make([]string, 0)
var projectSectionsSlice = make([]string, 0)
var resultChannel = make(chan *ResultData, 5)
var envLength int
var resultMapLock = &sync.Mutex{}

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
				resultMapLock.Lock()
				resultMap[data.inPath] = data.inData
				resultMapLock.Unlock()
			}
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
	startDirPtr := flag.String("startDir", coreopts.GetFolderPrefix()+"_templates", "Pull templates from this directory")
	endDirPtr := flag.String("endDir", "./"+coreopts.GetFolderPrefix()+"_seeds/", "Write generated seed files to this directory")
	logFilePtr := flag.String("log", "./"+coreopts.GetFolderPrefix()+"x.log", "Output path for log file")
	helpPtr := flag.Bool("h", false, "Provide options for "+coreopts.GetFolderPrefix()+"x")
	tokenPtr := flag.String("token", "", "Vault access token")
	secretMode := flag.Bool("secretMode", true, "Only override secret values in templates?")
	genAuth := flag.Bool("genAuth", false, "Generate auth section of seed data?")
	cleanPtr := flag.Bool("clean", false, "Cleans seed files locally")
	secretIDPtr := flag.String("secretID", "", "Secret app role ID")
	appRoleIDPtr := flag.String("appRoleID", "", "Public app role ID")
	tokenNamePtr := flag.String("tokenName", "", "Token name used by this "+coreopts.GetFolderPrefix()+"x to access the vault")
	noVaultPtr := flag.Bool("novault", false, "Don't pull configuration data from vault.")
	pingPtr := flag.Bool("ping", false, "Ping vault.")

	fileAddrPtr := flag.String("seedpath", "", "Path for seed file")
	fieldsPtr := flag.String("fields", "", "Fields to enter")
	encryptedPtr := flag.String("encrypted", "", "Fields to encrypt")
	readOnlyPtr := flag.Bool("readonly", false, "Fields to encrypt")
	dynamicPathPtr := flag.String("dynamicPath", "", "Generate seeds for a dynamic path in vault.")

	var insecurePtr *bool
	if insecurePtrIn == nil {
		insecurePtr = flag.Bool("insecure", false, "By default, every ssl connection is secure.  Allows to continue with server connections considered insecure.")
	} else {
		insecurePtr = insecurePtrIn
	}

	diffPtr := flag.Bool("diff", false, "Diff files")
	versionPtr := flag.Bool("versions", false, "Gets version metadata information")
	wantCertsPtr := flag.Bool("certs", false, "Pull certificates into directory specified by endDirPtr")
	filterTemplatePtr := flag.String("templateFilter", "", "Specifies which templates to filter") // -templateFilter=config.yml

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

	config := &eUtils.DriverConfig{ExitOnFailure: true, Insecure: *insecurePtr}

	// Initialize logging
	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if f != nil {
		// Terminate logging
		defer f.Close()
	}
	eUtils.CheckError(config, err, true)
	logger := log.New(f, "["+coreopts.GetFolderPrefix()+"x]", log.LstdFlags)
	config.Log = logger

	envRaw := *envPtr

	Yellow := "\033[33m"
	Reset := "\033[0m"
	if runtime.GOOS == "windows" {
		Reset = ""
		Yellow = ""
	}

	var fileFilter []string
	if len(*filterTemplatePtr) != 0 {
		fileFilter = strings.Split(*filterTemplatePtr, ",")
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
	} else if (len(*eUtils.ServiceFilterPtr) == 0 || len(*eUtils.IndexNameFilterPtr) == 0) && len(*eUtils.IndexedPtr) != 0 {
		fmt.Println("-serviceFilter and -indexFilter must be specified to use -indexed flag")
		os.Exit(1)
	} else if len(*eUtils.ServiceFilterPtr) == 0 && len(*eUtils.RestrictedPtr) != 0 {
		fmt.Println("-serviceFilter must be specified to use -restricted flag")
		os.Exit(1)
	} else if (len(*eUtils.ServiceFilterPtr) == 0 || len(*eUtils.IndexValueFilterPtr) == 0) && *diffPtr && len(*eUtils.IndexedPtr) != 0 {
		fmt.Println("-indexFilter and -indexValueFilter must be specified to use -indexed & -diff flag")
		os.Exit(1)
	} else if (len(*eUtils.ServiceFilterPtr) == 0 || len(*eUtils.IndexValueFilterPtr) == 0) && *versionPtr && len(*eUtils.IndexedPtr) != 0 {
		fmt.Println("-indexFilter and -indexValueFilter must be specified to use -indexed & -versions flag")
		os.Exit(1)
	} else if *versionPtr && len(*eUtils.RestrictedPtr) > 0 {
		fmt.Println("-restricted flags cannot be used with -versions flag")
		os.Exit(1)
	} else if (strings.HasPrefix(*envPtr, "staging") || strings.HasPrefix(*envPtr, "prod")) && *addrPtr == "" {
		fmt.Println("The -addr flag must be used with staging/prod environment")
		os.Exit(1)
	} else if (len(*fieldsPtr) == 0) && len(*fileAddrPtr) != 0 {
		fmt.Println("The -fields flag must be used with -seedPath flag; -encrypted flag is optional")
		os.Exit(1)
	} else if *readOnlyPtr && (len(*encryptedPtr) == 0 || len(*fileAddrPtr) == 0) {
		fmt.Println("The -encrypted flag must be used with -seedPath flag if -readonly is used")
		os.Exit(1)
	}

	trcxe := false
	sectionSlice := []string{""}
	if len(*fileAddrPtr) != 0 { //Checks if seed file exists & figured out if index/restricted
		trcxe = true
		directorySplit := strings.Split(*fileAddrPtr, "/")
		indexed := false
		if !*noVaultPtr {
			pwd, _ := os.Getwd()
			_, fileErr := os.Open(pwd + "/" + coreopts.GetFolderPrefix() + "_seeds/" + *envPtr + "/Index/" + *fileAddrPtr + "_seed.yml")
			if errors.Is(fileErr, os.ErrNotExist) {
				_, fileRErr := os.Open(pwd + "/" + coreopts.GetFolderPrefix() + "_seeds/" + *envPtr + "/Restricted/" + *fileAddrPtr + "_seed.yml")
				if errors.Is(fileRErr, os.ErrNotExist) {
					fmt.Println("Specified seed file could not be found.")
					os.Exit(1)
				}
			} else {
				indexed = true
			}
		} else {
			indexed = true
		}

		if indexed {
			if len(directorySplit) >= 3 { //Don't like this, will change later
				*eUtils.IndexedPtr = directorySplit[0]
				*eUtils.IndexNameFilterPtr = directorySplit[1]
				*eUtils.IndexValueFilterPtr = directorySplit[2]
				sectionSlice = strings.Split(*eUtils.IndexValueFilterPtr, ",")
			}
		} else {
			fmt.Println("Not supported for restricted section.")
			os.Exit(1)
		}
	}

	if len(*eUtils.ServiceFilterPtr) != 0 && len(*eUtils.IndexNameFilterPtr) == 0 && len(*eUtils.RestrictedPtr) != 0 {
		eUtils.IndexNameFilterPtr = eUtils.ServiceFilterPtr
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
			*envPtr = strings.Split(*envPtr, "_")[0]
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
		if !*noVaultPtr {
			autoErr := eUtils.AutoAuth(config, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, *pingPtr)

			if autoErr != nil {
				fmt.Println("Auth failure: " + autoErr.Error())
				eUtils.LogErrorMessage(config, autoErr.Error(), true)
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

	if memonly.IsMemonly() {
		mlock.MunlockAll(nil)
		mlock.Mlock2(nil, tokenPtr)
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
	if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/"+coreopts.GetFolderPrefix()+"x.log" {
		*logFilePtr = "./" + coreopts.GetFolderPrefix() + "x.log"
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
		autoErr := eUtils.AutoAuth(&eUtils.DriverConfig{Insecure: *insecurePtr, Log: logger, ExitOnFailure: true}, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, *pingPtr)
		if autoErr != nil {
			fmt.Println("Missing auth components.")
			eUtils.LogErrorMessage(config, autoErr.Error(), true)
		}
	}

	if (tokenPtr == nil || *tokenPtr == "") && !*noVaultPtr && len(envSlice) == 1 {
		fmt.Println("Missing required auth token.")
		os.Exit(1)
	}

	if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
		var err error
		*envPtr, err = eUtils.LoginToLocal()
		fmt.Println(*envPtr)
		eUtils.CheckError(config, err, true)
	}

	logger.Println("=============== Initializing Seed Generator ===============")

	logger.SetPrefix("[" + coreopts.GetFolderPrefix() + "x]")
	logger.Printf("Looking for template(s) in directory: %s\n", *startDirPtr)

	var subSectionName string
	if len(*eUtils.IndexNameFilterPtr) > 0 {
		subSectionName = *eUtils.IndexNameFilterPtr
	} else {
		subSectionName = ""
	}
	var waitg sync.WaitGroup
	sectionKey := "/"
	if len(envSlice) == 1 || (len(*eUtils.IndexValueFilterPtr) > 0 && len(*eUtils.IndexedPtr) > 0) {
		if strings.Contains(envSlice[0], "*") || len(*eUtils.IndexedPtr) > 0 || len(*eUtils.RestrictedPtr) > 0 || len(*eUtils.ProtectedPtr) > 0 {
			if len(*eUtils.IndexedPtr) > 0 {
				sectionKey = "/Index/"
			} else if len(*eUtils.RestrictedPtr) > 0 {
				sectionKey = "/Restricted/"
			} else if len(*eUtils.ProtectedPtr) > 0 {
				sectionKey = "/Protected/"
			}

			newSectionSlice := make([]string, 0)
			if !*noVaultPtr && !trcxe {
				var baseEnv string
				if strings.Contains(envSlice[0], "_") {
					baseEnv = strings.Split(envSlice[0], "_")[0]
				} else {
					baseEnv = envSlice[0]
				}
				//Ask vault for list of dev.<id>.* environments, add to envSlice
				authErr := eUtils.AutoAuth(&eUtils.DriverConfig{Insecure: *insecurePtr, Log: logger, ExitOnFailure: true}, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, &baseEnv, addrPtr, *pingPtr)
				if authErr != nil {
					eUtils.LogErrorMessage(config, "Auth failure: "+authErr.Error(), true)
				}
				testMod, err := helperkv.NewModifier(*insecurePtr, *tokenPtr, *addrPtr, baseEnv, regions, true, logger)
				testMod.Env = baseEnv
				if err != nil {
					logger.Printf(err.Error())
				}
				// Only look at index values....
				//Checks for indexed projects
				if len(*eUtils.IndexedPtr) > 0 {
					projectSectionsSlice = append(projectSectionsSlice, strings.Split(*eUtils.IndexedPtr, ",")...)
				}

				if len(*eUtils.RestrictedPtr) > 0 {
					projectSectionsSlice = append(projectSectionsSlice, strings.Split(*eUtils.RestrictedPtr, ",")...)
				}

				if len(*eUtils.ProtectedPtr) > 0 {
					projectSectionsSlice = append(projectSectionsSlice, strings.Split(*eUtils.ProtectedPtr, ",")...)
				}

				var listValues *api.Secret
				if len(projectSectionsSlice) > 0 { //If eid -> look inside Index and grab all environments
					subSectionPath := projectSectionsSlice[0] + "/"
					listValues, err = testMod.ListEnv("super-secrets/"+testMod.Env+sectionKey+subSectionPath, config.Log)
					if err != nil {
						if strings.Contains(err.Error(), "permission denied") {
							eUtils.LogErrorMessage(config, "Attempt to access restricted section of the vault denied.", true)
						}
					}

					// Further path modifications needed.
					if listValues == nil {
						eUtils.LogAndSafeExit(config, "No available indexes found for "+subSectionPath, 1)
					}
					for k, valuesPath := range listValues.Data {
						for _, indexNameInterface := range valuesPath.([]interface{}) {
							if indexNameInterface != (subSectionName + "/") {
								continue
							}
							indexList, err := testMod.ListEnv("super-secrets/"+testMod.Env+sectionKey+subSectionPath+"/"+indexNameInterface.(string), config.Log)
							if err != nil {
								logger.Printf(err.Error())
							}

							for _, indexPath := range indexList.Data {
								for _, indexInterface := range indexPath.([]interface{}) {
									if len(*eUtils.IndexValueFilterPtr) > 0 {
										if indexInterface != (*eUtils.IndexValueFilterPtr + "/") {
											continue
										}
									}
									newSectionSlice = append(newSectionSlice, strings.ReplaceAll(indexInterface.(string), "/", ""))
								}
							}
						}
						delete(listValues.Data, k) //delete it so it doesn't repeat below
					}
				} else {
					listValues, err = testMod.ListEnv("values/", config.Log)
				}
				if err != nil {
					logger.Printf(err.Error())
				}
				if len(newSectionSlice) > 0 {
					sectionSlice = newSectionSlice
				}
				if testMod != nil {
					testMod.Release()
				}
			} else { //novault takes this path
				if len(*eUtils.IndexedPtr) > 0 {
					projectSectionsSlice = append(projectSectionsSlice, strings.Split(*eUtils.IndexedPtr, ",")...)
				}

				if len(*eUtils.RestrictedPtr) > 0 {
					projectSectionsSlice = append(projectSectionsSlice, strings.Split(*eUtils.RestrictedPtr, ",")...)
				}

				if len(*eUtils.ProtectedPtr) > 0 {
					projectSectionsSlice = append(projectSectionsSlice, strings.Split(*eUtils.ProtectedPtr, ",")...)
				}
			}
		}
	}

	var filteredSectionSlice []string
	var serviceFilterSlice []string

	if len(*eUtils.IndexValueFilterPtr) > 0 {
		filterSlice := strings.Split(*eUtils.IndexValueFilterPtr, ",")
		for _, filter := range filterSlice {
			for _, section := range sectionSlice {
				if filter == section {
					filteredSectionSlice = append(filteredSectionSlice, section)
				}
			}
		}
		sectionSlice = filteredSectionSlice
	}
	if len(*eUtils.ServiceFilterPtr) > 0 {
		if len(sectionSlice) == 0 {
			eUtils.LogAndSafeExit(config, "No available indexes found for "+*eUtils.IndexValueFilterPtr, 1)
		}
		serviceFilterSlice = strings.Split(*eUtils.ServiceFilterPtr, ",")
		if len(*eUtils.ServiceNameFilterPtr) > 0 {
			*eUtils.ServiceNameFilterPtr = "/" + *eUtils.ServiceNameFilterPtr //added "/" - used path later
		}
	}

	go reciever() //Channel reciever
	for _, env := range envSlice {
		envVersion := eUtils.SplitEnv(env)
		*envPtr = envVersion[0]
		if secretIDPtr != nil && *secretIDPtr != "" && appRoleIDPtr != nil && *appRoleIDPtr != "" {
			*tokenPtr = ""
		}
		for _, section := range sectionSlice {
			var servicesWanted []string
			if !*noVaultPtr && *tokenPtr == "" {
				authErr := eUtils.AutoAuth(&eUtils.DriverConfig{Insecure: *insecurePtr, Log: logger, ExitOnFailure: true}, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, *pingPtr)
				if authErr != nil {
					// Retry once.
					authErr := eUtils.AutoAuth(&eUtils.DriverConfig{Insecure: *insecurePtr, Log: logger, ExitOnFailure: true}, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, *pingPtr)
					if authErr != nil {
						eUtils.LogAndSafeExit(config, fmt.Sprintf("Unexpected auth error %v ", authErr), 1)
					}
				}
			} else if *tokenPtr == "" {
				*tokenPtr = "novault"
			}
			if len(envVersion) >= 2 { //Put back env+version together
				*envPtr = envVersion[0] + "_" + envVersion[1]
			} else {
				*envPtr = envVersion[0] + "_0"
			}

			var trcxeList []string
			if trcxe {
				projectSectionsSlice = append(projectSectionsSlice, strings.Split(*eUtils.IndexedPtr, ",")...)

				trcxeList = append(trcxeList, *fieldsPtr)
				trcxeList = append(trcxeList, *encryptedPtr)
				if *noVaultPtr {
					trcxeList = append(trcxeList, "new")
				}
			}
			config := eUtils.DriverConfig{
				Context:         ctx,
				Insecure:        *insecurePtr,
				Token:           *tokenPtr,
				VaultAddress:    *addrPtr,
				EnvRaw:          envRaw,
				Env:             *envPtr,
				SectionKey:      sectionKey,
				SectionName:     subSectionName,
				SubSectionValue: section,
				SubSectionName:  *eUtils.ServiceNameFilterPtr,
				Regions:         regions,
				SecretMode:      *secretMode,
				ServicesWanted:  servicesWanted,
				StartDir:        append([]string{}, *startDirPtr),
				EndDir:          *endDirPtr,
				WantCerts:       *wantCertsPtr,
				GenAuth:         *genAuth,
				Log:             logger,
				Clean:           *cleanPtr,
				Diff:            *diffPtr,
				Update:          messenger,
				VersionInfo:     eUtils.VersionHelper,
				FileFilter:      fileFilter,
				ProjectSections: projectSectionsSlice,
				ServiceFilter:   serviceFilterSlice,
				ExitOnFailure:   true,
				Trcxe:           trcxeList,
				Trcxr:           *readOnlyPtr,
			}
			waitg.Add(1)
			go func() {
				defer waitg.Done()
				eUtils.ConfigControl(ctx, &config, configDriver)
			}()
		}
	}
	waitg.Wait()
	close(resultChannel)
	if *diffPtr { //Diff if needed
		waitg.Add(1)
		go func() {
			defer waitg.Done()
			retry := 0
			for {
				resultMapLock.Lock()
				if len(resultMap) == len(envSlice)*len(sectionSlice) || retry == 3 {
					resultMapLock.Unlock()
					break
				}
				resultMapLock.Unlock()
				time.Sleep(time.Duration(time.Second))
				retry++
			}
			eUtils.DiffHelper(resultMap, envLength, envSlice, -1, false, resultMapLock)
		}()
	}
	waitg.Wait() //Wait for diff

	logger.SetPrefix("[" + coreopts.GetFolderPrefix() + "x]")
	logger.Println("=============== Terminating Seed Generator ===============")
	logger.SetPrefix("[END]")
	logger.Println()
}

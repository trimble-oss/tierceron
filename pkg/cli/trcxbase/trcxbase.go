package trcxbase

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/pkg/core"
	"github.com/trimble-oss/tierceron/pkg/core/cache"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"

	"github.com/hashicorp/vault/api"
)

func messenger(configCtx *config.ConfigContext, inData *string, inPath string) {
	var data config.ResultData
	data.InData = inData
	data.InPath = inPath
	configCtx.ResultChannel <- &data
}

func receiver(configCtx *config.ConfigContext) {
	for {
		select {
		case data := <-configCtx.ResultChannel:
			if data != nil && data.InData != nil && data.InPath != "" {
				configCtx.Mutex.Lock()
				configCtx.ResultMap[data.InPath] = data.InData
				configCtx.Mutex.Unlock()
			}
		}
	}
}

// CommonMain This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func CommonMain(ctx config.ProcessContext,
	configDriver config.ConfigDriver,
	envPtr *string,
	addrPtrIn *string,
	envCtxPtr *string,
	insecurePtrIn *bool,
	flagset *flag.FlagSet,
	argLines []string) {
	// Executable input arguments(flags)
	if flagset == nil {
		flagset = flag.NewFlagSet(argLines[0], flag.ExitOnError)
		flagset.Usage = func() {
			fmt.Fprintf(flagset.Output(), "Usage of %s:\n", argLines[0])
			flagset.PrintDefaults()
		}
		flagset.String("env", "dev", "Environment to configure")
	}
	addrPtr := flagset.String("addr", "", "API endpoint for the vault")
	if addrPtrIn != nil && *addrPtrIn != "" {
		addrPtr = addrPtrIn
	}

	startDirPtr := flagset.String("startDir", coreopts.BuildOptions.GetFolderPrefix(nil)+"_templates", "Pull templates from this directory")
	endDirPtr := flagset.String("endDir", "./"+coreopts.BuildOptions.GetFolderPrefix(nil)+"_seeds/", "Write generated seed files to this directory")
	logFilePtr := flagset.String("log", "./"+coreopts.BuildOptions.GetFolderPrefix(nil)+"x.log", "Output path for log file")
	helpPtr := flagset.Bool("h", false, "Provide options for "+coreopts.BuildOptions.GetFolderPrefix(nil)+"x")
	tokenPtr := flagset.String("token", "", "Vault access token")
	secretMode := flagset.Bool("secretMode", true, "Only override secret values in templates?")
	genAuth := flagset.Bool("genAuth", false, "Generate auth section of seed data?")
	cleanPtr := flagset.Bool("clean", false, "Cleans seed files locally")
	secretIDPtr := flagset.String("secretID", "", "Secret for app role ID")
	appRoleIDPtr := flagset.String("appRoleID", "", "Public app role ID")
	tokenNamePtr := flagset.String("tokenName", "", "Token name used by this "+coreopts.BuildOptions.GetFolderPrefix(nil)+"x to access the vault")
	noVaultPtr := flagset.Bool("novault", false, "Don't pull configuration data from vault.")
	pingPtr := flagset.Bool("ping", false, "Ping vault.")

	fileAddrPtr := flagset.String("seedpath", "", "Path for seed file")
	fieldsPtr := flagset.String("fields", "", "Fields to enter")
	encryptedPtr := flagset.String("encrypted", "", "Fields to encrypt")
	readOnlyPtr := flagset.Bool("readonly", false, "Fields to encrypt")
	dynamicPathPtr := flagset.String("dynamicPath", "", "Generate seeds for a dynamic path in vault.")

	var insecurePtr *bool
	if insecurePtrIn == nil {
		insecurePtr = flagset.Bool("insecure", false, "By default, every ssl connection this tool makes is verified secure.  This option allows to tool to continue with server connections considered insecure.")
	} else {
		insecurePtr = insecurePtrIn
	}

	diffPtr := flagset.Bool("diff", false, "Diff files")
	versionPtr := flagset.Bool("versions", false, "Gets version metadata information")
	wantCertsPtr := flagset.Bool("certs", false, "Pull certificates into directory specified by endDirPtr")
	filterTemplatePtr := flagset.String("templateFilter", "", "Specifies which templates to filter") // -templateFilter=config.yml

	eUtils.CheckInitFlags(flagset)

	// Checks for proper flag input
	args := argLines[1:]
	for i := 0; i < len(args); i++ {
		s := args[i]
		if s[0] != '-' {
			fmt.Println("Wrong flag syntax: ", s)
			os.Exit(1)
		}
	}

	flagset.Parse(argLines[1:])
	configCtx := &config.ConfigContext{
		ResultMap:            make(map[string]*string),
		EnvSlice:             make([]string, 0),
		ProjectSectionsSlice: make([]string, 0),
		ResultChannel:        make(chan *config.ResultData, 5),
		FileSysIndex:         -1,
		ConfigWg:             sync.WaitGroup{},
		Mutex:                &sync.Mutex{},
	}

	if eUtils.RefLength(tokenNamePtr) == 0 && eUtils.RefLength(tokenPtr) > 0 {
		tokenName := fmt.Sprintf("config_token_%s", *envPtr)
		tokenNamePtr = &tokenName
	}
	driverConfigBase := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			Insecure:      *insecurePtr,
			TokenCache:    cache.NewTokenCache(*tokenNamePtr, tokenPtr),
			ExitOnFailure: true,
		},
	}

	// Initialize logging
	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if f != nil {
		// Terminate logging
		defer f.Close()
	}
	eUtils.CheckError(driverConfigBase.CoreConfig, err, true)
	logger := log.New(f, "["+coreopts.BuildOptions.GetFolderPrefix(nil)+"x]", log.LstdFlags)
	driverConfigBase.CoreConfig.Log = logger

	envBasis := *envPtr

	Yellow := "\033[33m"
	Reset := "\033[0m"
	if eUtils.IsWindows() {
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
	} else {
		if len(*dynamicPathPtr) == 0 {
			if (len(*eUtils.ServiceFilterPtr) == 0 || len(*eUtils.IndexNameFilterPtr) == 0) && len(*eUtils.IndexedPtr) != 0 {
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
			}
		}
	}

	trcxe := false
	sectionSlice := []string{""}
	if len(*fileAddrPtr) != 0 { //Checks if seed file exists & figured out if index/restricted
		trcxe = true
		directorySplit := strings.Split(*fileAddrPtr, "/")
		indexed := false
		if !*noVaultPtr {
			pwd, _ := os.Getwd()
			fileIndex, fileErr := os.Open(pwd + "/" + coreopts.BuildOptions.GetFolderPrefix(nil) + "_seeds/" + *envPtr + "/Index/" + *fileAddrPtr + "_seed.yml")
			if fileIndex != nil {
				defer fileIndex.Close()
			}
			if errors.Is(fileErr, os.ErrNotExist) {
				fileRestricted, fileRErr := os.Open(pwd + "/" + coreopts.BuildOptions.GetFolderPrefix(nil) + "_seeds/" + *envPtr + "/Restricted/" + *fileAddrPtr + "_seed.yml")
				if fileRestricted != nil {
					defer fileRestricted.Close()
				}
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
		configCtx.EnvSlice = append(configCtx.EnvSlice, *envPtr+"_versionInfo")
		goto skipDiff
	}

	//Diff flag parsing check
	if *diffPtr {
		if strings.ContainsAny(*envPtr, ",") { //Multiple environments
			*envPtr = strings.ReplaceAll(*envPtr, "latest", "0")
			configCtx.EnvSlice = strings.Split(*envPtr, ",")
			configCtx.EnvLength = len(configCtx.EnvSlice)
			if len(configCtx.EnvSlice) > 4 {
				fmt.Println("Unsupported number of environments - Maximum: 4")
				os.Exit(1)
			}
			for i, env := range configCtx.EnvSlice {
				if env == "local" {
					fmt.Println("Unsupported env: local not available with diff flag")
					os.Exit(1)
				}
				if !strings.Contains(env, "_") {
					configCtx.EnvSlice[i] = env + "_0"
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
		configCtx.EnvSlice = append(configCtx.EnvSlice, (*envPtr))
		envVersion := strings.Split(*envPtr, "_") //Break apart env+version for token
		*envPtr = envVersion[0]
		if !*noVaultPtr {
			appconfigrolePtr := new(string)
			autoErr := eUtils.AutoAuth(driverConfigBase, secretIDPtr, appRoleIDPtr, tokenNamePtr, envPtr, addrPtr, envCtxPtr, appconfigrolePtr, *pingPtr)

			if autoErr != nil {
				fmt.Println("Auth failure: " + autoErr.Error())
				eUtils.LogErrorMessage(driverConfigBase.CoreConfig, autoErr.Error(), true)
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

	//Duplicate env check
	for _, entry := range configCtx.EnvSlice {
		if _, value := keysCheck[entry]; !value {
			keysCheck[entry] = true
			listCheck = append(listCheck, entry)
		}
	}

	if len(listCheck) != len(configCtx.EnvSlice) {
		fmt.Printf("Cannot diff an environment against itself.\n")
		os.Exit(1)
	}

skipDiff:
	// Prints usage if no flags are specified
	if *helpPtr {
		flagset.Usage()
		os.Exit(1)
	}
	if ctx == nil {
		if _, err := os.Stat(*startDirPtr); os.IsNotExist(err) {
			fmt.Println("Missing required start template folder: " + *startDirPtr)
			os.Exit(1)
		}
		if !*diffPtr { // -diff doesn't require seed folder
			if _, err := os.Stat(*endDirPtr); os.IsNotExist(err) {
				fmt.Println("Missing required start seed folder: " + *endDirPtr)
				os.Exit(1)
			}
		}
	}

	// If logging production directory does not exist and is selected log to local directory
	if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/"+coreopts.BuildOptions.GetFolderPrefix(nil)+"x.log" {
		*logFilePtr = "./" + coreopts.BuildOptions.GetFolderPrefix(nil) + "x.log"
	}

	regions := []string{}

	if len(configCtx.EnvSlice) == 1 && !*noVaultPtr {
		if strings.HasPrefix(*envPtr, "staging") || strings.HasPrefix(*envPtr, "prod") {
			secretIDPtr = nil
			appRoleIDPtr = nil
		}
		if strings.HasPrefix(*envPtr, "staging") || strings.HasPrefix(*envPtr, "prod") || strings.HasPrefix(*envPtr, "dev") {
			regions = eUtils.GetSupportedProdRegions()
		}
		appconfigrolePtr := new(string)

		autoErr := eUtils.AutoAuth(&config.DriverConfig{
			CoreConfig: &core.CoreConfig{
				ExitOnFailure: true,
				TokenCache:    driverConfigBase.CoreConfig.TokenCache,
				Insecure:      *insecurePtr,
				Log:           logger,
			},
		}, secretIDPtr, appRoleIDPtr, tokenNamePtr, envPtr, addrPtr, envCtxPtr, appconfigrolePtr, *pingPtr)
		if autoErr != nil {
			fmt.Println("Missing auth components.")
			eUtils.LogErrorMessage(driverConfigBase.CoreConfig, autoErr.Error(), true)
		}
	}

	var env string
	if envPtr != nil {
		env = *envPtr
		if strings.Contains(*envPtr, "_") && len(configCtx.EnvSlice) == 1 {
			env = configCtx.EnvSlice[0]
		}
	}

	if len(configCtx.EnvSlice) == 1 && (eUtils.RefLength(driverConfigBase.CoreConfig.TokenCache.GetToken(fmt.Sprintf("config_token_%s", env))) == 0) && !*noVaultPtr {
		fmt.Printf("Missing required auth token for env: %s\n", env)
		os.Exit(1)
	}

	if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
		var err error
		*envPtr, err = eUtils.LoginToLocal()
		fmt.Println(*envPtr)
		eUtils.CheckError(driverConfigBase.CoreConfig, err, true)
	}

	logger.Println("=============== Initializing Seed Generator ===============")

	logger.SetPrefix("[" + coreopts.BuildOptions.GetFolderPrefix(nil) + "x]")
	logger.Printf("Looking for template(s) in directory: %s\n", *startDirPtr)

	var subSectionName string
	if len(*eUtils.IndexNameFilterPtr) > 0 {
		subSectionName = *eUtils.IndexNameFilterPtr
	} else {
		subSectionName = ""
	}
	var waitg sync.WaitGroup
	sectionKey := ""
	var serviceFilterSlice []string

	if len(*dynamicPathPtr) > 0 {
		go receiver(configCtx) //Channel receiver

		dynamicPathParts := strings.Split(*dynamicPathPtr, "/")

		for _, env := range configCtx.EnvSlice {
			envVersion := eUtils.SplitEnv(env)
			*envPtr = envVersion[0]
			if secretIDPtr != nil && *secretIDPtr != "" && appRoleIDPtr != nil && *appRoleIDPtr != "" {
				*tokenPtr = ""
			}
			var testMod *kv.Modifier = nil
			var baseEnv string

			if strings.Contains(*dynamicPathPtr, "%s") {
				if strings.Contains(configCtx.EnvSlice[0], "_") {
					baseEnv = strings.Split(configCtx.EnvSlice[0], "_")[0]
				} else {
					baseEnv = configCtx.EnvSlice[0]
				}
				if !*noVaultPtr && eUtils.RefLength(driverConfigBase.CoreConfig.TokenCache.GetToken(fmt.Sprintf("config_token_%s", *envPtr))) == 0 {
					//Ask vault for list of dev.<id>.* environments, add to envSlice
					appconfigrolePtr := new(string)

					authErr := eUtils.AutoAuth(&config.DriverConfig{
						CoreConfig: &core.CoreConfig{
							ExitOnFailure: true,
							TokenCache:    driverConfigBase.CoreConfig.TokenCache,
							Insecure:      *insecurePtr,
							Log:           logger,
						},
					}, secretIDPtr, appRoleIDPtr, tokenNamePtr, &baseEnv, addrPtr, envCtxPtr, appconfigrolePtr, *pingPtr)
					if authErr != nil {
						eUtils.LogErrorMessage(driverConfigBase.CoreConfig, "Auth failure: "+authErr.Error(), true)
					}
				}
			}

			// Look up and flush out any dynamic components.
			pathGen := ""
			var recursivePathBuilder func(testMod *kv.Modifier, pGen string, dynamicPathParts []string)

			recursivePathBuilder = func(testMod *kv.Modifier, pGen string, dynamicPathParts []string) {
				if len(dynamicPathParts) == 0 {
					if !*noVaultPtr && eUtils.RefLength(driverConfigBase.CoreConfig.TokenCache.GetToken(fmt.Sprintf("config_token_%s", *envPtr))) == 0 {
						appconfigrolePtr := new(string)

						authErr := eUtils.AutoAuth(&config.DriverConfig{
							CoreConfig: &core.CoreConfig{
								ExitOnFailure: true,
								TokenCache:    driverConfigBase.CoreConfig.TokenCache,
								Insecure:      *insecurePtr,
								Log:           logger,
							},
						}, secretIDPtr, appRoleIDPtr, tokenNamePtr, envPtr, addrPtr, envCtxPtr, appconfigrolePtr, *pingPtr)
						if authErr != nil {
							// Retry once.

							authErr := eUtils.AutoAuth(&config.DriverConfig{
								CoreConfig: &core.CoreConfig{
									ExitOnFailure: true,
									TokenCache:    driverConfigBase.CoreConfig.TokenCache,
									Insecure:      *insecurePtr,
									Log:           logger,
								},
							}, secretIDPtr, appRoleIDPtr, tokenNamePtr, envPtr, addrPtr, envCtxPtr, appconfigrolePtr, *pingPtr)
							if authErr != nil {
								eUtils.LogAndSafeExit(driverConfigBase.CoreConfig, fmt.Sprintf("Unexpected auth error %v ", authErr), 1)
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

					driverConfig := &config.DriverConfig{
						CoreConfig: &core.CoreConfig{
							WantCerts:         *wantCertsPtr,
							Insecure:          *insecurePtr,
							TokenCache:        driverConfigBase.CoreConfig.TokenCache,
							VaultAddressPtr:   addrPtr,
							EnvBasis:          envBasis,
							Env:               *envPtr,
							Regions:           regions,
							DynamicPathFilter: pGen,
							ExitOnFailure:     true,
							Log:               logger,
						},
						Context:       ctx,
						SecretMode:    *secretMode,
						StartDir:      append([]string{}, *startDirPtr),
						EndDir:        *endDirPtr,
						GenAuth:       *genAuth,
						Clean:         *cleanPtr,
						Diff:          *diffPtr,
						Update:        messenger,
						VersionInfo:   eUtils.VersionHelper,
						SubPathFilter: strings.Split(pGen, ","),
						FileFilter:    fileFilter,
						Trcxr:         *readOnlyPtr,
					}
					waitg.Add(1)
					go func(dc *config.DriverConfig) {
						defer waitg.Done()
						config.ConfigControl(ctx, configCtx, dc, configDriver)
					}(driverConfig)
					return
				}

				for i, dynamicPart := range dynamicPathParts {
					if dynamicPart == "%s" {
						if testMod == nil {
							testMod, err = kv.NewModifier(*insecurePtr, tokenPtr, addrPtr, baseEnv, regions, true, logger)
							testMod.Env = baseEnv
							if err != nil {
								eUtils.LogErrorMessage(driverConfigBase.CoreConfig, "Access to vault failure.", true)
							}
						}

						listValues, err := testMod.ListEnv("super-secrets/"+testMod.Env+"/"+pGen, driverConfigBase.CoreConfig.Log)
						if err != nil {
							if strings.Contains(err.Error(), "permission denied") {
								eUtils.LogErrorMessage(driverConfigBase.CoreConfig, fmt.Sprintf("Insufficient privileges accessing: %s", pGen), true)
							}
						}

						if listValues == nil {
							//							eUtils.LogInfo(config, fmt.Sprintf("Partial data with path: %s", "super-secrets/"+testMod.Env+"/"+pGen))
							return
						}
						levelPart := map[string]string{}
						for _, valuesPath := range listValues.Data {
							for _, indexNameInterface := range valuesPath.([]interface{}) {
								levelPart[strings.Trim(indexNameInterface.(string), "/")] = ""
							}
						}

						if len(dynamicPathParts) > i {
							for level := range levelPart {
								recursivePathBuilder(testMod, pGen+"/"+level, dynamicPathParts[i+1:])
							}
							return
						}
					} else {
						if len(pGen) > 0 {
							pGen = pGen + "/" + dynamicPart
						} else {
							pGen = pGen + dynamicPart
						}
					}
				}
				recursivePathBuilder(testMod, pGen, []string{})
			}

			recursivePathBuilder(testMod, pathGen, dynamicPathParts)

			if testMod != nil {
				testMod.Release()
			}
		}

	} else {
		sectionKey = "/"

		// TODO: Deprecated...
		// 1-800-ROIT
		if len(configCtx.EnvSlice) == 1 || (len(*eUtils.IndexValueFilterPtr) > 0 && len(*eUtils.IndexedPtr) > 0) {
			if strings.Contains(configCtx.EnvSlice[0], "*") || len(*eUtils.IndexedPtr) > 0 || len(*eUtils.RestrictedPtr) > 0 || len(*eUtils.ProtectedPtr) > 0 {
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
					if strings.Contains(configCtx.EnvSlice[0], "_") {
						baseEnv = strings.Split(configCtx.EnvSlice[0], "_")[0]
					} else {
						baseEnv = configCtx.EnvSlice[0]
					}
					appconfigrolePtr := new(string)

					//Ask vault for list of dev.<id>.* environments, add to envSlice
					authErr := eUtils.AutoAuth(&config.DriverConfig{
						CoreConfig: &core.CoreConfig{
							ExitOnFailure: true,
							TokenCache:    cache.NewTokenCache(fmt.Sprintf("config_token_%s", baseEnv), tokenPtr),
							Insecure:      *insecurePtr,
							Log:           logger,
						},
					}, secretIDPtr, appRoleIDPtr, tokenNamePtr, &baseEnv, addrPtr, envCtxPtr, appconfigrolePtr, *pingPtr)
					if authErr != nil {
						eUtils.LogErrorMessage(driverConfigBase.CoreConfig, "Auth failure: "+authErr.Error(), true)
					}
					testMod, err := kv.NewModifier(*insecurePtr, tokenPtr, addrPtr, baseEnv, regions, true, logger)
					testMod.Env = baseEnv
					if err != nil {
						logger.Printf(err.Error())
					}
					// Only look at index values....
					//Checks for indexed projects
					if len(*eUtils.IndexedPtr) > 0 {
						configCtx.ProjectSectionsSlice = append(configCtx.ProjectSectionsSlice, strings.Split(*eUtils.IndexedPtr, ",")...)
					}

					if len(*eUtils.RestrictedPtr) > 0 {
						configCtx.ProjectSectionsSlice = append(configCtx.ProjectSectionsSlice, strings.Split(*eUtils.RestrictedPtr, ",")...)
					}

					if len(*eUtils.ProtectedPtr) > 0 {
						configCtx.ProjectSectionsSlice = append(configCtx.ProjectSectionsSlice, strings.Split(*eUtils.ProtectedPtr, ",")...)
					}

					var listValues *api.Secret
					if len(configCtx.ProjectSectionsSlice) > 0 { //If eid -> look inside Index and grab all environments
						subSectionPath := configCtx.ProjectSectionsSlice[0] + "/"
						listValues, err = testMod.ListEnv("super-secrets/"+testMod.Env+sectionKey+subSectionPath, driverConfigBase.CoreConfig.Log)
						if err != nil {
							if strings.Contains(err.Error(), "permission denied") {
								eUtils.LogErrorMessage(driverConfigBase.CoreConfig, "Attempt to access restricted section of the vault denied.", true)
							}
						}

						// Further path modifications needed.
						if listValues == nil {
							eUtils.LogAndSafeExit(driverConfigBase.CoreConfig, "No available indexes found for "+subSectionPath, 1)
						}
						for k, valuesPath := range listValues.Data {
							for _, indexNameInterface := range valuesPath.([]interface{}) {
								if indexNameInterface != (subSectionName + "/") {
									continue
								}
								indexList, err := testMod.ListEnv("super-secrets/"+testMod.Env+sectionKey+subSectionPath+"/"+indexNameInterface.(string), driverConfigBase.CoreConfig.Log)
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
						listValues, err = testMod.ListEnv("values/", driverConfigBase.CoreConfig.Log)
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
						configCtx.ProjectSectionsSlice = append(configCtx.ProjectSectionsSlice, strings.Split(*eUtils.IndexedPtr, ",")...)
					}

					if len(*eUtils.RestrictedPtr) > 0 {
						configCtx.ProjectSectionsSlice = append(configCtx.ProjectSectionsSlice, strings.Split(*eUtils.RestrictedPtr, ",")...)
					}

					if len(*eUtils.ProtectedPtr) > 0 {
						configCtx.ProjectSectionsSlice = append(configCtx.ProjectSectionsSlice, strings.Split(*eUtils.ProtectedPtr, ",")...)
					}
				}
			}
		}

		var filteredSectionSlice []string

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
				eUtils.LogAndSafeExit(driverConfigBase.CoreConfig, "No available indexes found for "+*eUtils.IndexValueFilterPtr, 1)
			}
			serviceFilterSlice = strings.Split(*eUtils.ServiceFilterPtr, ",")
			if len(*eUtils.ServiceNameFilterPtr) > 0 {
				*eUtils.ServiceNameFilterPtr = "/" + *eUtils.ServiceNameFilterPtr //added "/" - used path later
			}
		}
	}

	go receiver(configCtx) //Channel receiver
	if len(*dynamicPathPtr) == 0 {
		for _, env := range configCtx.EnvSlice {
			envVersion := eUtils.SplitEnv(env)
			*envPtr = envVersion[0]
			for _, section := range sectionSlice {
				var servicesWanted []string
				if !*noVaultPtr {
					appconfigrolePtr := new(string)

					authErr := eUtils.AutoAuth(&config.DriverConfig{
						CoreConfig: &core.CoreConfig{
							ExitOnFailure: true,
							TokenCache:    driverConfigBase.CoreConfig.TokenCache,
							Insecure:      *insecurePtr,
							Log:           logger,
						},
					}, secretIDPtr, appRoleIDPtr, tokenNamePtr, envPtr, addrPtr, envCtxPtr, appconfigrolePtr, *pingPtr)
					if authErr != nil {
						// Retry once.
						authErr := eUtils.AutoAuth(&config.DriverConfig{
							CoreConfig: &core.CoreConfig{
								ExitOnFailure: true,
								TokenCache:    driverConfigBase.CoreConfig.TokenCache,
								Insecure:      *insecurePtr,
								Log:           logger,
							},
						}, secretIDPtr, appRoleIDPtr, tokenNamePtr, envPtr, addrPtr, envCtxPtr, appconfigrolePtr, *pingPtr)
						if authErr != nil {
							eUtils.LogAndSafeExit(driverConfigBase.CoreConfig, fmt.Sprintf("Unexpected auth error %v ", authErr), 1)
						}
					}
				} else if eUtils.RefLength(driverConfigBase.CoreConfig.TokenCache.GetToken(fmt.Sprintf("config_token_%s", *envPtr))) == 0 {
					token := "novault"
					driverConfigBase.CoreConfig.TokenCache.AddToken(fmt.Sprintf("config_token_%s", *envPtr), &token)
				}
				if len(envVersion) >= 2 { //Put back env+version together
					*envPtr = envVersion[0] + "_" + envVersion[1]
				} else {
					*envPtr = envVersion[0] + "_0"
				}

				var trcxeList []string
				if trcxe {
					configCtx.ProjectSectionsSlice = append(configCtx.ProjectSectionsSlice, strings.Split(*eUtils.IndexedPtr, ",")...)

					trcxeList = append(trcxeList, *fieldsPtr)
					trcxeList = append(trcxeList, *encryptedPtr)
					if *noVaultPtr {
						trcxeList = append(trcxeList, "new")
					}
				}
				driverConfig := &config.DriverConfig{
					CoreConfig: &core.CoreConfig{
						WantCerts:         *wantCertsPtr,
						Insecure:          *insecurePtr,
						TokenCache:        driverConfigBase.CoreConfig.TokenCache,
						VaultAddressPtr:   addrPtr,
						Regions:           regions,
						EnvBasis:          envBasis,
						Env:               *envPtr,
						DynamicPathFilter: *dynamicPathPtr,
						ExitOnFailure:     true,
						Log:               logger,
					},
					Context:         ctx,
					SectionKey:      sectionKey,
					SectionName:     subSectionName,
					SubSectionValue: section,
					SubSectionName:  *eUtils.ServiceNameFilterPtr,
					SecretMode:      *secretMode,
					ServicesWanted:  servicesWanted,
					StartDir:        append([]string{}, *startDirPtr),
					EndDir:          *endDirPtr,
					GenAuth:         *genAuth,
					Clean:           *cleanPtr,
					Diff:            *diffPtr,
					Update:          messenger,
					VersionInfo:     eUtils.VersionHelper,
					FileFilter:      fileFilter,
					SubPathFilter:   strings.Split(*eUtils.SubPathFilter, ","),
					ProjectSections: configCtx.ProjectSectionsSlice,
					ServiceFilter:   serviceFilterSlice,
					Trcxe:           trcxeList,
					Trcxr:           *readOnlyPtr,
				}
				waitg.Add(1)
				go func(dc *config.DriverConfig) {
					defer waitg.Done()
					config.ConfigControl(ctx, configCtx, dc, configDriver)
				}(driverConfig)
			}
		}
	}

	waitg.Wait()
	close(configCtx.ResultChannel)
	if *diffPtr { //Diff if needed
		waitg.Add(1)
		go func(cctx *config.ConfigContext) {
			defer waitg.Done()
			retry := 0
			for {
				cctx.Mutex.Lock()
				if len(cctx.ResultMap) == len(cctx.EnvSlice)*len(sectionSlice) || retry == 3 {
					cctx.Mutex.Unlock()
					break
				}
				cctx.Mutex.Unlock()
				time.Sleep(time.Duration(time.Second))
				retry++
			}
			configCtx.FileSysIndex = -1
			cctx.SetDiffFileCount(len(configCtx.ResultMap) / configCtx.EnvLength)
			eUtils.DiffHelper(cctx, false)
		}(configCtx)
	}
	waitg.Wait() //Wait for diff

	logger.SetPrefix("[" + coreopts.BuildOptions.GetFolderPrefix(nil) + "x]")
	logger.Println("=============== Terminating Seed Generator ===============")
	logger.SetPrefix("[END]")
	logger.Println()
}

package xutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	vcutils "tierceron/trcconfig/utils"
	xdb "tierceron/trcx/db"
	"tierceron/trcx/extract"
	"tierceron/utils"
	eUtils "tierceron/utils"
	"tierceron/vaulthelper/kv"

	"gopkg.in/yaml.v2"
)

var wg sync.WaitGroup
var wg2 sync.WaitGroup

var templateResultChan = make(chan *extract.TemplateResultData, 5)

// GenerateSeedsFromVaultRaw configures the templates in trc_templates and writes them to trcx
func GenerateSeedsFromVaultRaw(config eUtils.DriverConfig, fromVault bool, templatePaths []string) (string, bool, string) {
	// Initialize global variables
	valueCombinedSection := map[string]map[string]map[string]string{}
	valueCombinedSection["values"] = map[string]map[string]string{}

	secretCombinedSection := map[string]map[string]map[string]string{}
	secretCombinedSection["super-secrets"] = map[string]map[string]string{}

	// Declare local variables
	templateCombinedSection := map[string]interface{}{}
	sliceTemplateSection := []interface{}{}
	sliceValueSection := []map[string]map[string]map[string]string{}
	sliceSecretSection := []map[string]map[string]map[string]string{}
	maxDepth := -1

	endPath := ""
	multiService := false
	var mod *kv.Modifier
	noVault := false

	envVersion := strings.Split(config.Env, "_")
	if len(envVersion) != 2 {
		// Make it so.
		config.Env = config.Env + "_0"
		envVersion = strings.Split(config.Env, "_")
	}
	env := envVersion[0]
	version := envVersion[1]

	if config.Token != "" {
		var err error
		mod, err = kv.NewModifier(config.Insecure, config.Token, config.VaultAddress, env, config.Regions)
		if err != nil {
			panic(err)
		}
		mod.Env = env
		mod.Version = version
		if config.Token == "novault" {
			noVault = true
		}
	}

	if config.GenAuth && mod != nil {
		_, err := mod.ReadData("apiLogins/meta")
		if err != nil {
			fmt.Println("Cannot genAuth with provided token.")
			os.Exit(1)
		}
	}

	templateVersionMap := make(map[string]map[string]interface{})
	if mod.Version != "0" {
		mod.ProjectVersionFilter = config.VersionProjectFilter
		templatePathMap, err := mod.GetVersionValues(mod, "super-secrets") //Needs filter
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		var lastKey string
		for key, value := range templatePathMap {
			if len(config.VersionProjectFilter) > 0 && !strings.HasSuffix(key, config.VersionProjectFilter[0]) {
				lastKey = key
				continue
			} else {
				templateVersionMap[key] = value
				lastKey = key
			}
		}

		Cyan := "\033[36m"
		Reset := "\033[0m"
		if runtime.GOOS == "windows" {
			Reset = ""
			Cyan = ""
		}

		if templateVersionMap == nil {
			fmt.Println("No version data found - this filter was applied during search: ", config.VersionProjectFilter)
			os.Exit(1)
		} else if version == "versionInfo" {
			if templateVersionMap == nil {
				config.VersionInfo(templateVersionMap[lastKey], false, "")
			} else {
				config.VersionInfo(templatePathMap[lastKey], false, "")
			}
			os.Exit(1)
		} else {
			var versions []string //Check available version bounds for regular diff or config
			for version := range templatePathMap[lastKey] {
				versions = append(versions, version)
				sort.Slice(versions, func(i, j int) bool {
					numA, _ := strconv.Atoi(versions[i])
					numB, _ := strconv.Atoi(versions[j])
					return numA < numB
				})
			}

			if len(versions) >= 1 {
				latestVersion, _ := strconv.Atoi(versions[len(versions)-1])
				oldestVersion, _ := strconv.Atoi(versions[0])
				userVersion, _ := strconv.Atoi(version)
				if userVersion > latestVersion || userVersion < oldestVersion && len(versions) != 1 {
					fmt.Println(Cyan + "This version " + env + "_" + version + " is not available as the latest version is " + versions[len(versions)-1] + " and oldest version available is " + versions[0] + Reset)
					os.Exit(1)
				}
			} else {
				fmt.Println(Cyan + "No version data found" + Reset)
				os.Exit(1)
			}
		}
	}

	//Reciever for configs
	go func(c eUtils.DriverConfig) {
		for {
			select {
			case tResult := <-templateResultChan:
				if config.Env == tResult.Env {
					sliceTemplateSection = append(sliceTemplateSection, tResult.InterfaceTemplateSection)
					sliceValueSection = append(sliceValueSection, tResult.ValueSection)
					sliceSecretSection = append(sliceSecretSection, tResult.SecretSection)
					if tResult.TemplateDepth > maxDepth {
						maxDepth = tResult.TemplateDepth
						//templateCombinedSection = interfaceTemplateSection
					}
					wg.Done()
				} else {
					go func(tResult *extract.TemplateResultData) {
						templateResultChan <- tResult
					}(tResult)
				}
			default:
			}
		}
	}(config)

	commonPaths := []string{}
	if config.Token != "" {
		var commonMod *kv.Modifier
		var err error
		commonMod, err = kv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, config.Regions)
		if err != nil {
			panic(err)
		}
		envVersion := strings.Split(config.Env, "_")
		commonMod.Env = envVersion[0]
		commonMod.Version = envVersion[1]
		commonMod.Version = commonMod.Version + "***X-Mode"

		commonPaths, err = vcutils.GetPathsFromProject(commonMod, "Common")
		if len(commonPaths) > 0 && strings.Contains(commonPaths[len(commonPaths)-1], "!=!") {
			commonPaths = commonPaths[:len(commonPaths)-1]
		}
		commonMod.Close()
	}
	serviceFound := false
	// Configure each template in directory
	for _, templatePath := range templatePaths {
		var service string
		_, service, templatePath = vcutils.GetProjectService(templatePath)
		//This checks whether a enterprise env has the relevant project otherwise env gets skipped when generating seed files.
		if strings.Contains(mod.Env, ".") && !serviceFound {
			listValues, err := mod.ListEnv("values/" + mod.Env + "/") //Fix values to add to project to directory
			if err != nil {
				fmt.Println(err)
			}
			serviceSlice := make([]string, 0)
			for _, valuesPath := range listValues.Data {
				for _, envInterface := range valuesPath.([]interface{}) {
					env := envInterface.(string)
					serviceSlice = append(serviceSlice, env)
				}
			}
			for _, listedService := range serviceSlice {
				if strings.Contains(listedService, service) {
					serviceFound = true
				}
			}
		}
	}

	if !serviceFound { //Exit for irrelevant enterprises
		return "", false, ""
	}
	// Configure each template in directory
	for _, templatePath := range templatePaths {
		wg.Add(1)
		go func(templatePath string, multiService bool, c eUtils.DriverConfig, noVault bool) {
			project := ""
			service := ""

			// Map Subsections
			var templateResult extract.TemplateResultData

			templateResult.ValueSection = map[string]map[string]map[string]string{}
			templateResult.ValueSection["values"] = map[string]map[string]string{}

			templateResult.SecretSection = map[string]map[string]map[string]string{}
			templateResult.SecretSection["super-secrets"] = map[string]map[string]string{}

			var goMod *kv.Modifier

			if c.Token != "" {
				var err error
				goMod, err = kv.NewModifier(c.Insecure, c.Token, c.VaultAddress, c.Env, c.Regions)
				if err != nil {
					panic(err)
				}
				envVersion := strings.Split(config.Env, "_")
				goMod.Env = envVersion[0]
				goMod.Version = envVersion[1]
			}

			if c.GenAuth && goMod != nil {
				_, err := mod.ReadData("apiLogins/meta")
				if err != nil {
					fmt.Println("Cannot genAuth with provided token.")
					os.Exit(1)
				}
			}

			//check for template_files directory here
			project, service, templatePath = vcutils.GetProjectService(templatePath)

			requestedVersion := goMod.Version
			var cds *vcutils.ConfigDataStore
			if goMod != nil && !noVault {
				cds = new(vcutils.ConfigDataStore)
				goMod.Version = goMod.Version + "***X-Mode"
				cds.Init(goMod, c.SecretMode, true, project, commonPaths, service)
			}

			innerProject := "Not Found"
			if len(goMod.ProjectVersionFilter) >= 1 && strings.Contains(goMod.ProjectVersionFilter[len(goMod.ProjectVersionFilter)-1], "!=!") {
				innerProject = strings.Split(goMod.ProjectVersionFilter[len(goMod.ProjectVersionFilter)-1], "!=!")[1]
				goMod.ProjectVersionFilter = goMod.ProjectVersionFilter[:len(goMod.ProjectVersionFilter)-1]
			}
			if innerProject != "Not Found" {
				project = innerProject
				service = project
			}

			_, _, _, templateResult.TemplateDepth = extract.ToSeed(goMod,
				cds,
				templatePath,
				config.Log,
				project,
				service,
				fromVault,
				&(templateResult.InterfaceTemplateSection),
				&(templateResult.ValueSection),
				&(templateResult.SecretSection),
			)
			templateResult.Env = goMod.Env + "_" + requestedVersion
			templateResultChan <- &templateResult
		}(templatePath, multiService, config, noVault)
	}
	wg.Wait()

	// Combine values of slice
	combineSection(sliceTemplateSection, maxDepth, templateCombinedSection)
	combineSection(sliceValueSection, -1, valueCombinedSection)
	combineSection(sliceSecretSection, -1, secretCombinedSection)

	var authYaml []byte
	var errA error

	// Add special auth section.
	if config.GenAuth {
		if mod != nil {
			connInfo, err := mod.ReadData("apiLogins/meta")
			if err == nil {
				authSection := map[string]interface{}{}
				authSection["apiLogins"] = map[string]interface{}{}
				authSection["apiLogins"].(map[string]interface{})["meta"] = connInfo
				authYaml, errA = yaml.Marshal(authSection)
				if errA != nil {
					fmt.Println(errA)
				}
			} else {
				fmt.Println("Attempt to gen auth for reduced privilege token failed.  No permissions to gen auth.")
				os.Exit(1)
			}
		} else {
			authConfigurations := map[string]interface{}{}
			authConfigurations["authEndpoint"] = "<Enter Secret Here>"
			authConfigurations["pass"] = "<Enter Secret Here>"
			authConfigurations["sessionDB"] = "<Enter Secret Here>"
			authConfigurations["user"] = "<Enter Secret Here>"
			authConfigurations["trcAPITokenSecret"] = "<Enter Secret Here>"

			authSection := map[string]interface{}{}
			authSection["apiLogins"] = map[string]interface{}{}
			authSection["apiLogins"].(map[string]interface{})["meta"] = authConfigurations
			authYaml, errA = yaml.Marshal(authSection)
			if errA != nil {
				fmt.Println(errA)
			}
		}
	}

	// Create seed file structure
	template, errT := yaml.Marshal(templateCombinedSection)
	value, errV := yaml.Marshal(valueCombinedSection)
	secret, errS := yaml.Marshal(secretCombinedSection)

	if errT != nil {
		fmt.Println(errT)
	}

	if errV != nil {
		fmt.Println(errV)
	}

	if errS != nil {
		fmt.Println(errS)
	}
	templateData := string(template)
	// Remove single quotes generated by Marshal
	templateData = strings.ReplaceAll(templateData, "'", "")
	seedData := templateData + "\n\n\n" + string(value) + "\n\n\n" + string(secret) + "\n\n\n" + string(authYaml)

	return endPath, multiService, seedData
}

// GenerateSeedsFromVault configures the templates in trc_templates and writes them to trcx
func GenerateSeedsFromVault(ctx eUtils.ProcessContext, config eUtils.DriverConfig) interface{} {
	if config.Clean { //Clean flag in trcx
		if strings.HasSuffix(config.Env, "_0") {
			config.Env = strings.Split(config.Env, "_")[0]
		}
		_, err1 := os.Stat(config.EndDir + config.Env)
		err := os.RemoveAll(config.EndDir + config.Env)

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err1 == nil {
			fmt.Println("Seed removed from", config.EndDir+config.Env)
		}
		return nil
	}

	// Get files from directory
	tempTemplatePaths := []string{}
	for _, startDir := range config.StartDir {
		//get files from directory
		tp := getDirFiles(startDir)
		tempTemplatePaths = append(tempTemplatePaths, tp...)
	}

	//Duplicate path remover
	keys := make(map[string]bool)
	templatePaths := []string{}
	for _, path := range tempTemplatePaths {
		if _, value := keys[path]; !value {
			keys[path] = true
			templatePaths = append(templatePaths, path)
		}
	}

	endPath, multiService, seedData := GenerateSeedsFromVaultRaw(config, false, templatePaths)

	if endPath == "" && !multiService && seedData == "" {
		return nil
	}

	if strings.HasSuffix(config.Env, "_0") {
		config.Env = strings.Split(config.Env, "_")[0]
	}

	envBasePath, _, _ := kv.PreCheckEnvironment(config.Env)

	if multiService {
		if strings.HasPrefix(config.Env, "local") {
			endPath = config.EndDir + "local/local_seed.yml"
		} else {
			endPath = config.EndDir + envBasePath + "/" + config.Env + "_seed.yml"
		}
	} else {
		endPath = config.EndDir + envBasePath + "/" + config.Env + "_seed.yml"
	}

	//generate template or certificate
	if config.WantCerts {
		var certData map[int]string
		certLoaded := false

		for _, templatePath := range templatePaths {

			project, service, templatePath := vcutils.GetProjectService(templatePath)

			envVersion := strings.Split(config.Env, "_")
			if len(envVersion) != 2 {
				// Make it so.
				config.Env = config.Env + "_0"
				envVersion = strings.Split(config.Env, "_")
			}

			mod, err := kv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, config.Regions)
			if err != nil {
				panic(err)
			}
			mod.Env = envVersion[0]
			mod.Version = envVersion[1]

			_, certData, certLoaded = vcutils.ConfigTemplate(mod, templatePath, config.SecretMode, project, service, config.WantCerts, false)

			if len(certData) == 0 {
				if certLoaded {
					fmt.Println("Could not load cert ", templatePath)
					continue
				} else {
					continue
				}
			}

			certPath := fmt.Sprintf("%s", certData[2])
			fmt.Println("Writing certificate: " + certPath + ".")

			if strings.Contains(certPath, "ENV") {
				if len(mod.Env) >= 5 && (mod.Env)[:5] == "local" {
					envParts := strings.SplitN(mod.Env, "/", 3)
					certPath = strings.Replace(certPath, "ENV", envParts[1], 1)
				} else {
					certPath = strings.Replace(certPath, "ENV", mod.Env, 1)
				}
			}

			certDestination := config.EndDir + "/" + certPath
			writeToFile(certData[1], certDestination)
			fmt.Println("certificate written to ", certDestination)
		}
		return nil
	}

	if config.Diff {
		config.Update(&seedData, envBasePath+"||"+config.Env+"_seed.yml")
	} else {
		writeToFile(seedData, endPath)
		// Print that we're done
		fmt.Println("Seed created and written to " + strings.Replace(config.EndDir, "\\", "/", -1) + envBasePath + string(os.PathSeparator) + config.Env + "_seed.yml")
	}

	return nil
}

// GenerateSeedsFromVaultToDb pulls all data from vault for each template into a database
func GenerateSeedsFromVaultToDb(config eUtils.DriverConfig) interface{} {
	if config.Diff { //Clean flag in trcx
		_, err1 := os.Stat(config.EndDir + config.Env)
		err := os.RemoveAll(config.EndDir + config.Env)

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err1 == nil {
			fmt.Println("Seed removed from", config.EndDir+config.Env)
		}
		return nil
	}

	// Get files from directory
	tempTemplatePaths := []string{}
	for _, startDir := range config.StartDir {
		//get files from directory
		tp := getDirFiles(startDir)
		tempTemplatePaths = append(tempTemplatePaths, tp...)
	}

	//Duplicate path remover
	keys := make(map[string]bool)
	templatePaths := []string{}
	for _, path := range tempTemplatePaths {
		if _, value := keys[path]; !value {
			keys[path] = true
			templatePaths = append(templatePaths, path)
		}
	}

	tierceronEngine, err := xdb.CreateEngine(config,
		templatePaths)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return tierceronEngine
}

func writeToFile(data string, path string) {
	byteData := []byte(data)
	//Ensure directory has been created
	dirPath := filepath.Dir(path)
	err := os.MkdirAll(dirPath, os.ModePerm)
	utils.CheckError(err, true)
	//create new file
	newFile, err := os.Create(path)
	utils.CheckError(err, true)
	//write to file
	_, err = newFile.Write(byteData)
	utils.CheckError(err, true)
	err = newFile.Sync()
	utils.CheckError(err, true)
	newFile.Close()
}

func getDirFiles(dir string) []string {
	files, err := ioutil.ReadDir(dir)
	filePaths := []string{}
	//endPaths := []string{}
	if err != nil {
		//this is a file
		return []string{dir}
	}
	for _, file := range files {
		//add this directory to path names
		filename := file.Name()
		if strings.HasSuffix(filename, ".DS_Store") {
			continue
		}
		extension := filepath.Ext(filename)
		filePath := dir + file.Name()
		if !strings.HasSuffix(dir, "/") {
			filePath = dir + "/" + file.Name()
		}
		if extension == "" {
			//if subfolder add /
			filePath += "/"
		}
		//recurse to next level
		newPaths := getDirFiles(filePath)
		filePaths = append(filePaths, newPaths...)
	}
	return filePaths
}

// MergeMaps - merges 2 maps recursively.
func MergeMaps(x1, x2 interface{}) interface{} {
	switch x1 := x1.(type) {
	case map[string]interface{}:
		x2, ok := x2.(map[string]interface{})
		if !ok {
			return x1
		}
		for k, v2 := range x2 {
			if v1, ok := x1[k]; ok {
				x1[k] = MergeMaps(v1, v2)
			} else {
				x1[k] = v2
			}
		}
	case nil:
		x2, ok := x2.(map[string]interface{})
		if ok {
			return x2
		}
	}
	return x1
}

// Combines the values in a slice, creating a singular map from multiple
// Input:
//	- slice to combine
//	- template slice to combine
//	- depth of map (-1 for value/secret sections)
func combineSection(sliceSectionInterface interface{}, maxDepth int, combinedSectionInterface interface{}) {
	_, okMap := sliceSectionInterface.([]map[string]map[string]map[string]string)

	// Value/secret slice section
	if maxDepth < 0 && okMap {
		sliceSection := sliceSectionInterface.([]map[string]map[string]map[string]string)
		combinedSectionImpl := combinedSectionInterface.(map[string]map[string]map[string]string)
		for _, v := range sliceSection {
			for k2, v2 := range v {
				for k3, v3 := range v2 {
					if _, ok := combinedSectionImpl[k2][k3]; !ok {
						combinedSectionImpl[k2][k3] = map[string]string{}
					}
					for k4, v4 := range v3 {
						combinedSectionImpl[k2][k3][k4] = v4
					}
				}
			}
		}

		combinedSectionInterface = combinedSectionImpl

		// template slice section
	} else {
		if maxDepth < 0 && !okMap {
			fmt.Printf("Env failed to gen.  MaxDepth: %d, okMap: %t\n", maxDepth, okMap)
		}
		sliceSection := sliceSectionInterface.([]interface{})

		for _, v := range sliceSection {
			MergeMaps(combinedSectionInterface, v)
		}
	}
}

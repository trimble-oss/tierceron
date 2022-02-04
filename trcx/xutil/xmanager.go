package xutil

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	vcutils "tierceron/trcconfig/utils"
	"tierceron/trcx/extract"
	"tierceron/utils"
	eUtils "tierceron/utils"
	"tierceron/vaulthelper/kv"

	"github.com/hashicorp/vault/api"
	"gopkg.in/yaml.v2"
)

var wg sync.WaitGroup
var wg2 sync.WaitGroup

var templateResultChan = make(chan *extract.TemplateResultData, 5)

// GenerateSeedsFromVaultRaw configures the templates in trc_templates and writes them to trcx
func GenerateSeedsFromVaultRaw(config eUtils.DriverConfig, fromVault bool, templatePaths []string, logger *log.Logger) (string, bool, string) {
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

	filteredTemplatePaths := templatePaths[:0]
	if len(config.FileFilter) != 0 {
		for _, filter := range config.FileFilter {
			if !strings.HasSuffix(filter, ".tmpl") {
				filter = filter + ".tmpl"
			}
			for _, templatePath := range templatePaths {
				if strings.HasSuffix(templatePath, filter) {
					filteredTemplatePaths = append(filteredTemplatePaths, templatePath)
				}
			}
		}
	}
	if len(filteredTemplatePaths) > 0 {
		templatePaths = filteredTemplatePaths
	}

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
		mod, err = kv.NewModifier(config.Insecure, config.Token, config.VaultAddress, env, config.Regions, logger)
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
			eUtils.LogInfo("Cannot genAuth with provided token.", logger)
			os.Exit(1)
		}
	}

	if mod.Version != "0" { //If version isn't latest or is a flag
		var noCertPaths []string
		var certPaths []string
		for _, templatePath := range templatePaths { //Seperate cert vs normal paths
			if !strings.Contains(templatePath, "Common") {
				noCertPaths = append(noCertPaths, templatePath)
			} else {
				certPaths = append(certPaths, templatePath)
			}
		}

		if config.WantCerts { //Remove unneeded template paths
			templatePaths = certPaths
		} else {
			templatePaths = noCertPaths
		}

		project := ""
		if len(config.VersionFilter) > 0 {
			project = config.VersionFilter[0]
		}
		for _, templatePath := range templatePaths {
			_, service, _ := utils.GetProjectService(templatePath) //This checks for nested project names

			config.VersionFilter = append(config.VersionFilter, service) //Adds nested project name to filter otherwise it will be not found.
		}

		if config.WantCerts { //For cert version history
			config.VersionFilter = append(config.VersionFilter, "Common")
		}

		config.VersionFilter = utils.RemoveDuplicates(config.VersionFilter)
		mod.VersionFilter = config.VersionFilter
		versionMetadataMap := utils.GetProjectVersionInfo(config, mod, logger)

		if versionMetadataMap == nil {
			eUtils.LogInfo(fmt.Sprintf("No version data found - this filter was applied during search: %v\n", config.VersionFilter), logger)
			os.Exit(1)
		} else if version == "versionInfo" { //Version flag
			var masterKey string
			first := true
			for key := range versionMetadataMap {
				passed := false
				if config.WantCerts {
					for _, service := range mod.VersionFilter {
						if !passed && strings.Contains(key, "Common") && strings.Contains(key, service) && !strings.Contains(key, project) && !strings.HasSuffix(key, "Common") {
							if len(key) > 0 {
								keySplit := strings.Split(key, "/")
								config.VersionInfo(versionMetadataMap[key], false, keySplit[len(keySplit)-1], first)
								passed = true
								first = false
							}
						}
					}
				} else {
					if len(key) > 0 && len(masterKey) < 1 {
						masterKey = key
						config.VersionInfo(versionMetadataMap[masterKey], false, "", false)
						os.Exit(1)
					}
				}
			}
			os.Exit(1)
		} else { //Version bound check
			versionNumbers := utils.GetProjectVersions(config, versionMetadataMap)
			utils.BoundCheck(config, versionNumbers, version)
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
		commonMod, err = kv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, config.Regions, logger)
		if err != nil {
			panic(err)
		}
		envVersion := strings.Split(config.Env, "_")
		commonMod.Env = envVersion[0]
		commonMod.Version = envVersion[1]
		commonMod.Version = commonMod.Version + "***X-Mode"

		commonPaths, err = vcutils.GetPathsFromProject(commonMod, logger, "Common")
		if len(commonPaths) > 0 && strings.Contains(commonPaths[len(commonPaths)-1], "!=!") {
			commonPaths = commonPaths[:len(commonPaths)-1]
		}
		commonMod.Close()
	}

	// Configure each template in directory
	if strings.Contains(config.EnvRaw, ".*") {
		serviceFound := false
		for _, templatePath := range templatePaths {
			var service string
			_, service, templatePath = vcutils.GetProjectService(templatePath)
			_, _, indexed, _ := kv.PreCheckEnvironment(mod.Env)
			//This checks whether a enterprise env has the relevant project otherwise env gets skipped when generating seed files.
			if strings.Contains(mod.Env, ".") && !serviceFound {
				var listValues *api.Secret
				var err error
				if indexed {
					listValues, err = mod.ListEnv("super-secrets/" + mod.Env + "/")
				} else {
					listValues, err = mod.ListEnv("values/" + mod.Env + "/") //Fix values to add to project to directory
				}
				if err != nil {
					eUtils.LogErrorObject(err, logger, false)
				} else if listValues == nil {
					eUtils.LogInfo("No values were returned under values/.", logger)
				} else {
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
		}
		if !serviceFound { //Exit for irrelevant enterprises
			return "", false, ""
		}
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
				goMod, err = kv.NewModifier(c.Insecure, c.Token, c.VaultAddress, c.Env, c.Regions, logger)
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
					eUtils.LogInfo("Cannot genAuth with provided token.", logger)
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
				cds.Init(goMod, c.SecretMode, true, project, commonPaths, logger, service)
			}

			innerProject := "Not Found"
			if len(goMod.VersionFilter) >= 1 && strings.Contains(goMod.VersionFilter[len(goMod.VersionFilter)-1], "!=!") {
				innerProject = strings.Split(goMod.VersionFilter[len(goMod.VersionFilter)-1], "!=!")[1]
				goMod.VersionFilter = goMod.VersionFilter[:len(goMod.VersionFilter)-1]
			}
			if innerProject != "Not Found" {
				project = innerProject
				service = project
			}

			_, _, _, templateResult.TemplateDepth = extract.ToSeed(goMod,
				cds,
				templatePath,
				project,
				service,
				fromVault,
				&(templateResult.InterfaceTemplateSection),
				&(templateResult.ValueSection),
				&(templateResult.SecretSection),
				config.ExitOnFailure,
				config.Log,
			)
			templateResult.Env = goMod.Env + "_" + requestedVersion
			templateResultChan <- &templateResult
		}(templatePath, multiService, config, noVault)
	}
	wg.Wait()

	// Combine values of slice
	CombineSection(sliceTemplateSection, maxDepth, templateCombinedSection, logger)
	CombineSection(sliceValueSection, -1, valueCombinedSection, logger)
	CombineSection(sliceSecretSection, -1, secretCombinedSection, logger)

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
					eUtils.LogErrorObject(errA, logger, false)
				}
			} else {
				eUtils.LogInfo("Attempt to gen auth for reduced privilege token failed.  No permissions to gen auth.", logger)
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
				eUtils.LogErrorObject(errA, logger, false)
			}
		}
	}

	// Create seed file structure
	template, errT := yaml.Marshal(templateCombinedSection)
	value, errV := yaml.Marshal(valueCombinedSection)
	secret, errS := yaml.Marshal(secretCombinedSection)

	if errT != nil {
		eUtils.LogErrorObject(errT, logger, false)
	}

	if errV != nil {
		eUtils.LogErrorObject(errV, logger, false)
	}

	if errS != nil {
		eUtils.LogErrorObject(errS, logger, false)
	}
	templateData := string(template)
	// Remove single quotes generated by Marshal
	templateData = strings.ReplaceAll(templateData, "'", "")
	seedData := templateData + "\n\n\n" + string(value) + "\n\n\n" + string(secret) + "\n\n\n" + string(authYaml)

	return endPath, multiService, seedData
}

// GenerateSeedsFromVault configures the templates in trc_templates and writes them to trcx
func GenerateSeedsFromVault(ctx eUtils.ProcessContext, config eUtils.DriverConfig, logger *log.Logger) interface{} {
	if config.Clean { //Clean flag in trcx
		if strings.HasSuffix(config.Env, "_0") {
			config.Env = strings.Split(config.Env, "_")[0]
		}
		_, err1 := os.Stat(config.EndDir + config.Env)
		err := os.RemoveAll(config.EndDir + config.Env)

		if err != nil {
			eUtils.LogErrorObject(err, logger, false)
			os.Exit(1)
		}

		if err1 == nil {
			eUtils.LogInfo("Seed removed from"+config.EndDir+config.Env, logger)
		}
		return nil
	}

	// Get files from directory
	tempTemplatePaths := []string{}
	for _, startDir := range config.StartDir {
		//get files from directory
		tp := GetDirFiles(startDir)
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

	_, _, indexedEnv, _ := kv.PreCheckEnvironment(config.Env)

	if indexedEnv {
		// Get filtered using mod and templates.
		mod, err := kv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, config.Regions, logger)
		if err != nil {
			eUtils.LogErrorObject(err, logger, false)
			os.Exit(1)
		}
		mod.Env = config.Env

		templatePathsAccepted, err := kv.GetAcceptedTemplatePaths(mod, templatePaths)
		templatePaths = templatePathsAccepted
	}

	endPath, multiService, seedData := GenerateSeedsFromVaultRaw(config, false, templatePaths, logger)

	if endPath == "" && !multiService && seedData == "" {
		return nil
	}

	if strings.Contains(config.Env, "_0") {
		config.Env = strings.Split(config.Env, "_0")[0]
	}

	suffixRemoved := ""
	if strings.Contains(config.Env, "_") {
		envSplit := strings.Split(config.Env, "_")
		config.Env = envSplit[0]
		suffixRemoved = "_" + envSplit[1]
	}

	envBasePath, pathPart, pathInclude, _ := kv.PreCheckEnvironment(config.Env)

	if suffixRemoved != "" {
		config.Env = config.Env + suffixRemoved
	}

	if multiService {
		if strings.HasPrefix(config.Env, "local") {
			endPath = config.EndDir + "local/local_seed.yml"
		} else {
			if pathInclude {
				endPath = config.EndDir + envBasePath + "/" + pathPart + "/" + config.Env + "_seed.yml"
			} else {
				endPath = config.EndDir + envBasePath + "/" + config.Env + "_seed.yml"
			}
		}
	} else {
		if pathInclude {
			endPath = config.EndDir + envBasePath + "/" + pathPart + "/" + config.Env + "_seed.yml"
		} else {
			endPath = config.EndDir + envBasePath + "/" + config.Env + "_seed.yml"
		}
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

			mod, err := kv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, config.Regions, logger)
			if err != nil {
				panic(err)
			}
			mod.Env = envVersion[0]
			mod.Version = envVersion[1]

			var ctErr error
			_, certData, certLoaded, ctErr = vcutils.ConfigTemplate(mod, templatePath, config.SecretMode, project, service, config.WantCerts, false, logger)
			if ctErr != nil {
				if !strings.Contains(ctErr.Error(), "Missing .certData") {
					eUtils.CheckError(ctErr, true)
				}
			}

			if len(certData) == 0 {
				if certLoaded {
					eUtils.LogInfo("Could not load cert "+templatePath, logger)
					continue
				} else {
					continue
				}
			}

			certPath := fmt.Sprintf("%s", certData[2])
			eUtils.LogInfo("Writing certificate: "+certPath+".", logger)

			if strings.Contains(certPath, "ENV") {
				if len(mod.Env) >= 5 && (mod.Env)[:5] == "local" {
					envParts := strings.SplitN(mod.Env, "/", 3)
					certPath = strings.Replace(certPath, "ENV", envParts[1], 1)
				} else {
					certPath = strings.Replace(certPath, "ENV", mod.Env, 1)
				}
			}

			certDestination := config.EndDir + "/" + certPath
			certDestination = strings.ReplaceAll(certDestination, "//", "/")
			writeToFile(certData[1], certDestination)
			eUtils.LogInfo("certificate written to "+certDestination, logger)
		}
		return nil
	}

	if config.Diff {
		if !strings.Contains(config.Env, "_") {
			config.Env = config.Env + "_0"
		}
		config.Update(&seedData, config.Env+"||"+config.Env+"_seed.yml")
	} else {
		writeToFile(seedData, endPath)
		// Print that we're done
		if strings.Contains(config.Env, "_0") {
			config.Env = strings.Split(config.Env, "_")[0]
		}
		if strings.Contains(envBasePath, "_") {
			envBasePath = strings.Split(envBasePath, "_")[0]
		}

		eUtils.LogInfo("Seed created and written to "+endPath, logger)
	}

	return nil
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

func GetDirFiles(dir string) []string {
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
		newPaths := GetDirFiles(filePath)
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
func CombineSection(sliceSectionInterface interface{}, maxDepth int, combinedSectionInterface interface{}, logger *log.Logger) {
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
			eUtils.LogInfo(fmt.Sprintf("Env failed to gen.  MaxDepth: %d, okMap: %t\n", maxDepth, okMap), logger)
		}
		sliceSection := sliceSectionInterface.([]interface{})

		for _, v := range sliceSection {
			MergeMaps(combinedSectionInterface, v)
		}
	}
}

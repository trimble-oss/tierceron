package xutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"Vault.Whoville/utils"
	eUtils "Vault.Whoville/utils"
	vcutils "Vault.Whoville/vaultconfig/utils"
	"Vault.Whoville/vaulthelper/kv"
	"gopkg.in/yaml.v2"
)

var wg sync.WaitGroup

var wg2 sync.WaitGroup

type TemplateResultData struct {
	interfaceTemplateSection interface{}
	valueSection             map[string]map[string]map[string]string
	secretSection            map[string]map[string]map[string]string
	templateDepth            int
	env                      string
}

var templateResultChan = make(chan *TemplateResultData, 5)

// GenerateSeedsFromVaultRaw configures the templates in vault_templates and writes them to vaultx
func GenerateSeedsFromVaultRaw(config eUtils.DriverConfig, fromVault bool, templatePaths []string) (string, string, bool, string) {
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

	project := ""
	endPath := ""
	multiService := false
	service := ""
	var mod *kv.Modifier

	if config.Token != "" {
		var err error
		mod, err = kv.NewModifier(config.Token, config.VaultAddress, config.Env, config.Regions)
		if err != nil {
			panic(err)
		}
		mod.Env = config.Env
	}

	if config.GenAuth && mod != nil {
		_, err := mod.ReadData("apiLogins/meta")
		if err != nil {
			fmt.Println("Cannot genAuth with provided token.")
			os.Exit(1)
		}
	}

	//Reciever for configs
	go func(c eUtils.DriverConfig) {
		for {
			select {
			case tResult := <-templateResultChan:
				if config.Env == tResult.env {
					sliceTemplateSection = append(sliceTemplateSection, tResult.interfaceTemplateSection)
					sliceValueSection = append(sliceValueSection, tResult.valueSection)
					sliceSecretSection = append(sliceSecretSection, tResult.secretSection)
					if tResult.templateDepth > maxDepth {
						maxDepth = tResult.templateDepth
						//templateCombinedSection = interfaceTemplateSection
					}
					wg.Done()
				} else {
					go func(tResult *TemplateResultData) {
						templateResultChan <- tResult
					}(tResult)
				}
			default:
			}
		}
	}(config)

	// Configure each template in directory
	for _, templatePath := range templatePaths {
		wg.Add(1)
		go func(templatePath string, project string, service string, multiService bool, c eUtils.DriverConfig) {
			// Map Subsections
			var templateResult TemplateResultData

			templateResult.valueSection = map[string]map[string]map[string]string{}
			templateResult.valueSection["values"] = map[string]map[string]string{}

			templateResult.secretSection = map[string]map[string]map[string]string{}
			templateResult.secretSection["super-secrets"] = map[string]map[string]string{}

			var goMod *kv.Modifier

			if c.Token != "" {
				var err error
				goMod, err = kv.NewModifier(c.Token, c.VaultAddress, c.Env, c.Regions)
				if err != nil {
					panic(err)
				}
				goMod.Env = c.Env
			}

			if c.GenAuth && goMod != nil {
				_, err := mod.ReadData("apiLogins/meta")
				if err != nil {
					fmt.Println("Cannot genAuth with provided token.")
					os.Exit(1)
				}
			}

			//check for template_files directory here
			s := strings.Split(templatePath, "/")
			//figure out which path is vault_templates
			dirIndex := -1
			for j, piece := range s {
				if piece == "vault_templates" {
					dirIndex = j
				}
			}
			if dirIndex != -1 {
				project = s[dirIndex+1]
				if service != s[dirIndex+2] {
					multiService = true
				}
				service = s[dirIndex+2]
			}

			// Clean up service naming (Everything after '.' removed)
			dotIndex := strings.Index(service, ".")
			if dotIndex > 0 && dotIndex <= len(service) {
				service = service[0:dotIndex]
			}

			var cds *vcutils.ConfigDataStore
			if goMod != nil {
				cds = new(vcutils.ConfigDataStore)
				cds.Init(goMod, c.SecretMode, true, project, service)
			}

			templateResult.interfaceTemplateSection, templateResult.valueSection, templateResult.secretSection, templateResult.templateDepth = ToSeed(goMod,
				cds,
				templatePath,
				config.Log,
				project,
				service,
				fromVault,
				templateResult.interfaceTemplateSection,
				templateResult.valueSection,
				templateResult.secretSection,
			)
			templateResult.env = goMod.Env
			templateResultChan <- &templateResult
		}(templatePath, project, service, multiService, config)
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
			authConfigurations["vaultApiTokenSecret"] = "<Enter Secret Here>"

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

	return service, endPath, multiService, seedData
}

// GenerateSeedsFromVault configures the templates in vault_templates and writes them to vaultx
func GenerateSeedsFromVault(config eUtils.DriverConfig) {
	if config.Diff { //Clean flag in vaultX
		_, err1 := os.Stat(config.EndDir + config.Env)
		err := os.RemoveAll(config.EndDir + config.Env)

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err1 == nil {
			fmt.Println("Seed removed from", config.EndDir+config.Env)
		}
		return
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

	_, endPath, multiService, seedData := GenerateSeedsFromVaultRaw(config, false, templatePaths)

	if multiService {
		if strings.HasPrefix(config.Env, "local") {
			endPath = config.EndDir + "local/local_seed.yml"
		} else {
			endPath = config.EndDir + config.Env + "/" + config.Env + "_seed.yml"
		}
	} else {
		endPath = config.EndDir + config.Env + "/" + config.Env + "_seed.yml"
	}

	writeToFile(seedData, endPath)
	// Print that we're done
	fmt.Println("Seed created and written to " + strings.Replace(config.EndDir, "\\", "/", -1) + config.Env)
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

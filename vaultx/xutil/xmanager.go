package xutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"bitbucket.org/dexterchaney/whoville/utils"
	eUtils "bitbucket.org/dexterchaney/whoville/utils"
	vcutils "bitbucket.org/dexterchaney/whoville/vaultconfig/utils"
	"bitbucket.org/dexterchaney/whoville/vaulthelper/kv"
	"gopkg.in/yaml.v2"
)

// GenerateSeedsFromVault configures the templates in vault_templates and writes them to vaultx
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

	valueSection = map[string]map[string]map[string]string{}
	valueSection["values"] = map[string]map[string]string{}

	secretSection = map[string]map[string]map[string]string{}
	secretSection["super-secrets"] = map[string]map[string]string{}

	// Configure each template in directory
	for _, templatePath := range templatePaths {
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
		if dotIndex > 0 {
			service = service[0:dotIndex]
		}

		var cds *vcutils.ConfigDataStore
		if mod != nil {
			cds = new(vcutils.ConfigDataStore)
			cds.Init(mod, config.SecretMode, true, project, service)
		}

		interfaceTemplateSection, valueSection, secretSection, templateDepth := ToSeed(mod, cds, templatePath, config.Log, project, service, fromVault)
		if templateDepth > maxDepth {
			maxDepth = templateDepth
			//templateCombinedSection = interfaceTemplateSection
		}

		// Append new sections to propper slices
		sliceTemplateSection = append(sliceTemplateSection, interfaceTemplateSection)
		sliceValueSection = append(sliceValueSection, valueSection)
		sliceSecretSection = append(sliceSecretSection, secretSection)
	}

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
	// Get files from directory
	templatePaths := []string{}

	//templatePaths
	for _, startDir := range config.StartDir {
		//get files from directory
		tp := getDirFiles(startDir)
		templatePaths = append(templatePaths, tp...)
	}

	service, endPath, multiService, seedData := GenerateSeedsFromVaultRaw(config, false, templatePaths)

	config.EndDir = config.EndDir + config.Env + string(os.PathSeparator)
	if multiService {
		if strings.HasPrefix(config.Env, "local") {
			endPath = config.EndDir + "local_seed.yml"
		} else {
			endPath = config.EndDir + config.Env + "_seed.yml"
		}
	} else {
		endPath = config.EndDir + service + "_seed.yml"
	}
	writeToFile(seedData, endPath)

	// Print that we're done
	fmt.Println("seed created and written to ", config.EndDir)
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

	// Value/secret slice section
	if maxDepth < 0 {
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
		sliceSection := sliceSectionInterface.([]interface{})

		for _, v := range sliceSection {
			MergeMaps(combinedSectionInterface, v)
		}
	}
}

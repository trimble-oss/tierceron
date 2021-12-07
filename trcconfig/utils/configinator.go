package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"tierceron/utils"
	eUtils "tierceron/utils"
	"tierceron/vaulthelper/kv"
)

var mutex = &sync.Mutex{}

//GenerateConfigsFromVault configures the templates in trc_templates and writes them to trcconfig
func GenerateConfigsFromVault(ctx eUtils.ProcessContext, config eUtils.DriverConfig) interface{} {
	Cyan := "\033[36m"
	Reset := "\033[0m"
	if runtime.GOOS == "windows" {
		Reset = ""
		Cyan = ""
	}
	modCheck, err := kv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, config.Regions)
	modCheck.Env = config.Env
	version := ""
	if err != nil {
		panic(err)
	}
	modCheck.VersionFilter = config.VersionFilter

	//Check if templateInfo is selected for template or values
	templateInfo := false
	valueInfo := false
	if strings.Contains(config.Env, "_") {
		envAndVersion := strings.Split(config.Env, "_")
		config.Env = envAndVersion[0]
		version = envAndVersion[1]
		if version == "valueInfo" {
			valueInfo = true
		} else if version == "templateInfo" {
			templateInfo = true
		}
	}
	versionData := make(map[string]interface{})
	if !modCheck.ValidateEnvironment(config.Env, false) {
		fmt.Println("Mismatched token for requested environment: " + config.Env)
		os.Exit(1)
	}

	initialized := false
	templatePaths := []string{}
	endPaths := []string{}

	//templatePaths
	for _, startDir := range config.StartDir {
		//get files from directory
		tp, ep := getDirFiles(startDir, config.EndDir)
		templatePaths = append(templatePaths, tp...)
		endPaths = append(endPaths, ep...)
	}

	//File filter
	fileFound := true
	fileFilterIndex := make([]int, len(config.FileFilter))
	fileFilterCounter := 0
	if len(config.FileFilter) != 0 && config.FileFilter[0] != "" {
		for _, FileFilter := range config.FileFilter {
			for i, templatePath := range templatePaths {
				if strings.Contains(templatePath, FileFilter) {
					fileFilterIndex[fileFilterCounter] = i
					fileFilterCounter++
					fileFound = true
					break
				}
			}
		}
		if !fileFound {
			fmt.Println("Could not find specified file in templates")
			os.Exit(1)
		}

		fileTemplatePaths := []string{}
		fileEndPaths := []string{}
		for _, index := range fileFilterIndex {
			fileTemplatePaths = append(fileTemplatePaths, templatePaths[index])
			fileEndPaths = append(fileEndPaths, endPaths[index])
		}

		templatePaths = fileTemplatePaths
		endPaths = fileEndPaths
	}

	if valueInfo {
		versionDataMap := make(map[string]map[string]interface{})
		//Gets version metadata for super secrets or values if super secrets don't exist.
		if strings.Contains(modCheck.Env, ".") {
			config.VersionFilter = append(config.VersionFilter, strings.Split(modCheck.Env, "_")[0])
			modCheck.Env = strings.Split(modCheck.Env, "_")[0]
		}

		for _, templatePath := range templatePaths {
			_, service, _ := utils.GetProjectService(templatePath) //This checks for nested project names
			config.VersionFilter = append(config.VersionFilter, service)
		}

		if config.WantCerts { //For cert version history
			config.VersionFilter = append(config.VersionFilter, "Common")
		}

		config.VersionFilter = utils.RemoveDuplicates(config.VersionFilter)
		versionMetadataMap := utils.GetProjectVersionInfo(config, modCheck)
		var masterKey string
		project := ""
		neverPrinted := true
		if len(config.VersionFilter) > 0 {
			project = config.VersionFilter[0]
		}
		for key := range versionMetadataMap {
			passed := false
			if config.WantCerts {
				for _, service := range config.VersionFilter {
					if !passed && strings.Contains(key, "Common") && strings.Contains(key, service) && !strings.Contains(key, project) && !strings.HasSuffix(key, "Common") {
						if len(key) > 0 {
							keySplit := strings.Split(key, "/")
							config.VersionInfo(versionMetadataMap[key], false, keySplit[len(keySplit)-1], neverPrinted)
							passed = true
							neverPrinted = false
						}
					}
				}
			} else {
				if len(key) > 0 {
					config.VersionInfo(versionMetadataMap[key], false, "", false)
					os.Exit(1)
				}
			}
		}
		if neverPrinted {
			fmt.Println("No version data available for this env")
		}
		os.Exit(1)

		for valuePath, data := range versionMetadataMap {
			projectFound := false
			for _, project := range config.VersionFilter {
				if strings.Contains(valuePath, project) {
					projectFound = true
					initialized = true
					break
				}
			}
			if !projectFound {
				continue
			}

			versionDataMap[valuePath] = data
			masterKey = valuePath
		}

		if versionDataMap != nil {
			config.VersionInfo(versionDataMap[masterKey], false, masterKey, false)
		} else if !initialized {
			fmt.Println(Cyan + "No metadata found for this environment" + Reset)
		}
		return nil //End of -versions flag
	} else if !templateInfo {
		if version != "0" { //Check requested version bounds
			for _, templatePath := range templatePaths {
				_, service, _ := GetProjectService(templatePath)             //This checks for nested project names
				config.VersionFilter = append(config.VersionFilter, service) //Adds nested project name to filter otherwise it will be not found.
			}
			config.VersionFilter = utils.RemoveDuplicates(config.VersionFilter)

			versionMetadataMap := utils.GetProjectVersionInfo(config, modCheck)
			versionNumbers := utils.GetProjectVersions(config, versionMetadataMap)

			utils.BoundCheck(config, versionNumbers, version)
		}
	}

	var wg sync.WaitGroup
	//configure each template in directory
	for i, templatePath := range templatePaths {
		wg.Add(1)
		go func(i int, templatePath string, version string, versionData map[string]interface{}) {
			defer wg.Done()

			mod, _ := kv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, config.Regions)
			mod.Env = config.Env
			mod.Version = version
			//check for template_files directory here
			project, service, templatePath := GetProjectService(templatePath)

			var isCert bool
			if service != "" {
				if strings.HasSuffix(templatePath, ".DS_Store") {
					goto wait
				}

				isCert := false
				if strings.Contains(templatePath, ".pfx.mf") ||
					strings.Contains(templatePath, ".cer.mf") ||
					strings.Contains(templatePath, ".pem.mf") ||
					strings.Contains(templatePath, ".jks.mf") {
					isCert = true
				}

				if config.WantCerts != isCert {
					goto wait
				}

				if strings.HasSuffix(templatePath, ".tmpl") {
					if !config.ZeroConfig {
						if strings.HasSuffix(templatePath, "nc.properties.tmpl") {
							goto wait
						}
					} else {
						if !strings.HasSuffix(templatePath, "nc.properties.tmpl") {
							goto wait
						}
					}
				}

				var configuredTemplate string
				var certData map[int]string
				certLoaded := false
				if templateInfo {
					data := getTemplateVersionData(mod, config.SecretMode, project, service, endPaths[i])
					mutex.Lock()
					if data == nil {
						fmt.Println("Template version data could not be retrieved")
						os.Exit(1)
					}
					versionData[endPaths[i]] = data
					mutex.Unlock()
					goto wait
				} else {
					configuredTemplate, certData, certLoaded = ConfigTemplate(mod, templatePath, config.SecretMode, project, service, config.WantCerts, false)
				}
				//generate template or certificate
				if config.WantCerts && certLoaded {
					if len(certData) == 0 {
						fmt.Println("Could not load cert ", endPaths[i])
						goto wait
					}
					certDestination := config.EndDir + "/" + certData[0]
					writeToFile(certData[1], certDestination)
					fmt.Println("certificate written to ", certDestination)
					goto wait
				} else {
					if config.Diff {
						if version != "" {
							config.Update(&configuredTemplate, config.Env+"_"+version+"||"+endPaths[i])
						} else {
							config.Update(&configuredTemplate, config.Env+"||"+endPaths[i])
						}
					} else {
						writeToFile(configuredTemplate, endPaths[i])
					}
				}
			} else {
				isCert := false
				if strings.Contains(service, ".pfx.mf") ||
					strings.Contains(service, ".cer.mf") ||
					strings.Contains(service, ".pem.mf") ||
					strings.Contains(service, ".jks.mf") {
					isCert = true
				}

				if config.WantCerts != isCert {
					goto wait
				}
				//assume the starting directory was trc_templates
				var configuredTemplate string
				var certData map[int]string
				certLoaded := false
				if templateInfo {
					data := getTemplateVersionData(mod, config.SecretMode, project, service, endPaths[i])
					versionData[endPaths[i]] = data
					goto wait
				} else {
					configuredTemplate, certData, certLoaded = ConfigTemplate(mod, templatePath, config.SecretMode, project, service, config.WantCerts, false)
				}
				if config.WantCerts && certLoaded {
					certDestination := config.EndDir + "/" + certData[0]
					writeToFile(certData[1], certDestination)
					fmt.Println("certificate written to ", certDestination)
					goto wait
				} else {
					if config.Diff {
						if version != "" {
							config.Update(&configuredTemplate, config.Env+"_"+version+"||"+endPaths[i])
						} else {
							config.Update(&configuredTemplate, config.Env+"||"+endPaths[i])
						}
					} else {
						writeToFile(configuredTemplate, endPaths[i])
					}
				}
			}

			//print that we're done
			if !config.Diff && !isCert && !templateInfo {
				if runtime.GOOS == "windows" {
					fmt.Println("template configured and written to " + endPaths[i])
				} else {
					fmt.Println("\033[0;33m" + "template configured and written to " + endPaths[i] + "\033[0m")
				}
			}

		wait:
			mod.Close()
		}(i, templatePath, version, versionData)
	}
	wg.Wait()
	if templateInfo {
		config.VersionInfo(versionData, true, "", false)
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

func getDirFiles(dir string, endDir string) ([]string, []string) {
	files, err := ioutil.ReadDir(dir)
	filePaths := []string{}
	endPaths := []string{}
	if err != nil {
		//this is a file
		return []string{dir}, []string{endDir}
	}
	for _, file := range files {
		//add this directory to path names
		if dir[len(dir)-1] != '/' {
			dir = dir + "/"
		}

		filePath := dir + file.Name()

		//take off .tmpl extension
		filename := file.Name()
		if strings.HasSuffix(filename, ".DS_Store") {
			continue
		}
		extension := filepath.Ext(filename)
		endPath := ""
		if extension == ".tmpl" {
			name := filename[0 : len(filename)-len(extension)]
			endPath = endDir + "/" + name
		} else {
			endPath = endDir + "/" + filename
		}
		//recurse to next level
		newPaths, newEndPaths := getDirFiles(filePath, endPath)
		filePaths = append(filePaths, newPaths...)
		endPaths = append(endPaths, newEndPaths...)
		//add endings of path names
	}
	return filePaths, endPaths
}

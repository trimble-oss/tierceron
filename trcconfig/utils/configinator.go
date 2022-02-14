package utils

import (
	"io/ioutil"
	"log"
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
func GenerateConfigsFromVault(ctx eUtils.ProcessContext, config eUtils.DriverConfig, logger *log.Logger) interface{} {
	Cyan := "\033[36m"
	Reset := "\033[0m"
	if runtime.GOOS == "windows" {
		Reset = ""
		Cyan = ""
	}
	modCheck, err := kv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, config.Regions, logger)
	modCheck.Env = config.Env
	version := ""
	if err != nil {
		eUtils.LogErrorObject(err, logger, false)
	}
	modCheck.VersionFilter = config.VersionFilter

	//Check if templateInfo is selected for template or values
	templateInfo := false
	versionInfo := false
	if strings.Contains(config.Env, "_") {
		envVersion := eUtils.SplitEnv(config.Env)

		config.Env = envVersion[0]
		version = envVersion[1]
		if version == "versionInfo" {
			versionInfo = true
		} else if version == "templateInfo" {
			templateInfo = true
		}
	}
	versionData := make(map[string]interface{})
	if !modCheck.ValidateEnvironment(config.Env, false, logger) {
		eUtils.LogInfo("Mismatched token for requested environment: "+config.Env, logger)
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

	_, _, indexedEnv, _ := kv.PreCheckEnvironment(config.Env)
	if indexedEnv {
		templatePaths, err = kv.GetAcceptedTemplatePaths(modCheck, templatePaths)
		if err != nil {
			eUtils.LogErrorObject(err, logger, false)
		}
		endPaths, err = kv.GetAcceptedTemplatePaths(modCheck, endPaths)
		if err != nil {
			eUtils.LogErrorObject(err, logger, false)
		}
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
			eUtils.LogInfo("Could not find specified file in templates", logger)
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

	for _, templatePath := range templatePaths {
		if !config.WantCerts && strings.Contains(templatePath, "Common") {
			continue
		}
		_, service, _ := utils.GetProjectService(templatePath)       //This checks for nested project names
		config.VersionFilter = append(config.VersionFilter, service) //Adds nested project name to filter otherwise it will be not found.
	}

	if config.WantCerts && versionInfo { //For cert version history
		config.VersionFilter = append(config.VersionFilter, "Common")
	}

	config.VersionFilter = utils.RemoveDuplicates(config.VersionFilter)
	modCheck.VersionFilter = config.VersionFilter

	if versionInfo {
		versionDataMap := make(map[string]map[string]interface{})
		//Gets version metadata for super secrets or values if super secrets don't exist.
		if strings.Contains(modCheck.Env, ".") {
			envVersion := eUtils.SplitEnv(modCheck.Env)
			config.VersionFilter = append(config.VersionFilter, envVersion[0])
			modCheck.Env = envVersion[0]
		}

		versionMetadataMap := utils.GetProjectVersionInfo(config, modCheck, logger)
		var masterKey string
		project := ""
		neverPrinted := true
		if len(config.VersionFilter) > 0 {
			project = config.VersionFilter[0]
		}
		for key := range versionMetadataMap {
			passed := false
			if config.WantCerts {
				//If paths were clean - this would be logic
				/*
					if len(key) > 0 {
						keySplit := strings.Split(key, "/")
						config.VersionInfo(versionMetadataMap[key], false, keySplit[len(keySplit)-1], neverPrinted)
						neverPrinted = false
					}
				*/
				//This is happening because of garbage paths that look like this -> values/{projectName}/{service}/Common/{file.cer}
				for _, service := range config.VersionFilter { //The following for loop could be removed if vault paths were clean
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
			eUtils.LogInfo("No version data available for this env", logger)
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
			eUtils.LogInfo(Cyan+"No metadata found for this environment"+Reset, logger)
		}
		return nil //End of -versions flag
	} else if !templateInfo {
		if version != "0" { //Check requested version bounds
			versionMetadataMap := utils.GetProjectVersionInfo(config, modCheck, logger)
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

			mod, _ := kv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, config.Regions, logger)
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
					data := getTemplateVersionData(mod, config.SecretMode, project, service, endPaths[i], logger)
					mutex.Lock()
					if data == nil {
						eUtils.LogInfo("Template version data could not be retrieved", logger)
						os.Exit(1)
					}
					versionData[endPaths[i]] = data
					mutex.Unlock()
					goto wait
				} else {
					var ctErr error
					configuredTemplate, certData, certLoaded, ctErr = ConfigTemplate(mod, templatePath, config.SecretMode, project, service, config.WantCerts, false, logger)
					if ctErr != nil {
						if !strings.Contains(ctErr.Error(), "Missing .certData") {
							eUtils.CheckError(ctErr, true)
						}
					}
				}
				//generate template or certificate
				if config.WantCerts && certLoaded {
					if len(certData) == 0 {
						eUtils.LogInfo("Could not load cert "+endPaths[i], logger)
						goto wait
					}
					certDestination := config.EndDir + "/" + certData[0]
					writeToFile(certData[1], certDestination)
					eUtils.LogInfo("certificate written to "+certDestination, logger)
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
					data := getTemplateVersionData(mod, config.SecretMode, project, service, endPaths[i], logger)
					versionData[endPaths[i]] = data
					goto wait
				} else {
					var ctErr error
					configuredTemplate, certData, certLoaded, ctErr = ConfigTemplate(mod, templatePath, config.SecretMode, project, service, config.WantCerts, false, logger)
					if ctErr != nil {
						if !strings.Contains(ctErr.Error(), "Missing .certData") {
							eUtils.CheckError(ctErr, true)
						}
					}
				}
				if config.WantCerts && certLoaded {
					certDestination := config.EndDir + "/" + certData[0]
					writeToFile(certData[1], certDestination)
					eUtils.LogInfo("certificate written to "+certDestination, logger)
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
					eUtils.LogInfo("template configured and written to "+endPaths[i], logger)
				} else {
					eUtils.LogInfo("\033[0;33m"+"template configured and written to "+endPaths[i]+"\033[0m", logger)
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

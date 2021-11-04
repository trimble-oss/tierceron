package utils

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
	modCheck.ProjectVersionFilter = config.VersionProjectFilter

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
	if valueInfo {
		versionDataMap := make(map[string]map[string]interface{})
		versionMetadataMap := make(map[string]map[string]interface{})
		//Gets version metadata for super secrets or values if super secrets don't exist.
		secretMetadataMap, err := modCheck.GetVersionValues(modCheck, "super-secrets")
		if secretMetadataMap == nil {
			versionMetadataMap, err = modCheck.GetVersionValues(modCheck, "values")
		}
		for key, value := range secretMetadataMap {
			versionMetadataMap[key] = value
		}
		if versionMetadataMap == nil {
			fmt.Println("Unable to get version metadata for values")
			os.Exit(1)
		}
		if err != nil {
			panic(err)
		}
		for valuePath, data := range versionMetadataMap {
			projectFound := false
			for _, project := range config.VersionProjectFilter {
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
		}
		//Find shortest path
		pathCount := 0
		shortestPath := ""
		secretExist := false
		secretPath := ""
		for fullPath, data := range versionDataMap {
			if strings.Contains(fullPath, "super-secret") && strings.HasSuffix(fullPath, config.VersionProjectFilter[0]) {
				secretExist = true
				secretPath = fullPath
			}
			tempCount := strings.Count(fullPath, "/")
			if tempCount < pathCount || tempCount == 0 || data != nil {
				pathCount = tempCount
				shortestPath = fullPath
			}
		}
		if secretExist {
			config.VersionInfo(versionDataMap[secretPath], false, secretPath)
		} else {
			config.VersionInfo(versionDataMap[shortestPath], false, shortestPath)
		}
		if !initialized {
			fmt.Println(Cyan + "No metadata found for this environment" + Reset)
		}
		return nil
	} else {
		if version != "0" { //Check requested version bounds
			versionNumbers := make([]int, 0)
			versionMetadataMap, err := modCheck.GetVersionValues(modCheck, "values")
			if err != nil {
				panic(err)
			}
			for valuePath, data := range versionMetadataMap {
				projectFound := false
				for _, project := range config.VersionProjectFilter {
					if strings.Contains(valuePath, project) {

						projectFound = true
						initialized = true
						for key, _ := range data {
							versionNo, err := strconv.Atoi(key)
							if err != nil {
								fmt.Println()
							}
							versionNumbers = append(versionNumbers, versionNo)
						}
					}
					if !projectFound {
						continue
					}
				}
			}

			sort.Ints(versionNumbers)
			if len(versionNumbers) >= 1 {
				latestVersion := versionNumbers[len(versionNumbers)-1]
				oldestVersion := versionNumbers[0]
				userVersion, _ := strconv.Atoi(version)
				if userVersion > latestVersion || userVersion < oldestVersion && len(versionNumbers) != 1 {
					fmt.Println(Cyan + "This version " + config.Env + "_" + version + " is not available as the latest version is " + strconv.Itoa(versionNumbers[len(versionNumbers)-1]) + " and oldest version available is " + strconv.Itoa(versionNumbers[0]) + Reset)
					os.Exit(1)
				}
			} else {
				fmt.Println(Cyan + "No version data found" + Reset)
				os.Exit(1)
			}
		}
	}
	templatePaths := []string{}
	endPaths := []string{}

	//templatePaths
	for _, startDir := range config.StartDir {
		//get files from directory
		tp, ep := getDirFiles(startDir, config.EndDir)
		templatePaths = append(templatePaths, tp...)
		endPaths = append(endPaths, ep...)
	}

	//file filter
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
			s := strings.Split(templatePath, "/")
			//figure out which path is trc_templates
			dirIndex := -1
			for j, piece := range s {
				if piece == "trc_templates" {
					dirIndex = j
					break
				}
			}

			var isCert bool
			if dirIndex != -1 {
				serviceTemplate := s[dirIndex+2]
				if strings.HasSuffix(templatePath, ".DS_Store") {
					goto wait
				}

				isCert := false
				if strings.Contains(serviceTemplate, ".pfx.mf") ||
					strings.Contains(serviceTemplate, ".cer.mf") ||
					strings.Contains(serviceTemplate, ".pem.mf") ||
					strings.Contains(serviceTemplate, ".jks.mf") {
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
				if templateInfo {
					data := getTemplateVersionData(mod, config.SecretMode, s[dirIndex+1], serviceTemplate, endPaths[i])
					mutex.Lock()
					if data == nil {
						fmt.Println("Template version data could not be retrieved")
						os.Exit(1)
					}
					versionData[endPaths[i]] = data
					mutex.Unlock()
					goto wait
				} else {
					configuredTemplate, certData = ConfigTemplate(mod, templatePath, endPaths[i], config.SecretMode, s[dirIndex+1], serviceTemplate, config.WantCerts, false)
				}
				//generate template or certificate
				if config.WantCerts {
					if len(certData) == 0 {
						fmt.Println("Could not load cert ", endPaths[i])
						goto wait
					}
					certDestination := config.EndDir + "/" + certData[0]
					writeToFile(certData[1], certDestination)
					fmt.Println("certificate written to ", certDestination)
					goto wait
				} else if !config.WantCerts {
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
				serviceTemplate := s[len(s)-1]
				isCert := false
				if strings.Contains(serviceTemplate, ".pfx.mf") ||
					strings.Contains(serviceTemplate, ".cer.mf") ||
					strings.Contains(serviceTemplate, ".pem.mf") ||
					strings.Contains(serviceTemplate, ".jks.mf") {
					isCert = true
				}

				if config.WantCerts != isCert {
					goto wait
				}
				//assume the starting directory was trc_templates
				var configuredTemplate string
				var certData map[int]string
				if templateInfo {
					data := getTemplateVersionData(mod, config.SecretMode, s[dirIndex+1], serviceTemplate, endPaths[i])
					versionData[endPaths[i]] = data
					goto wait
				} else {
					configuredTemplate, certData = ConfigTemplate(mod, templatePath, endPaths[i], config.SecretMode, s[dirIndex+1], serviceTemplate, config.WantCerts, false)
				}
				if config.WantCerts {
					certDestination := config.EndDir + "/" + certData[0]
					writeToFile(certData[1], certDestination)
					fmt.Println("certificate written to ", certDestination)
					goto wait
				} else if !config.WantCerts {
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
		config.VersionInfo(versionData, true, "")
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
		filePath := dir + "/" + file.Name()
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

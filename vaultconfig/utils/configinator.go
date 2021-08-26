package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"Vault.Whoville/utils"
	eUtils "Vault.Whoville/utils"
	"Vault.Whoville/vaulthelper/kv"
)

//GenerateConfigsFromVault configures the templates in vault_templates and writes them to vaultconfig
func GenerateConfigsFromVault(config eUtils.DriverConfig) {
	modCheck, err := kv.NewModifier(config.Token, config.VaultAddress, config.Env, config.Regions)
	if err != nil {
		panic(err)
	}

	if !modCheck.ValidateEnvironment(config.Env) {
		fmt.Println("Mismatched token for requested environment: " + config.Env)
		os.Exit(1)
	}

	modCheck.Env = config.Env
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
		go func(i int, templatePath string) {
			defer wg.Done()

			mod, _ := kv.NewModifier(config.Token, config.VaultAddress, config.Env, config.Regions)
			mod.Env = config.Env
			//check for template_files directory here
			s := strings.Split(templatePath, "/")
			//figure out which path is vault_templates
			dirIndex := -1
			for j, piece := range s {
				if piece == "vault_templates" {
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

				if config.WantCert != isCert {
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

				configuredTemplate, certData := ConfigTemplate(mod, templatePath, endPaths[i], config.SecretMode, s[dirIndex+1], serviceTemplate, config.WantCert, false)
				//generate template or certificate
				if config.WantCert {
					if len(certData) == 0 {
						fmt.Println("Could not load cert ", endPaths[i])
						goto wait
					}
					certDestination := config.EndDir + "/" + certData[0]
					writeToFile(certData[1], certDestination)
					fmt.Println("certificate written to ", certDestination)
					goto wait
				} else if !config.WantCert {
					if config.Diff {
						config.Update(&configuredTemplate, config.Env+"||"+endPaths[i])
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

				if config.WantCert != isCert {
					goto wait
				}
				//assume the starting directory was vault_templates
				configuredTemplate, certData := ConfigTemplate(mod, templatePath, endPaths[i], config.SecretMode, s[1], serviceTemplate, config.WantCert, false)
				if config.WantCert {
					certDestination := config.EndDir + "/" + certData[0]
					writeToFile(certData[1], certDestination)
					fmt.Println("certificate written to ", certDestination)
					goto wait
				} else if !config.WantCert {
					if config.Diff {
						config.Update(&configuredTemplate, config.Env+"||"+endPaths[i])
					} else {
						writeToFile(configuredTemplate, endPaths[i])
					}
				}
			}

			//print that we're done
			if !config.Diff && !isCert {
				if runtime.GOOS == "windows" {
					fmt.Println("template configured and written to " + endPaths[i])
				} else {
					fmt.Println("\033[0;33m" + "template configured and written to " + endPaths[i] + "\033[0m")
				}
			}

		wait:
			mod.Close()
		}(i, templatePath)
	}
	wg.Wait()
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

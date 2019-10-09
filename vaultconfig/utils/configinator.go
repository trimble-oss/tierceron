package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"bitbucket.org/dexterchaney/whoville/utils"
	eUtils "bitbucket.org/dexterchaney/whoville/utils"
	"bitbucket.org/dexterchaney/whoville/vaulthelper/kv"
)

//GenerateConfigsFromVault configures the templates in vault_templates and writes them to vaultconfig
func GenerateConfigsFromVault(config eUtils.DriverConfig) {
	mod, err := kv.NewModifier(config.Token, config.VaultAddress)
	if err != nil {
		panic(err)
	}
	if !mod.ValidateEnvironment(config.Env) {
		fmt.Println("Mismatched token for requested environment: " + config.Env)
		os.Exit(1)
	}

	mod.Env = config.Env

	templatePaths := []string{}
	endPaths := []string{}

	//templatePaths
	for _, startDir := range config.StartDir {
		//get files from directory
		tp, ep := getDirFiles(startDir, config.EndDir)
		templatePaths = append(templatePaths, tp...)
		endPaths = append(endPaths, ep...)
	}

	//configure each template in directory
	for i, templatePath := range templatePaths {
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
		if dirIndex != -1 {
			serviceTemplate := s[dirIndex+2]

			isCert := false
			if strings.HasSuffix(serviceTemplate, ".pfx.mf.tmpl") || strings.HasSuffix(serviceTemplate, ".cer.mf.tmpl") {
				isCert = true
			}

			if config.WantCert != isCert {
				continue
			}

			configuredTemplate, certData := ConfigTemplate(mod, templatePath, endPaths[i], config.SecretMode, s[dirIndex+1], serviceTemplate, config.WantCert)
			//generate template or certificate
			if config.WantCert {
				if len(certData) == 0 {
					fmt.Println("Could not load cert ", endPaths[i])
					continue
				}
				certDestination := config.EndDir + "/" + certData[0]
				writeToFile(certData[1], certDestination)
				fmt.Println("certificate written to ", certDestination)
				continue
			} else if !config.WantCert {
				writeToFile(configuredTemplate, endPaths[i])
			}
		} else {
			serviceTemplate := s[2]
			isCert := false
			if strings.HasSuffix(serviceTemplate, ".pfx.mf.tmpl") || strings.HasSuffix(serviceTemplate, ".cer.mf.tmpl") {
				isCert = true
			}

			if config.WantCert != isCert {
				continue
			}

			//assume the starting directory was vault_templates
			configuredTemplate, certData := ConfigTemplate(mod, templatePath, endPaths[i], config.SecretMode, s[1], serviceTemplate, config.WantCert)
			if config.WantCert {
				certDestination := config.EndDir + "/" + certData[0]
				writeToFile(certData[1], certDestination)
				fmt.Println("certificate written to ", certDestination)
				continue
			} else if !config.WantCert {
				writeToFile(configuredTemplate, endPaths[i])
			}
		}
		//print that we're done
		fmt.Println("templates configured and written to ", endPaths[i])
	}

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

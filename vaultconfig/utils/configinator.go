package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"bitbucket.org/dexterchaney/whoville/utils"
	"bitbucket.org/dexterchaney/whoville/vaulthelper/kv"
)

//ConfigFromVault configures the templates in vault_templates and writes them to vaultconfig
func ConfigFromVault(token string, address string, env string, secretMode bool, servicesWanted []string, startDir string, endDir string, cert bool) {
	generatedCert := false
	mod, err := kv.NewModifier(token, address)
	if err != nil {
		panic(err)
	}
	if !mod.ValidateEnvironment(env) {
		fmt.Println("Mismatched token for requested environment: " + env)
		os.Exit(1)
	}

	mod.Env = env

	//get files from directory
	templatePaths, endPaths := getDirFiles(startDir, endDir)
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
			configuredTemplate, certData := ConfigTemplate(mod, templatePath, endPaths[i], secretMode, s[dirIndex+1], s[dirIndex+2], cert)
			//generate template or certificate
			if !generatedCert && cert {
				writeToFile(certData[1], endDir+"/"+certData[0])
				generatedCert = true
				fmt.Println("certificate written to ", endDir)
				return
			} else if !cert {
				writeToFile(configuredTemplate, endPaths[i])
			}
		} else {
			//assume the starting directory was vault_templates
			configuredTemplate, certData := ConfigTemplate(mod, templatePath, endPaths[i], secretMode, s[1], s[2], cert)
			if !generatedCert && cert {
				writeToFile(certData[1], endDir+"/"+certData[0])
				generatedCert = true
				fmt.Println("certificate written to ", endDir)
				return
			} else if !cert {
				writeToFile(configuredTemplate, endPaths[i])
			}
		}
	}
	//print that we're done
	fmt.Println("templates configured and written to ", endDir)

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

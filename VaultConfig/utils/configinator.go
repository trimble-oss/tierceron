package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"bitbucket.org/dexterchaney/whoville/utils"
	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
)

//ConfigFromVault configures the templates in vault_templates and writes them to VaultConfig
func ConfigFromVault(token string, address string, cert []byte, env string, secretMode bool, servicesWanted []string, startDir string, templateDir string, endDir string) {

	mod, err := kv.NewModifier(token, address, cert)
	if err != nil {
		panic(err)
	}
	mod.Env = env

	templateFileDir := templateDir

	if startDir != "" {
		templateFileDir = startDir + templateDir
	}
	//get files from directory
	templateFilePaths, endPaths := getDirFiles(templateFileDir, endDir)
	//configure each template in directory
	for i, templateFilePath := range templateFilePaths {
		templatePath := templateFilePath
		if startDir != "" {
			templatePath = strings.Replace(templateFilePath, startDir, "", 1)
		}
		s := strings.Split(templatePath, "/")
		configuredTemplate := ConfigTemplate(mod, templateFilePath, templatePath, secretMode, s[1])
		writeToFile(configuredTemplate, endPaths[i])
	}
	//print that we're done
	endDir = strings.Split(endDir, "/")[0]
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

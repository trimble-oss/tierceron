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
func ConfigFromVault(token string, address string, certPath string, env string, secretMode bool, servicesWanted []string) {
	startDir := "vault_templates/"
	endDir := "config_files/"
	//certPath := "certs/cert_files/serv_cert.pem"
	//secretMode := true
	//servicesWanted := []string{}

	mod, err := kv.NewModifier(token, address, certPath)
	mod.Env = env
	if err != nil {
		panic(err)
	}
	//get files from directory
	templatePaths, endPaths := getDirFiles(startDir, endDir)
	//configure each template in directory
	for i, templatePath := range templatePaths {
		configuredTemplate := ConfigTemplate(mod, templatePath, endPaths[i], secretMode, servicesWanted...)
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
	utils.CheckError(err)
	//create new file
	newFile, err := os.Create(path)
	utils.CheckError(err)
	//write to file
	_, err = newFile.Write(byteData)
	utils.CheckError(err)
	newFile.Close()
}

//ConfigTemplate takes a modifier object, a file path where the template is located, the target path, and two maps of data to populate the template with.
//It configures the template and writes it to the specified file path.
func ConfigTemplate(modifier *kv.Modifier, emptyFilePath string, configuredFilePath string, secretMode bool, servicesWanted ...string) string {
	//get template
	emptyTemplate, err := ioutil.ReadFile(emptyFilePath)
	template := string(emptyTemplate)
	utils.CheckError(err)
	//populate template
	template = PopulateTemplate(template, modifier, secretMode, servicesWanted...)
	return template
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

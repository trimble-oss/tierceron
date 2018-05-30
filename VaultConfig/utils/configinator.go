package utils

import (
	"fmt"
	"io/ioutil"
	"os"

	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
)

//ConfigTemplates takes a file directory to read templates from and a directory to write templates to and configures the templates.
func ConfigTemplates(dir string, endDir string, modifier *kv.Modifier, dataPaths ...string) {
	//get files from directory
	fmt.Println(dataPaths)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		panic(err)
	}
	for _, f := range files {
		templatePath := dir + f.Name()
		endPath := endDir + f.Name()
		ConfigTemplate(modifier, templatePath, endPath, dataPaths...)
	}
	fmt.Println("templates configured")
	//config each template in directory
	//write files to end directory
}

//yaml config file: what the config template is, path to templates (configTemplates), target path
//pass host in on command line

//ConfigTemplate takes a modifier object, a file path where the template is located, the target path, and two maps of data to populate the ntemplate with
func ConfigTemplate(modifier *kv.Modifier, emptyFilePath string, configuredFilePath string, dataPaths ...string) {
	//fmt.Println(dataPaths)
	emptyTemplate, err := ioutil.ReadFile(emptyFilePath)
	template := string(emptyTemplate)
	if err != nil {
		panic(err)
	}
	//populate template and return
	//get public and private maps directly from vault
	//we just need to put values in the map that it's looking for?
	//template populates values with nil if there is no corresponding value. Have to use data from all dataPaths at once
	template = PopulateTemplate(template, modifier, dataPaths...)
	popTemplate := []byte(template)
	newFile, err := os.Create(configuredFilePath)
	if err != nil {
		panic(err)
	}
	newFile.Write(popTemplate)
	//save file to location
	//write to specific file type
}

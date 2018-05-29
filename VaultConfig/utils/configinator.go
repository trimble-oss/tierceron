package utils

import (
	"fmt"
	"io/ioutil"
	"os"

	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
)

//ConfigTemplate takes a modifier object, a file path where the template is located, the target path, and two maps of data to populate the ntemplate with
func ConfigTemplate(modifier *kv.Modifier, emptyFilePath string, configuredFilePath string, dataPath string) {

	emptyTemplate, err := ioutil.ReadFile(emptyFilePath)
	empTemplate := string(emptyTemplate)
	if err != nil {
		panic(err)
	}
	//populate template and return
	//get public and private maps directly from vault
	//we just need to put values in the map that it's looking for?
	populatedTemplate := PopulateTemplate(dataPath, empTemplate, modifier)
	popTemplate := []byte(populatedTemplate)
	newFile, err := os.Create(configuredFilePath)
	if err != nil {
		panic(err)
	}
	newFile.Write(popTemplate)
	fmt.Println("file successfully configured")
	//save file to location
	//write to specific file type
	//local file, applies vault values, write file
	//config.yaml
}

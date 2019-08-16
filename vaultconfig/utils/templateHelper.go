package utils

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"text/template"

	"bitbucket.org/dexterchaney/whoville/utils"
	"bitbucket.org/dexterchaney/whoville/vaulthelper/kv"
)

//ConfigTemplate takes a modifier object, a file path where the template is located, the target path, and two maps of data to populate the template with.
//It configures the template and writes it to the specified file path.
func ConfigTemplate(modifier *kv.Modifier, emptyFilePath string, configuredFilePath string, secretMode bool, project string, service string, cert bool) (string, map[int]string) {
	//get template
	emptyTemplate, err := ioutil.ReadFile(emptyFilePath)
	utils.CheckError(err, true)
	template := string(emptyTemplate)
	// cert map
	certData := make(map[int]string)

	// Construct path for vault
	s := strings.Split(emptyFilePath, "/")

	// Remove file extensions
	filename := s[len(s)-1][0:strings.LastIndex(s[len(s)-1], ".")]

	extra := ""
	// Please rework... Urg...
	for i, component := range s {
		if component == "vault_templates" {
			extra = ""
			continue
		}
		if component == project || component == service || component == "" || i == (len(s)-1) {
			continue
		}
		if extra == "" {
			extra = "/" + component
		} else {
			extra = extra + "/" + component
		}
	}
	filename = filename[0:strings.LastIndex(filename, ".")]

	if extra != "" {
		filename = extra + "/" + filename
	}
	//populate template
	template, certData = PopulateTemplate(template, modifier, secretMode, project, service, filename, cert)
	return template, certData
}

//PopulateTemplate takes an empty template and a modifier.
//It populates the template and returns it in a string.
func PopulateTemplate(emptyTemplate string, modifier *kv.Modifier, secretMode bool, project string, service string, filename string, cert bool) (string, map[int]string) {
	str := emptyTemplate
	cds := new(ConfigDataStore)
	cds.Init(modifier, secretMode, true, project, service)
	certData := make(map[int]string)
	if values, ok := cds.dataMap[service].(map[string]interface{}); ok {
		if cert {
			//Verify that config file and cert variables exist in cds map
			config, ok := values["config"].(map[string]interface{})
			if !ok {
				fmt.Println("No template named config in this service. Unable to generate cert.pfx")
				os.Exit(1)
			}
			name, ok := config["certDestPath"].(interface{})
			if !ok {
				fmt.Println("No certDestPath in config template section of seed for this service. Unable to generate cert.pfx")
				os.Exit(1)
			}
			certData[0] = name.(string)
			data, ok := config["certData"].(interface{})
			if !ok {
				fmt.Println("No certData in config template section of seed for this service. Unable to generate cert.pfx")
				os.Exit(1)
			}
			encoded := fmt.Sprintf("%s", data)
			//Decode cert as it was encoded in vaultinit
			decoded, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				panic(err)
			}
			certData[1] = fmt.Sprintf("%s", decoded)
			return "", certData
		}
		//create new template from template string
		fmt.Println("filename is " + filename)
		t := template.New("template")
		t, err := t.Parse(emptyTemplate)
		if err != nil {
			panic(err)
		}
		var doc bytes.Buffer
		//configure the template

		//Check if filename exists in values map
		_, data := values[filename]
		if data == false {
			fmt.Println("Filename does not exist in values. Please check seed files to verify that folder structures are correct.")
		}
		err = t.Execute(&doc, values[filename])
		str = doc.String()
		if err != nil {
			panic(err)
		}
	}
	return str, certData
}

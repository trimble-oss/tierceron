package utils

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"
	"text/template"

	"bitbucket.org/dexterchaney/whoville/utils"
	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
)

//ConfigTemplate takes a modifier object, a file path where the template is located, the target path, and two maps of data to populate the template with.
//It configures the template and writes it to the specified file path.
func ConfigTemplate(modifier *kv.Modifier, emptyFilePath string, configuredFilePath string, secretMode bool, service string) string {
	//get template
	emptyTemplate, err := ioutil.ReadFile(emptyFilePath)
	utils.CheckError(err, true)
	template := string(emptyTemplate)

	// Construct path for vault
	s := strings.Split(configuredFilePath, "/")
	// Remove file extensions
	filename := s[len(s)-1][0:strings.LastIndex(s[len(s)-1], ".")]
	extra := ""
	// Please rework... Urg...
	for i, component := range s {
		if component == "vault_templates" || component == service || component == "" || i == (len(s)-1) {
			continue
		}
		if extra == "" {
			extra = "/" + component
		} else {
			extra = extra + "/" + component
		}
	}
	filename = filename[0:strings.LastIndex(filename, ".")]
	vaultPath := service + extra + "/" + filename
	fmt.Printf("Vault path %s\n", vaultPath)

	if extra != "" {
		filename = extra + "/" + filename
	}

	//populate template
	template = PopulateTemplate(template, modifier, secretMode, service, filename)
	return template
}

//PopulateTemplate takes an empty template and a modifier.
//It populates the template and returns it in a string.
func PopulateTemplate(emptyTemplate string, modifier *kv.Modifier, secretMode bool, service string, filename string) string {
	str := emptyTemplate
	cds := new(ConfigDataStore)
	//fmt.Println(secretMode)
	cds.init(modifier, secretMode, true, service)
	if values, ok := cds.dataMap[service].(map[string]interface{}); ok {
		//os.Exit(0)
		//create new template from template string
		t := template.New("template")
		t, err := t.Parse(emptyTemplate)
		if err != nil {
			panic(err)
		}
		var doc bytes.Buffer
		//configure the template
		err = t.Execute(&doc, values[filename])
		str = doc.String()
		if err != nil {
			panic(err)
		}
	}
	return str
}

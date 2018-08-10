package utils

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"
	"text/template"

	"bitbucket.org/dexterchaney/whoville/utils"
	"bitbucket.org/dexterchaney/whoville/vaulthelper/kv"
)

//ConfigTemplate takes a modifier object, a file path where the template is located, the target path, and two maps of data to populate the template with.
//It configures the template and writes it to the specified file path.
func ConfigTemplate(modifier *kv.Modifier, emptyFilePath string, configuredFilePath string, secretMode bool, project string, service string) string {
	//get template
	emptyTemplate, err := ioutil.ReadFile(emptyFilePath)
	utils.CheckError(err, true)
	template := string(emptyTemplate)

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
	template = PopulateTemplate(template, modifier, secretMode, project, service, filename)
	return template
}

//PopulateTemplate takes an empty template and a modifier.
//It populates the template and returns it in a string.
func PopulateTemplate(emptyTemplate string, modifier *kv.Modifier, secretMode bool, project string, service string, filename string) string {
	fmt.Println("filename is " + filename)
	str := emptyTemplate
	cds := new(ConfigDataStore)
	cds.init(modifier, secretMode, true, project, service)
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

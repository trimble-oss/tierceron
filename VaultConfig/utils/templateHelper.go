package utils

import (
	"bytes"
	"html/template"
	"io/ioutil"

	"bitbucket.org/dexterchaney/whoville/utils"
	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
)

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

//PopulateTemplate takes an empty template and a modifier.
//It populates the template and returns it in a string.
func PopulateTemplate(emptyTemplate string, modifier *kv.Modifier, secretMode bool, servicesWanted ...string) string {
	cds := new(ConfigDataStore)
	cds.init(modifier, secretMode, servicesWanted...)
	//create new template from template string
	t := template.New("template")
	t, err := t.Parse(emptyTemplate)
	if err != nil {
		panic(err)
	}
	var doc bytes.Buffer
	//configure the template
	err = t.Execute(&doc, cds.dataMap)
	str := doc.String()
	if err != nil {
		panic(err)
	}
	return str
}

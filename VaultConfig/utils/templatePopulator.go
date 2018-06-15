package utils

import (
	"bytes"
	"html/template"

	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
)

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

package utils

import (
	"bytes"
	"html/template"

	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
)

//make it so it doesn't need the maps, just the vault?

//PopulateTemplate takes a map of public information, a map of private information, a file to write to, and a template string.
//It populates the template and returns it in a string.
func PopulateTemplate(emptyTemplate string, modifier *kv.Modifier, dataPaths ...string) string {
	dataMap := make(map[string]interface{})
	for _, path := range dataPaths {
		secrets, err := modifier.Read(path)
		if err != nil {
			panic(err)
		}
		for key, value := range secrets.Data {
			dataMap[key] = value
		}
	}

	t := template.New("template")
	t, err := t.Parse(emptyTemplate)
	if err != nil {
		panic(err)
	}
	var doc bytes.Buffer
	err = t.Execute(&doc, dataMap)
	str := doc.String()
	if err != nil {
		panic(err)
	}
	return str
}

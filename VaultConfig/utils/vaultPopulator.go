package utils

import (
	"bytes"
	"html/template"

	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
)

//make it so it doesn't need the maps, just the vault?

//DataMap holds a map of public data and private data.
type DataMap struct {
	Data map[string]interface{}
}

//PopulateTemplate takes a map of public information, a map of private information, a file to write to, and a template string.
//It populates the template and returns it in a string.
func PopulateTemplate(dataPath string, emptyTemplate string, modifier *kv.Modifier) string {
	dataMap := make(map[string]interface{})
	privateSecrets, err := modifier.Read(dataPath)
	for key, value := range privateSecrets.Data {
		dataMap[key] = value
	}
	myMap := DataMap{dataMap}
	t := template.New("template")
	t, err = t.Parse(emptyTemplate)
	if err != nil {
		panic(err)
	}
	var doc bytes.Buffer
	err = t.Execute(&doc, myMap)
	str := doc.String()
	if err != nil {
		panic(err)
	}
	return str
}

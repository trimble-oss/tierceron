package utils

import (
	"bytes"
	"html/template"
	"whoville/vault-helper/kv"
)

//make it so it doesn't need the maps, just the vault?

//Maps holds a map of public data and private data.
type Maps struct {
	PublicData  map[string]interface{}
	PrivateData map[string]interface{}
}

//PopulateTemplate takes a map of public information, a map of private information, a file to write to, and a template string.
//It populates the template and returns it in a string.
func PopulateTemplate(publicPath string, privatePath string, emptyTemplate string, modifier *kv.Modifier) string {
	privateMap := make(map[string]interface{})
	publicMap := make(map[string]interface{})
	privateSecrets, err := modifier.Read(privatePath)
	publicSecrets, err := modifier.Read(publicPath)
	for key, value := range privateSecrets.Data {
		privateMap[key] = value
	}
	for key, value := range publicSecrets.Data {
		publicMap[key] = value
	}
	myMap := Maps{publicMap, privateMap}
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

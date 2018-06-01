package utils

import (
	"bytes"
	"errors"
	"html/template"
	"reflect"

	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
)

//PopulateTemplate takes an empty template, a modifier, and the template data paths in the vault.
//It populates the template and returns it in a string.
func PopulateTemplate(emptyTemplate string, modifier *kv.Modifier, dataPaths ...string) string {
	dataMap := make(map[string]interface{})
	ogKeys := []string{}
	valuePaths := [][]string{}
	for _, path := range dataPaths {
		//for each path, read the secrets there
		secrets, err := modifier.ReadData(path)
		if err != nil {
			panic(err)
		}
		//get the keys and values in secrets
		for key, value := range secrets {
			if reflect.TypeOf(value) == reflect.TypeOf("") {
				//if it's a string, it's not the data we're looking for
			} else {
				ogKeys = append(ogKeys, key)
				newVal := value.([]interface{})
				newValues := []string{}
				for _, val := range newVal {
					newValues = append(newValues, val.(string))
				}
				valuePaths = append(valuePaths, newValues)
			}
		}
		for i, valuePath := range valuePaths {
			if len(valuePath) != 2 {
				panic(errors.New("value path is not the correct length"))
			} else {
				//first element is the path
				path := valuePath[0]
				//second element is the key
				key := valuePath[1]
				value := modifier.ReadValue(path, key)
				//put the original key with the correct value
				dataMap[ogKeys[i]] = value
			}
		}
	}
	//create new template from template string
	t := template.New("template")
	t, err := t.Parse(emptyTemplate)
	if err != nil {
		panic(err)
	}
	var doc bytes.Buffer
	//configure the template
	err = t.Execute(&doc, dataMap)
	str := doc.String()
	if err != nil {
		panic(err)
	}
	return str
}

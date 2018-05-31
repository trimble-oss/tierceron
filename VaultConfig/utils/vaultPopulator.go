package utils

import (
	"bytes"
	"fmt"
	"html/template"
	"reflect"

	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
)

//make it so it doesn't need the maps, just the vault?

//PopulateTemplate takes a map of public information, a map of private information, a file to write to, and a template string.
//It populates the template and returns it in a string.
func PopulateTemplate(emptyTemplate string, modifier *kv.Modifier, dataPaths ...string) string {
	dataMap := make(map[string]interface{})
	ogKeys := []string{}
	valuePaths := [][]string{}
	for _, path := range dataPaths {
		//for neach path, read the secrets there
		secrets, err := modifier.ReadData(path)
		if err != nil {
			panic(err)
		}
		fmt.Println(secrets)
		//get the keys and values in secrets
		for key, value := range secrets {
			if reflect.TypeOf(value) == reflect.TypeOf("") {
				//fmt.Println("type is string")
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
		fmt.Println("value paths: ", valuePaths)
		for i, valuePath := range valuePaths {
			//first element is the path
			if len(valuePath) != 2 {
				fmt.Println("value path length is ", len(valuePath))
			} else {
				path := valuePath[0]
				fmt.Println("path is", path)
				//second element is the key
				key := valuePath[1]
				fmt.Println("key is", key)
				value := modifier.ReadValue(path, key)
				fmt.Println(ogKeys[i], value)
				dataMap[ogKeys[i]] = value
			}
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

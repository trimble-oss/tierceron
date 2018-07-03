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
	template := string(emptyTemplate)
	utils.CheckError(err)

	// Construct path for vault
	s := strings.Split(emptyFilePath, "/")
	// Remove file extensions
	filename := s[2][0:strings.LastIndex(s[2], ".")]
	filename = filename[0:strings.LastIndex(filename, ".")]
	vaultPath := service + "/" + filename
	fmt.Printf("Vault path %s\n", vaultPath)

	//populate template
	template = PopulateTemplate(template, modifier, secretMode, service, filename)
	//fmt.Println(template)
	return template
}

//PopulateTemplate takes an empty template and a modifier.
//It populates the template and returns it in a string.
func PopulateTemplate(emptyTemplate string, modifier *kv.Modifier, secretMode bool, service string, filename string) string {
	str := emptyTemplate
	cds := new(ConfigDataStore)
	fmt.Println("Data Store:")
	cds.init(modifier, false, true, service)
	fmt.Printf("Service %s File %s\n", service, filename)
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

// func printMap(data map[string]interface{}, level int) {
// 	var keys []string
// 	for k := range data {
// 		keys = append(keys, k)
// 	}
// 	sort.Strings(keys)
// 	for _, k := range keys {
// 		for i := 0; i < level; i++ {
// 			fmt.Print("\t")
// 		}
// 		fmt.Printf("%-50s", k)
// 		if next, ok := data[k].(map[string]interface{}); ok {
// 			fmt.Println()
// 			printMap(next, level+1)
// 		} else {
// 			fmt.Println(data[k])
// 		}
// 	}
// }

package main

import (
	"flag"
	"html/template"
	"log"
	"os"
)

//Maps holds a map of public data and private data.
type Maps struct {
	PublicData  map[string]interface{}
	PrivateData map[string]interface{}
}

//var fakeTemplate = "{{range $index, $element := .}}{{$index}}{{range $element}}{{end}}"

/*
This Vault configurator app will read and populate templates
*/
func main() {
	flag.String("accesstoken", "", "vault access token")
	flag.Parse()
	map1 := map[string]interface{}{"myKey1": "myVal1", "fake": "data"}
	map2 := map[string]interface{}{"password": "123", "username": "user"}
	template := "public data: {{.PublicData}} private data: {{.PrivateData}}"
	target := "/mnt/c/Users/Sara.wille/workspace/example"
	populateTemplate(map1, map2, target, template)
}

func populateTemplate(publicMap map[string]interface{}, privateMap map[string]interface{}, targetLocation string, temp string) {
	myMap := Maps{publicMap, privateMap}
	t := template.New("practice template")
	t, err := t.Parse(temp)
	if err != nil {
		panic(err)
	}
	file, err := os.Create(targetLocation)
	if err != nil {
		log.Println("create file: ", err)
		return
	}
	//rand := Random{Data: "randy"}
	err = t.Execute(file, myMap)
	//return template
	if err != nil {
		panic(err)
	}
	file.Close()
	//os.Create might just override an existing file - try importing template file directly and see if that works?
}

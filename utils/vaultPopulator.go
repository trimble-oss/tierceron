package utils

import (
	"bytes"
	"html/template"
	"os"
)

//Maps holds a map of public data and private data.
type Maps struct {
	PublicData  map[string]interface{}
	PrivateData map[string]interface{}
}

//PopulateTemplate takes a map of public information, a map of private information, a file to write to, and a template string.
//It populates the template and writes it to the file.
func PopulateTemplate(publicMap map[string]interface{}, privateMap map[string]interface{}, file *os.File, temp string) string {
	myMap := Maps{publicMap, privateMap}
	t := template.New("practice template")
	t, err := t.Parse(temp)
	if err != nil {
		panic(err)
	}
	var doc bytes.Buffer
	err = t.Execute(&doc, myMap)
	str := doc.String()
	//return template
	if err != nil {
		panic(err)
	}
	file.Close()
	//b, err := ioutil.ReadAll(file)
	//str := string(b)
	//fmt.Println("printing string: ", str)
	return str
}

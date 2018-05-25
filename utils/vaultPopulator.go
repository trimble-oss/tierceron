package utils

import (
	"html/template"
	"os"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/codedeploy"
)

//Maps holds a map of public data and private data.
type Maps struct {
	PublicData  map[string]interface{}
	PrivateData map[string]interface{}
}
S3_REGION = ""
S3_BUCKET = ""
//PopulateTemplate takes a map of public information, a map of private information, a file to write to, and a template string.
//It populates the template and writes it to the file.
func PopulateTemplate(publicMap map[string]interface{}, privateMap map[string]interface{}, file *os.File, temp string) {
	myMap := Maps{publicMap, privateMap}
	t := template.New("practice template")
	t, err := t.Parse(temp)
	if err != nil {
		panic(err)
	}
	err = t.Execute(file, myMap)
	//return template
	if err != nil {
		panic(err)
	}
	file.Close()
}

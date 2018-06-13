package main

import (
	"context"
	"fmt"
	"net/http"

	"bitbucket.org/dexterchaney/whoville/twirpapi/rpc/templatesapi"
	"bitbucket.org/dexterchaney/whoville/utils"
)

// Small twirp client made to test the twirp server endpoints

func main() {
	client := templatessapi.NewTemplatesProtobufClient("http://localhost:8080", &http.Client{})

	req := &templatessapi.TemplateReq{
		Service: "ST",
		File:    "hibernate"}

	res, err := client.GetTemplate(context.Background(), req)
	utils.CheckError(err)
	fmt.Printf("File:\n%s\n", res.Data)
	fmt.Printf("Ext: %s\n", res.Ext)
}

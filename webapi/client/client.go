package main

import (
	"context"
	"fmt"
	"net/http"

	"bitbucket.org/dexterchaney/whoville/utils"
	pb "bitbucket.org/dexterchaney/whoville/webapi/rpc/twirpapi"
)

// Small twirp tmpClient made to test the twirp server endpoints

func main() {
	// tmpClient := tmp.NewTemplatesProtobufClient("http://localhost:8080", &http.Client{})
	apiClient := pb.NewTwirpAPIProtobufClient("http://localhost:8080", &http.Client{})

	templateReq := &pb.TemplateReq{
		Service: "ST",
		File:    "hibernate",
	}

	validReq := &pb.ValidationReq{
		Service: "ServiceTechDB",
		Env:     "dev",
	}

	templateRes, err := apiClient.GetTemplate(context.Background(), templateReq)
	utils.CheckError(err)

	validRes, err := apiClient.Validate(context.Background(), validReq)
	utils.CheckError(err)

	fmt.Printf("Template: \nService:\t%s\n File:\t%s\n Data:\n%s\nExt:\t%s\n\n", templateReq.Service, templateReq.File, templateRes.Data, templateRes.Ext)
	fmt.Printf("Validity: \nService:\t%s\n Env:\t%s\n Valid:\t%v\n", validReq.Service, validReq.Env, validRes.IsValid)
}

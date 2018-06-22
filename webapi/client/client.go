package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"

	"bitbucket.org/dexterchaney/whoville/utils"
	pb "bitbucket.org/dexterchaney/whoville/webapi/rpc/apinator"
)

func main() {
	addrPtr := flag.String("addr", "http://127.0.0.1:8080", "API endpoint for the vault")
	apiClient := pb.NewEnterpriseServiceBrokerProtobufClient(*addrPtr, &http.Client{})

	//templates := getTemplates("ST")
	// templateReq := &pb.TemplateReq{
	// 	Service: "ST",
	// 	File:    "hibernate",
	// }

	// validReq := &pb.ValidationReq{
	// 	Service: "ServiceTechDB",
	// 	Env:     "QA",
	// }

	// listReq := &pb.ListReq{
	// 	Service: "ST",
	// }

	// servicePathReq := &pb.ServicePathReq{
	// 	Service: "values",
	// 	Env:     "dev",
	// }

	makeVaultReq := &pb.MakeVaultReq{}

	vault, err := apiClient.MakeVault(context.Background(), makeVaultReq)
	utils.CheckError(err)

	// templateRes, err := apiClient.GetTemplate(context.Background(), templateReq)
	// utils.CheckError(err)

	// validRes, err := apiClient.Validate(context.Background(), validReq)
	// utils.CheckError(err)

	// listRes, err := apiClient.ListServiceTemplates(context.Background(), listReq)
	// utils.CheckError(err)

	// servicePathRes, err := apiClient.GetServicePaths(context.Background(), servicePathReq)
	// utils.CheckError(err)

	// for _, path := range {
	// 	pathDataReq := &pb.PathDataReq{
	// 		Paths: servicePathRes.Paths,
	// 		Env:   "dev",
	// 	}
	// }

	//pathDataRes, err := apiClient.GetPathData(context.Background(), pathDataReq)
	//utils.CheckError(err)

	//fmt.Printf("Template: \nService:\t%s\n File:\t%s\n Data:\n%s\nExt:\t%s\n\n", templateReq.Service, templateReq.File, templateRes.Data, templateRes.Ext)
	//fmt.Println("")
	//fmt.Printf("Validity: \nService:\t%s\n Env:\t%s\n Valid:\t%v\n", validReq.Service, validReq.Env, validRes.IsValid)
	//fmt.Println("")
	//fmt.Printf("List: \nService:\t%s\nResult:\t%v\n", listReq.Service, listRes.Templates)
	//fmt.Println("")
	//fmt.Printf("List service paths: \nService:\t%s\nEnv:\t%s\nResult:\t%v\n", servicePathReq.Service, servicePathReq.Env, servicePathRes.Paths)
	//fmt.Println("")
	//fmt.Printf("List path data: \nPaths:\t%s\nEnv:\t%s\nResult:\t%v\n", pathDataReq.Paths, pathDataReq.Env, pathDataRes.Data)
	fmt.Printf("Vault: \n")
	for _, env := range vault.Envs {
		fmt.Printf("Env: %s\n", env.Name)
		for _, service := range env.Services {
			fmt.Printf("\tService: %s\n", service.Name)
			for _, file := range service.Files {
				fmt.Printf("\t\tFile: %s\n", file.Name)
				for _, val := range file.Values {
					fmt.Printf("\t\t\tkey: %s\tvalue: %s\n", val.Key, val.Value)
				}
			}
		}
	}
}

// func getTemplates(service string) []string {
// 	templates := []string{}
// 	listReq := &pb.ListReq{
// 		Service: service,
// 	}
// 	listRes, err := apiClient.ListServiceTemplates(context.Background(), listReq)
// 	utils.CheckError(err)
// 	for _, file := range listRes.Templates {
// 		templateReq := &pb.TemplateReq{
// 			Service: service,
// 			File:    file,
// 		}
// 		templateRes, err := apiClient.GetTemplate(context.Background(), templateReq)
// 		utils.CheckError(err)
// 		templateString := string(b64.Encoding.DecodeString(templateRes.Data))
// 		templates = append(templates, templateString)
// 		//turn template back to a string
// 	}
// 	return templates
// }

// func getServiceData(env ){

// }

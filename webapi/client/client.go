package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"

	eUtils "github.com/trimble-oss/tierceron/utils"
	pb "github.com/trimble-oss/tierceron/webapi/rpc/apinator"
)

func main() {
	addrPtr := flag.String("addr", "http://127.0.0.1:8008", "API endpoint for the vault")
	apiClient := pb.NewEnterpriseServiceBrokerProtobufClient(*addrPtr, &http.Client{})

	makeVaultReq := &pb.GetValuesReq{}
	config := &eUtils.DriverConfig{ExitOnFailure: true}

	vault, err := apiClient.GetValues(context.Background(), makeVaultReq)
	eUtils.CheckError(config, err, true)

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

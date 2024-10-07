package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"

	"github.com/trimble-oss/tierceron/pkg/core"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	pb "github.com/trimble-oss/tierceron/trcweb/rpc/apinator"
)

func main() {
	addrPtr := flag.String("addr", "http://127.0.0.1:8008", "API endpoint for the vault")
	apiClient := pb.NewEnterpriseServiceBrokerProtobufClient(*addrPtr, &http.Client{})

	makeVaultReq := &pb.GetValuesReq{}
	driverConfig := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			ExitOnFailure: true,
		},
	}

	vault, err := apiClient.GetValues(context.Background(), makeVaultReq)
	eUtils.CheckError(driverConfig.CoreConfig, err, true)

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

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	eUtils "Vault.Whoville/utils"
	"Vault.Whoville/vaultconfig/utils"
)

func main() {
	fmt.Println("Version: " + "1.14")
	addrPtr := flag.String("addr", "", "API endpoint for the vault")
	tokenPtr := flag.String("token", "", "Vault access token")
	startDirPtr := flag.String("startDir", "vault_templates", "Template directory")
	endDirPtr := flag.String("endDir", ".", "Directory to put configured templates into")
	envPtr := flag.String("env", "dev", "Environment to configure")
	regionPtr := flag.String("region", "", "Region to configure")
	secretMode := flag.Bool("secretMode", true, "Only override secret values in templates?")
	servicesWanted := flag.String("servicesWanted", "", "Services to pull template values for, in the form 'service1,service2' (defaults to all services)")
	secretIDPtr := flag.String("secretID", "", "Secret app role ID")
	appRoleIDPtr := flag.String("appRoleID", "", "Public app role ID")
	tokenNamePtr := flag.String("tokenName", "", "Token name used by this vaultconfig to access the vault")
	wantCertPtr := flag.Bool("cert", false, "Pull certificate into directory specified by endDirPtr")
	logFilePtr := flag.String("log", "./vaultconfig.log", "Output path for log file")
	pingPtr := flag.Bool("ping", false, "Ping vault.")
	zcPtr := flag.Bool("zc", false, "Zero config (no configuration option).")

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		s := args[i]
		if s[0] != '-' {
			fmt.Println("Wrong flag syntax: ", s)
			os.Exit(1)
		}
	}
	flag.Parse()

	if _, err := os.Stat(*startDirPtr); os.IsNotExist(err) {
		fmt.Println("Missing required template folder: " + *startDirPtr)
		os.Exit(1)
	}

	if *zcPtr {
		*wantCertPtr = false
	}

	eUtils.AutoAuth(secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, *pingPtr)

	if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
		var err error
		*envPtr, err = eUtils.LoginToLocal()
		fmt.Println(*envPtr)
		eUtils.CheckError(err, true)
	}
	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	eUtils.CheckError(err, true)
	logger := log.New(f, "[vaultconfig]", log.LstdFlags)
	services := []string{}
	if *servicesWanted != "" {
		services = strings.Split(*servicesWanted, ",")
	}

	for _, service := range services {
		service = strings.TrimSpace(service)
	}
	regions := []string{}

	if *envPtr == "staging" || *envPtr == "prod" {
		supportedRegions := eUtils.GetSupportedProdRegions()
		if *regionPtr != "" {
			for _, supportedRegion := range supportedRegions {
				if *regionPtr == supportedRegion {
					regions = append(regions, *regionPtr)
					break
				}
			}
			if len(regions) == 0 {
				fmt.Println("Unsupported region: " + *regionPtr)
				os.Exit(1)
			}
		}
	}

	config := eUtils.DriverConfig{
		Token:          *tokenPtr,
		VaultAddress:   *addrPtr,
		Env:            *envPtr,
		Regions:        regions,
		SecretMode:     *secretMode,
		ServicesWanted: services,
		StartDir:       append([]string{}, *startDirPtr),
		EndDir:         *endDirPtr,
		WantCert:       *wantCertPtr,
		ZeroConfig:     *zcPtr,
		GenAuth:        false,
		Log:            logger,
	}
	eUtils.ConfigControl(config, utils.GenerateConfigsFromVault)
}

package vaultxbase

import (
	"flag"
	"fmt"
	"log"
	"os"

	eUtils "Vault.Whoville/utils"
	"Vault.Whoville/vaultx/xutil"
)

// CommonMain This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func CommonMain(envPtr *string, addrPtrIn *string) {
	// Executable input arguments(flags)
	addrPtr := flag.String("addr", "", "API endpoint for the vault")
	if addrPtrIn != nil && *addrPtrIn != "" {
		addrPtr = addrPtrIn
	}
	startDirPtr := flag.String("startDir", "vault_templates", "Pull templates from this directory")
	endDirPtr := flag.String("endDir", "./vault_seeds/", "Write generated seed files to this directory")
	logFilePtr := flag.String("log", "./var/log/vaultx.log", "Output path for log file")
	helpPtr := flag.Bool("h", false, "Provide options for vaultx")
	tokenPtr := flag.String("token", "", "Vault access token")
	secretMode := flag.Bool("secretMode", true, "Only override secret values in templates?")
	genAuth := flag.Bool("genAuth", false, "Generate auth section of seed data?")
	cleanPtr := flag.Bool("clean", false, "Cleans seed files locally")
	secretIDPtr := flag.String("secretID", "", "Secret app role ID")
	appRoleIDPtr := flag.String("appRoleID", "", "Public app role ID")
	tokenNamePtr := flag.String("tokenName", "", "Token name used by this vaultx to access the vault")
	noVaultPtr := flag.Bool("novault", false, "Don't pull configuration data from vault.")
	pingPtr := flag.Bool("ping", false, "Ping vault.")

	// Checks for proper flag input
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		s := args[i]
		if s[0] != '-' {
			fmt.Println("Wrong flag syntax: ", s)
			os.Exit(1)
		}
	}

	flag.Parse()

	// Prints usage if no flags are specified
	if *helpPtr {
		flag.Usage()
		os.Exit(1)
	}
	if _, err := os.Stat(*startDirPtr); os.IsNotExist(err) {
		fmt.Println("Missing required start template folder: " + *startDirPtr)
		os.Exit(1)
	}
	if _, err := os.Stat(*endDirPtr); os.IsNotExist(err) {
		fmt.Println("Missing required start seed folder: " + *endDirPtr)
		os.Exit(1)
	}

	// If logging production directory does not exist and is selected log to local directory
	if _, err := os.Stat("./var/log/"); os.IsNotExist(err) && *logFilePtr == "./var/log/vaultx.log" {
		*logFilePtr = "./vaultx.log"
	}

	regions := []string{}

	if !*noVaultPtr {
		if *envPtr == "staging" || *envPtr == "prod" {
			secretIDPtr = nil
			appRoleIDPtr = nil
			regions = eUtils.GetSupportedProdRegions()
		}
		eUtils.AutoAuth(secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, *pingPtr)
	}

	if tokenPtr == nil || *tokenPtr == "" && !*noVaultPtr {
		fmt.Println("Missing required auth token.")
		os.Exit(1)
	}

	if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
		var err error
		*envPtr, err = eUtils.LoginToLocal()
		fmt.Println(*envPtr)
		eUtils.CheckError(err, true)
	}

	// Initialize logging
	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	eUtils.CheckError(err, true)
	logger := log.New(f, "[vaultx]", log.LstdFlags)
	logger.Println("=============== Initializing Seed Generator ===============")

	logger.SetPrefix("[vaultx]")
	logger.Printf("Looking for template(s) in directory: %s\n", *startDirPtr)

	config := eUtils.DriverConfig{
		Token:          *tokenPtr,
		VaultAddress:   *addrPtr,
		Env:            *envPtr,
		Regions:        regions,
		SecretMode:     *secretMode,
		ServicesWanted: []string{},
		StartDir:       append([]string{}, *startDirPtr),
		EndDir:         *endDirPtr,
		WantCert:       false,
		GenAuth:        *genAuth,
		Log:            logger,
		Diff:           *cleanPtr,
	}
	eUtils.ConfigControl(config, xutil.GenerateSeedsFromVault)

	logger.SetPrefix("[vaultx]")
	logger.Println("=============== Terminating Seed Generator ===============")
	logger.SetPrefix("[END]")
	logger.Println()

	// Terminate logging
	f.Close()
}

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	trcname "tierceron/trcvault/opts/trcname"

	il "tierceron/trcinit/initlib"
	"tierceron/utils"
	eUtils "tierceron/utils"
	"tierceron/vaulthelper/kv"

	configcore "VaultConfig.Bootstrap/configcore"
)

// Reads in template files in specified directory
// Template directory should contain directories for each service
// Templates are uploaded to templates/<service>/<file name>/template-file
// The file is saved under the data key, and the extension under the ext key
// Vault automatically encodes the file into base64

func main() {
	fmt.Println("Version: " + "1.3")
	dirPtr := flag.String("dir", trcname.GetFolderPrefix()+"_templates", "Directory containing template files for vault")
	envPtr := flag.String("env", "dev", "Environement in vault")
	addrPtr := flag.String("addr", configcore.VaultHostPort, "API endpoint for the vault")
	tokenPtr := flag.String("token", "", "Vault access token")
	secretIDPtr := flag.String("secretID", "", "Public app role ID")
	appRoleIDPtr := flag.String("appRoleID", "", "Secret app role ID")
	tokenNamePtr := flag.String("tokenName", "", "Token name used by this "+trcname.GetFolderPrefix()+"pub to access the vault")
	pingPtr := flag.Bool("ping", false, "Ping vault.")
	insecurePtr := flag.Bool("insecure", false, "By default, every ssl connection is secure.  Allows to continue with server connections considered insecure.")
	logFilePtr := flag.String("log", "./"+trcname.GetFolderPrefix()+"sub.log", "Output path for log files")
	projectInfoPtr := flag.Bool("projectInfo", false, "Lists all project info")
	filterTemplatePtr := flag.String("templateFilter", "", "Specifies which templates to filter")

	flag.Parse()

	if len(*filterTemplatePtr) == 0 && !*projectInfoPtr {
		fmt.Printf("Must specify either -projectInfo or -templateFilter flag \n")
		os.Exit(1)
	}

	// If logging production directory does not exist and is selected log to local directory
	if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/"+trcname.GetFolderPrefix()+"sub.log" {
		*logFilePtr = "./" + trcname.GetFolderPrefix() + "sub.log"
	}
	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)

	logger := log.New(f, "[INIT]", log.LstdFlags)
	config := &eUtils.DriverConfig{Insecure: *insecurePtr, Log: logger, ExitOnFailure: true}
	eUtils.CheckError(config, err, true)

	if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
		var err error
		*envPtr, err = utils.LoginToLocal()
		fmt.Println(*envPtr)
		utils.CheckError(config, err, true)
	}

	fmt.Printf("Connecting to vault @ %s\n", *addrPtr)

	autoErr := eUtils.AutoAuth(config, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, *pingPtr)
	if autoErr != nil {
		fmt.Println("Missing auth components.")
		os.Exit(1)
	}
	mod, err := kv.NewModifier(*insecurePtr, *tokenPtr, *addrPtr, *envPtr, nil, logger)
	utils.CheckError(config, err, true)
	mod.Env = *envPtr

	if *projectInfoPtr {
		templateList, err := mod.List("templates/")
		if err != nil {
			utils.CheckError(config, err, true)
		}
		fmt.Printf("\nProjects available:\n")
		for _, templatePath := range templateList.Data {
			for _, projectInterface := range templatePath.([]interface{}) {
				project := projectInterface.(string)
				fmt.Println(strings.TrimRight(project, "/"))
			}
		}
		os.Exit(1)
	} else {
		fmt.Printf("Downloading templates from vault to %s\n", *dirPtr)
		// The actual download templates goes here.
		err, warn := il.DownloadTemplateDirectory(config, mod, *dirPtr, logger, filterTemplatePtr)
		if err != nil {
			fmt.Println(err)
			if strings.Contains(err.Error(), "x509: certificate") {
				os.Exit(-1)
			}
		}
		utils.CheckError(config, err, true)
		utils.CheckWarnings(config, warn, true)
	}
}

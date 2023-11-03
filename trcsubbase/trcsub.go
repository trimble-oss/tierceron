package trcsubbase

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	il "github.com/trimble-oss/tierceron/trcinit/initlib"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
	eUtils "github.com/trimble-oss/tierceron/utils"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
)

// Reads in template files in specified directory
// Template directory should contain directories for each service
// Templates are uploaded to templates/<service>/<file name>/template-file
// The file is saved under the data key, and the extension under the ext key
// Vault automatically encodes the file into base64

func CommonMain(envPtr *string, addrPtr *string, envCtxPtr *string) {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	fmt.Println("Version: " + "1.4")
	dirPtr := flag.String("dir", coreopts.GetFolderPrefix(nil)+"_templates", "Directory containing template files for vault")
	tokenPtr := flag.String("token", "", "Vault access token")
	secretIDPtr := flag.String("secretID", "", "Public app role ID")
	appRoleIDPtr := flag.String("appRoleID", "", "Secret app role ID")
	tokenNamePtr := flag.String("tokenName", "", "Token name used by this "+coreopts.GetFolderPrefix(nil)+"pub to access the vault")
	pingPtr := flag.Bool("ping", false, "Ping vault.")
	insecurePtr := flag.Bool("insecure", false, "By default, every ssl connection is secure.  Allows to continue with server connections considered insecure.")
	logFilePtr := flag.String("log", "./"+coreopts.GetFolderPrefix(nil)+"sub.log", "Output path for log files")
	projectInfoPtr := flag.Bool("projectInfo", false, "Lists all project info")
	filterTemplatePtr := flag.String("templateFilter", "", "Specifies which templates to filter")
	templatePathsPtr := flag.String("templatePaths", "", "Specifies which specific templates to download.")

	flag.Parse()

	if len(*filterTemplatePtr) == 0 && !*projectInfoPtr && *templatePathsPtr == "" {
		fmt.Printf("Must specify either -projectInfo or -templateFilter flag \n")
		os.Exit(1)
	}

	// If logging production directory does not exist and is selected log to local directory
	if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/"+coreopts.GetFolderPrefix(nil)+"sub.log" {
		*logFilePtr = "./" + coreopts.GetFolderPrefix(nil) + "sub.log"
	}
	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)

	logger := log.New(f, "[INIT]", log.LstdFlags)
	config := &eUtils.DriverConfig{Insecure: *insecurePtr, Log: logger, ExitOnFailure: true}
	eUtils.CheckError(config, err, true)

	if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
		var err error
		*envPtr, err = eUtils.LoginToLocal()
		fmt.Println(*envPtr)
		eUtils.CheckError(config, err, true)
	}

	fmt.Printf("Connecting to vault @ %s\n", *addrPtr)

	autoErr := eUtils.AutoAuth(config, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, envCtxPtr, "", *pingPtr)
	if autoErr != nil {
		fmt.Println("Missing auth components.")
		os.Exit(1)
	}
	if memonly.IsMemonly() {
		memprotectopts.MemUnprotectAll(nil)
		memprotectopts.MemProtect(nil, tokenPtr)
	}

	mod, err := helperkv.NewModifier(*insecurePtr, *tokenPtr, *addrPtr, *envPtr, nil, true, logger)
	if mod != nil {
		defer mod.Release()
	}
	eUtils.CheckError(config, err, true)
	mod.Env = *envPtr

	if *templatePathsPtr != "" {
		fmt.Printf("Downloading templates from vault to %s\n", *dirPtr)
		// The actual download templates goes here.
		il.DownloadTemplates(config, mod, *dirPtr, logger, templatePathsPtr)
	} else if *projectInfoPtr {
		templateList, err := mod.List("templates/", logger)
		if err != nil {
			eUtils.CheckError(config, err, true)
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
		warn, err := il.DownloadTemplateDirectory(config, mod, *dirPtr, logger, filterTemplatePtr)
		if err != nil {
			fmt.Println(err)
			if strings.Contains(err.Error(), "x509: certificate") {
				os.Exit(-1)
			}
		}
		eUtils.CheckError(config, err, true)
		eUtils.CheckWarnings(config, warn, true)
	}
}

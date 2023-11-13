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

func CommonMain(envPtr *string, addrPtr *string, envCtxPtr *string,
	secretIDPtr *string,
	appRoleIDPtr *string,
	c *eUtils.DriverConfig) error {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	fmt.Println("Version: " + "1.5")
	dirPtr := flag.String("dir", coreopts.GetFolderPrefix(nil)+"_templates", "Directory containing template files for vault")
	tokenPtr := flag.String("token", "", "Vault access token")
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
	var config *eUtils.DriverConfig
	var appRoleConfigPtr *string

	if c != nil {
		config = c
		appRoleConfigPtr = &c.AppRoleConfig

	} else {
		// If logging production directory does not exist and is selected log to local directory
		if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/"+coreopts.GetFolderPrefix(nil)+"sub.log" {
			*logFilePtr = "./" + coreopts.GetFolderPrefix(nil) + "sub.log"
		}
		f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			fmt.Println("Log init failure")
			return err
		}

		logger := log.New(f, "[INIT]", log.LstdFlags)
		config = &eUtils.DriverConfig{Insecure: *insecurePtr, Log: logger, ExitOnFailure: true}
		appRoleConfigPtr = new(string)
	}

	if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
		var err error
		*envPtr, err = eUtils.LoginToLocal()
		fmt.Println(*envPtr)
		eUtils.CheckError(config, err, false)
		return err
	}

	fmt.Printf("Connecting to vault @ %s\n", *addrPtr)

	autoErr := eUtils.AutoAuth(config, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, envCtxPtr, *appRoleConfigPtr, *pingPtr)
	if autoErr != nil {
		fmt.Println("Missing auth components.")
		return autoErr
	}
	if memonly.IsMemonly() {
		memprotectopts.MemUnprotectAll(nil)
		memprotectopts.MemProtect(nil, tokenPtr)
	}

	mod, err := helperkv.NewModifier(*insecurePtr, *tokenPtr, *addrPtr, *envPtr, nil, true, config.Log)
	if mod != nil {
		defer mod.Release()
	}
	if err != nil {
		fmt.Println("Failure to init to vault")
		config.Log.Println("Failure to init to vault")
		return err
	}
	mod.Env = *envPtr

	if *templatePathsPtr != "" {
		fmt.Printf("Downloading templates from vault to %s\n", *dirPtr)
		// The actual download templates goes here.
		il.DownloadTemplates(config, mod, *dirPtr, config.Log, templatePathsPtr)
	} else if *projectInfoPtr {
		templateList, err := mod.List("templates/", config.Log)
		if err != nil {
			fmt.Println("Failure read templates")
			config.Log.Println("Failure read templates")
			return err
		}
		fmt.Printf("\nProjects available:\n")
		for _, templatePath := range templateList.Data {
			for _, projectInterface := range templatePath.([]interface{}) {
				project := projectInterface.(string)
				fmt.Println(strings.TrimRight(project, "/"))
			}
		}
		return nil
	} else {
		fmt.Printf("Downloading templates from vault to %s\n", *dirPtr)
		// The actual download templates goes here.
		warn, err := il.DownloadTemplateDirectory(config, mod, *dirPtr, config.Log, filterTemplatePtr)
		if err != nil {
			fmt.Println(err)
			config.Log.Printf("Failure to download: %s", err.Error())
			if strings.Contains(err.Error(), "x509: certificate") {
				return err
			}
		}
		eUtils.CheckError(config, err, false)
		eUtils.CheckWarnings(config, warn, false)
	}
	return nil
}

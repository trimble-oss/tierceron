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
	flagset *flag.FlagSet,
	argLines []string,
	c *eUtils.DriverConfig) error {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	fmt.Println("Version: " + "1.6")

	if flagset == nil {
		flagset = flag.NewFlagSet(argLines[0], flag.ExitOnError)
		flagset.Usage = flag.Usage
		flagset.String("env", "dev", "Environment to configure")
		flagset.String("addr", "", "API endpoint for the vault")
		flagset.String("secretID", "", "Public app role ID")
		flagset.String("appRoleID", "", "Secret app role ID")
	}
	endDirPtr := flagset.String("endDir", coreopts.GetFolderPrefix(nil)+"_templates", "Directory to put configured templates into")
	tokenPtr := flagset.String("token", "", "Vault access token")
	tokenNamePtr := flagset.String("tokenName", "", "Token name used by this "+coreopts.GetFolderPrefix(nil)+"pub to access the vault")
	pingPtr := flagset.Bool("ping", false, "Ping vault.")
	insecurePtr := flagset.Bool("insecure", false, "By default, every ssl connection is secure.  Allows to continue with server connections considered insecure.")
	logFilePtr := flagset.String("log", "./"+coreopts.GetFolderPrefix(nil)+"sub.log", "Output path for log files")
	projectInfoPtr := flagset.Bool("projectInfo", false, "Lists all project info")
	filterTemplatePtr := flagset.String("templateFilter", "", "Specifies which templates to filter")
	templatePathsPtr := flagset.String("templatePaths", "", "Specifies which specific templates to download.")

	flagset.Parse(argLines[1:])

	if len(*filterTemplatePtr) == 0 && !*projectInfoPtr && *templatePathsPtr == "" {
		fmt.Printf("Must specify either -projectInfo or -templateFilter flag \n")
		os.Exit(1)
	}
	var configBase *eUtils.DriverConfig
	var appRoleConfigPtr *string

	if c != nil {
		configBase = c
		if len(configBase.EndDir) == 0 && len(*endDirPtr) != 0 {
			// Bad inputs... use default.
			configBase.EndDir = *endDirPtr
		}
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
		configBase = &eUtils.DriverConfig{Insecure: *insecurePtr,
			EndDir:        *endDirPtr,
			Log:           logger,
			ExitOnFailure: true}
		appRoleConfigPtr = new(string)
	}

	if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
		var err error
		*envPtr, err = eUtils.LoginToLocal()
		fmt.Println(*envPtr)
		eUtils.CheckError(configBase, err, false)
		return err
	}

	fmt.Printf("Connecting to vault @ %s\n", *addrPtr)

	autoErr := eUtils.AutoAuth(configBase, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, envCtxPtr, *appRoleConfigPtr, *pingPtr)
	if autoErr != nil {
		fmt.Println("Missing auth components.")
		return autoErr
	}
	if memonly.IsMemonly() {
		memprotectopts.MemUnprotectAll(nil)
		memprotectopts.MemProtect(nil, tokenPtr)
	}

	mod, err := helperkv.NewModifier(*insecurePtr, *tokenPtr, *addrPtr, *envPtr, nil, true, configBase.Log)
	if mod != nil {
		defer mod.Release()
	}
	if err != nil {
		fmt.Println("Failure to init to vault")
		configBase.Log.Println("Failure to init to vault")
		return err
	}
	mod.Env = *envPtr

	if *templatePathsPtr != "" {
		fmt.Printf("Downloading templates from vault to %s\n", configBase.EndDir)
		// The actual download templates goes here.
		il.DownloadTemplates(configBase, mod, configBase.EndDir, configBase.Log, templatePathsPtr)
	} else if *projectInfoPtr {
		templateList, err := mod.List("templates/", configBase.Log)
		if err != nil {
			fmt.Println("Failure read templates")
			configBase.Log.Println("Failure read templates")
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
		fmt.Printf("Downloading templates from vault to %s\n", configBase.EndDir)
		// The actual download templates goes here.
		warn, err := il.DownloadTemplateDirectory(configBase, mod, configBase.EndDir, configBase.Log, filterTemplatePtr)
		if err != nil {
			fmt.Println(err)
			configBase.Log.Printf("Failure to download: %s", err.Error())
			if strings.Contains(err.Error(), "x509: certificate") {
				return err
			}
		}
		eUtils.CheckError(configBase, err, false)
		eUtils.CheckWarnings(configBase, warn, false)
	}
	return nil
}

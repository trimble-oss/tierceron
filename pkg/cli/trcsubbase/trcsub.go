package trcsubbase

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/pkg/core"
	il "github.com/trimble-oss/tierceron/pkg/trcinit/initlib"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
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
	driverConfig *eUtils.DriverConfig) error {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}

	exitOnFailure := false
	if flagset == nil {
		fmt.Println("Version: " + "1.6")
		flagset = flag.NewFlagSet(argLines[0], flag.ExitOnError)
		flagset.Usage = func() {
			fmt.Fprintf(flagset.Output(), "Usage of %s:\n", argLines[0])
			flagset.PrintDefaults()
		}
		flagset.String("env", "dev", "Environment to configure")
		flagset.String("addr", "", "API endpoint for the vault")
		flagset.String("secretID", "", "Secret for app role ID")
		flagset.String("appRoleID", "", "Public app role ID")
	}
	exitOnFailure = true
	endDirPtr := flagset.String("endDir", coreopts.BuildOptions.GetFolderPrefix(nil)+"_templates", "Directory to put configured templates into")
	tokenPtr := flagset.String("token", "", "Vault access token")
	tokenNamePtr := flagset.String("tokenName", "", "Token name used by this "+coreopts.BuildOptions.GetFolderPrefix(nil)+"pub to access the vault")
	pingPtr := flagset.Bool("ping", false, "Ping vault.")
	insecurePtr := flagset.Bool("insecure", false, "By default, every ssl connection is secure.  Allows to continue with server connections considered insecure.")
	logFilePtr := flagset.String("log", "./"+coreopts.BuildOptions.GetFolderPrefix(nil)+"sub.log", "Output path for log files")
	projectInfoPtr := flagset.Bool("projectInfo", false, "Lists all project info")
	filterTemplatePtr := flagset.String("templateFilter", "", "Specifies which templates to filter")
	templatePathsPtr := flagset.String("templatePaths", "", "Specifies which specific templates to download.")

	flagset.Parse(argLines[1:])

	if len(*filterTemplatePtr) == 0 && !*projectInfoPtr && *templatePathsPtr == "" {
		fmt.Printf("Must specify either -projectInfo or -templateFilter flag \n")
		return errors.New("must specify either -projectInfo or -templateFilter flag")
	}
	var driverConfigBase *eUtils.DriverConfig
	var appRoleConfigPtr *string

	if driverConfig != nil {
		driverConfigBase = driverConfig
		if len(driverConfigBase.EndDir) == 0 && len(*endDirPtr) != 0 {
			// Bad inputs... use default.
			driverConfigBase.EndDir = *endDirPtr
		}
		appRoleConfigPtr = &driverConfig.AppRoleConfig

	} else {
		// If logging production directory does not exist and is selected log to local directory
		if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/"+coreopts.BuildOptions.GetFolderPrefix(nil)+"sub.log" {
			*logFilePtr = "./" + coreopts.BuildOptions.GetFolderPrefix(nil) + "sub.log"
		}
		f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			fmt.Println("Log init failure")
			return err
		}

		logger := log.New(f, "[INIT]", log.LstdFlags)
		driverConfigBase = &eUtils.DriverConfig{
			CoreConfig: core.CoreConfig{
				ExitOnFailure: exitOnFailure,
				Log:           logger,
			},
			Insecure: *insecurePtr,
			EndDir:   *endDirPtr,
		}
		appRoleConfigPtr = new(string)
	}

	if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
		var err error
		*envPtr, err = eUtils.LoginToLocal()
		fmt.Println(*envPtr)
		eUtils.CheckError(&driverConfigBase.CoreConfig, err, false)
		return err
	}

	fmt.Printf("Connecting to vault @ %s\n", *addrPtr)

	autoErr := eUtils.AutoAuth(driverConfigBase, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, envCtxPtr, *appRoleConfigPtr, *pingPtr)
	if autoErr != nil {
		fmt.Println("Missing auth components.")
		return autoErr
	}
	if memonly.IsMemonly() {
		memprotectopts.MemUnprotectAll(nil)
		memprotectopts.MemProtect(nil, tokenPtr)
	}

	mod, err := helperkv.NewModifier(*insecurePtr, *tokenPtr, *addrPtr, *envPtr, nil, true, driverConfigBase.CoreConfig.Log)
	if mod != nil {
		defer mod.Release()
	}
	if err != nil {
		fmt.Println("Failure to init to vault")
		driverConfigBase.CoreConfig.Log.Println("Failure to init to vault")
		return err
	}
	mod.Env = *envPtr

	if *templatePathsPtr != "" {
		fmt.Printf("Downloading templates from vault to %s\n", driverConfigBase.EndDir)
		// The actual download templates goes here.
		il.DownloadTemplates(&driverConfigBase.CoreConfig, mod, driverConfigBase.EndDir, driverConfigBase.CoreConfig.Log, templatePathsPtr)
	} else if *projectInfoPtr {
		templateList, err := mod.List("templates/", driverConfigBase.CoreConfig.Log)
		if err != nil {
			fmt.Println("Failure read templates")
			driverConfigBase.CoreConfig.Log.Println("Failure read templates")
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
		fmt.Printf("Downloading templates from vault to %s\n", driverConfigBase.EndDir)
		// The actual download templates goes here.
		warn, err := il.DownloadTemplateDirectory(&driverConfigBase.CoreConfig, mod, driverConfigBase.EndDir, driverConfigBase.CoreConfig.Log, filterTemplatePtr)
		if err != nil {
			fmt.Println(err)
			driverConfigBase.CoreConfig.Log.Printf("Failure to download: %s", err.Error())
			if strings.Contains(err.Error(), "x509: certificate") {
				return err
			}
		}
		eUtils.CheckError(&driverConfigBase.CoreConfig, err, false)
		eUtils.CheckWarnings(&driverConfigBase.CoreConfig, warn, false)
	}
	return nil
}

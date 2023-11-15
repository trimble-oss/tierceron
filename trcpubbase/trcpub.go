package trcpubbase

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

func CommonMain(envPtr *string,
	addrPtr *string,
	tokenPtr *string,
	envCtxPtr *string,
	secretIDPtr *string,
	appRoleIDPtr *string,
	tokenNamePtr *string,
	flagset *flag.FlagSet,
	argLines []string,
	c *eUtils.DriverConfig) {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	if flagset == nil {
		flagset = flag.NewFlagSet(argLines[0], flag.ExitOnError)
		flagset.Usage = func() {
			fmt.Fprintf(flagset.Output(), "Usage of %s:\n", argLines[0])
			flagset.PrintDefaults()
		}
		flagset.String("env", "dev", "Environment to configure")
		flagset.String("addr", "", "API endpoint for the vault")
		flagset.String("token", "", "Vault access token")
		flagset.String("secretID", "", "Public app role ID")
		flagset.String("appRoleID", "", "Secret app role ID")
		flagset.String("tokenName", "", "Token name used by this "+coreopts.GetFolderPrefix(nil)+"pub to access the vault")
	}
	dirPtr := flagset.String("dir", coreopts.GetFolderPrefix(nil)+"_templates", "Directory containing template files for vault")
	pingPtr := flagset.Bool("ping", false, "Ping vault.")
	insecurePtr := flagset.Bool("insecure", false, "By default, every ssl connection is secure.  Allows to continue with server connections considered insecure.")
	logFilePtr := flagset.String("log", "./"+coreopts.GetFolderPrefix(nil)+"pub.log", "Output path for log files")
	appRolePtr := flagset.String("approle", "configpub.yml", "Name of auth config file - example.yml (optional)")

	if c == nil || !c.IsShellSubProcess {
		flagset.Parse(argLines[1:])
	} else {
		flagset.Parse(nil)
	}

	var configBase *eUtils.DriverConfig
	if c != nil {
		configBase = c
		*insecurePtr = configBase.Insecure
		*appRolePtr = configBase.AppRoleConfig
	} else {
		// If logging production directory does not exist and is selected log to local directory
		if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/"+coreopts.GetFolderPrefix(nil)+"pub.log" {
			*logFilePtr = "./" + coreopts.GetFolderPrefix(nil) + "pub.log"
		}
		f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		logger := log.New(f, "[INIT]", log.LstdFlags)
		configBase = &eUtils.DriverConfig{Insecure: true, Log: logger, ExitOnFailure: true}
		eUtils.CheckError(configBase, err, true)
	}

	autoErr := eUtils.AutoAuth(configBase, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, envCtxPtr, *appRolePtr, *pingPtr)
	eUtils.CheckError(configBase, autoErr, true)

	if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
		var err error
		*envPtr, err = eUtils.LoginToLocal()
		fmt.Println(*envPtr)
		eUtils.CheckError(configBase, err, true)
	}

	if c != nil && c.IsShell {
		c.Log.Printf("Connecting to vault @ %s\n", *addrPtr)
		c.Log.Printf("Uploading templates in %s to vault\n", *dirPtr)
	} else {
		fmt.Printf("Connecting to vault @ %s\n", *addrPtr)
		fmt.Printf("Uploading templates in %s to vault\n", *dirPtr)
	}

	mod, err := helperkv.NewModifier(*insecurePtr, *tokenPtr, *addrPtr, *envPtr, nil, true, configBase.Log)
	if mod != nil {
		defer mod.Release()
	}
	eUtils.CheckError(configBase, err, true)
	mod.Env = *envPtr

	warn, err := il.UploadTemplateDirectory(configBase, mod, *dirPtr)
	if err != nil {
		if strings.Contains(err.Error(), "x509: certificate") {
			os.Exit(-1)
		}
	}

	eUtils.CheckError(configBase, err, true)
	eUtils.CheckWarnings(configBase, warn, true)
}

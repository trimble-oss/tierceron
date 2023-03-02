package trcpubbase

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	il "github.com/trimble-oss/tierceron/trcinit/initlib"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
	eUtils "github.com/trimble-oss/tierceron/utils"
	"github.com/trimble-oss/tierceron/utils/mlock"
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
	c *eUtils.DriverConfig) {
	if memonly.IsMemonly() {
		mlock.Mlock(nil)
	}
	dirPtr := flag.String("dir", coreopts.GetFolderPrefix()+"_templates", "Directory containing template files for vault")
	pingPtr := flag.Bool("ping", false, "Ping vault.")
	insecurePtr := flag.Bool("insecure", false, "By default, every ssl connection is secure.  Allows to continue with server connections considered insecure.")
	logFilePtr := flag.String("log", "./"+coreopts.GetFolderPrefix()+"pub.log", "Output path for log files")
	appRolePtr := flag.String("", "config.yml", "Name of auth config file - example.yml")

	if c == nil || !c.IsShell {
		flag.Parse()
	} else {
		flag.CommandLine.Parse(nil)
	}

	var configBase *eUtils.DriverConfig
	if c != nil {
		configBase = c
		*insecurePtr = configBase.Insecure
		*appRolePtr = configBase.AppRoleConfig
	} else {
		// If logging production directory does not exist and is selected log to local directory
		if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/"+coreopts.GetFolderPrefix()+"pub.log" {
			*logFilePtr = "./" + coreopts.GetFolderPrefix() + "pub.log"
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

	fmt.Printf("Connecting to vault @ %s\n", *addrPtr)
	fmt.Printf("Uploading templates in %s to vault\n", *dirPtr)

	mod, err := helperkv.NewModifier(*insecurePtr, *tokenPtr, *addrPtr, *envPtr, nil, true, configBase.Log)
	if mod != nil {
		defer mod.Release()
	}
	eUtils.CheckError(configBase, err, true)
	mod.Env = *envPtr

	err, warn := il.UploadTemplateDirectory(mod, *dirPtr, configBase.Log)
	if err != nil {
		if strings.Contains(err.Error(), "x509: certificate") {
			os.Exit(-1)
		}
	}

	eUtils.CheckError(configBase, err, true)
	eUtils.CheckWarnings(configBase, warn, true)
}

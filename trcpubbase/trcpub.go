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
	sys "github.com/trimble-oss/tierceron/vaulthelper/system"
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
	configFilePtr := flag.String("", "config.yml", "Name of auth config file - example.yml")

	flag.Parse()

	// If logging production directory does not exist and is selected log to local directory
	if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/"+coreopts.GetFolderPrefix()+"pub.log" {
		*logFilePtr = "./" + coreopts.GetFolderPrefix() + "pub.log"
	}
	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)

	logger := log.New(f, "[INIT]", log.LstdFlags)

	var config *eUtils.DriverConfig
	if c != nil {
		config = c
		*insecurePtr = config.Insecure
		*configFilePtr = config.FileFilter[0]
	} else {
		config = &eUtils.DriverConfig{Insecure: true, Log: logger, ExitOnFailure: true}
	}

	eUtils.CheckError(config, err, true)

	if len(*tokenNamePtr) > 0 {
		if len(*appRoleIDPtr) == 0 || len(*secretIDPtr) == 0 {
			eUtils.CheckError(config, fmt.Errorf("Need both public and secret app role to retrieve token from vault"), true)
		}
		v, err := sys.NewVault(*insecurePtr, *addrPtr, *envPtr, false, *pingPtr, false, logger)
		if v != nil {
			defer v.Close()
		}
		eUtils.CheckError(config, err, true)

		master, err := v.AppRoleLogin(*appRoleIDPtr, *secretIDPtr)
		eUtils.CheckError(config, err, true)

		mod, err := helperkv.NewModifier(*insecurePtr, master, *addrPtr, *envPtr, nil, true, logger)
		if mod != nil {
			defer mod.Release()
		}
		eUtils.CheckError(config, err, true)
		mod.RawEnv = "bamboo"
		mod.Env = "bamboo"

		*tokenPtr, err = mod.ReadValue("super-secrets/tokens", *tokenNamePtr)
		eUtils.CheckError(config, err, true)
	}
	if memonly.IsMemonly() {
		mlock.MunlockAll(nil)
		mlock.Mlock2(nil, tokenPtr)
	}

	autoErr := eUtils.AutoAuth(config, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, envCtxPtr, *configFilePtr, *pingPtr)
	eUtils.CheckError(config, autoErr, true)

	if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
		var err error
		*envPtr, err = eUtils.LoginToLocal()
		fmt.Println(*envPtr)
		eUtils.CheckError(config, err, true)
	}

	fmt.Printf("Connecting to vault @ %s\n", *addrPtr)
	fmt.Printf("Uploading templates in %s to vault\n", *dirPtr)

	mod, err := helperkv.NewModifier(*insecurePtr, *tokenPtr, *addrPtr, *envPtr, nil, true, logger)
	if mod != nil {
		defer mod.Release()
	}
	eUtils.CheckError(config, err, true)
	mod.Env = *envPtr

	err, warn := il.UploadTemplateDirectory(mod, *dirPtr, logger)
	if err != nil {
		if strings.Contains(err.Error(), "x509: certificate") {
			os.Exit(-1)
		}
	}

	eUtils.CheckError(config, err, true)
	eUtils.CheckWarnings(config, warn, true)
}

package trcpubbase

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/pkg/core/cache"
	il "github.com/trimble-oss/tierceron/pkg/trcinit/initlib"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

func PrintVersion() {
	fmt.Println("Version: " + "1.27")
}

// Reads in template files in specified directory
// Template directory should contain directories for each service
// Templates are uploaded to templates/<service>/<file name>/template-file
// The file is saved under the data key, and the extension under the ext key
// Vault automatically encodes the file into base64

func CommonMain(envPtr *string,
	addrPtr *string,
	envCtxPtr *string,
	secretIDPtr *string,
	appRoleIDPtr *string,
	tokenNamePtr *string,
	flagset *flag.FlagSet,
	argLines []string,
	driverConfig *config.DriverConfig) {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	var tokenPtr *string = nil
	if flagset == nil {
		PrintVersion()
		flagset = flag.NewFlagSet(argLines[0], flag.ExitOnError)
		flagset.Usage = func() {
			fmt.Fprintf(flagset.Output(), "Usage of %s:\n", argLines[0])
			flagset.PrintDefaults()
		}
		flagset.String("env", "dev", "Environment to configure")
		flagset.String("addr", "", "API endpoint for the vault")
		flagset.String("token", "", "Vault access token")
		flagset.String("secretID", "", "Secret for app role ID")
		flagset.String("appRoleID", "", "Public app role ID")
		flagset.String("tokenName", "", "Token name used by this "+coreopts.BuildOptions.GetFolderPrefix(nil)+"pub to access the vault")
	} else {
		tokenPtr = flagset.String("token", "", "Vault access token")
	}
	dirPtr := flagset.String("dir", coreopts.BuildOptions.GetFolderPrefix(nil)+"_templates", "Directory containing template files for vault")
	pingPtr := flagset.Bool("ping", false, "Ping vault.")
	insecurePtr := flagset.Bool("insecure", false, "By default, every ssl connection this tool makes is verified secure.  This option allows to tool to continue with server connections considered insecure.")
	logFilePtr := flagset.String("log", "./"+coreopts.BuildOptions.GetFolderPrefix(nil)+"pub.log", "Output path for log files")
	appRoleConfigPtr := flagset.String("approle", "configpub.yml", "Name of auth config file - example.yml (optional)")
	filterTemplatePtr := flagset.String("templateFilter", "", "Specifies which templates to filter")

	if driverConfig == nil || !driverConfig.IsShellSubProcess {
		flagset.Parse(argLines[1:])
	} else {
		flagset.Parse(nil)
	}

	var driverConfigBase *config.DriverConfig
	if driverConfig.CoreConfig.IsShell {
		driverConfigBase = driverConfig
		*insecurePtr = driverConfigBase.CoreConfig.Insecure
		if eUtils.RefLength(driverConfigBase.CoreConfig.AppRoleConfigPtr) > 0 {
			appRoleConfigPtr = driverConfigBase.CoreConfig.AppRoleConfigPtr
		}
	} else {
		// If logging production directory does not exist and is selected log to local directory
		if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/"+coreopts.BuildOptions.GetFolderPrefix(nil)+"pub.log" {
			*logFilePtr = "./" + coreopts.BuildOptions.GetFolderPrefix(nil) + "pub.log"
		}
		f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		logger := log.New(f, "[INIT]", log.LstdFlags)
		driverConfigBase = driverConfig
		driverConfigBase.CoreConfig.Insecure = false
		driverConfigBase.CoreConfig.Log = logger
		if eUtils.RefLength(tokenNamePtr) == 0 && eUtils.RefLength(tokenPtr) > 0 {
			tokenName := fmt.Sprintf("vault_pub_token_%s", *envPtr)
			tokenNamePtr = &tokenName
		}
		driverConfigBase.CoreConfig.TokenCache = cache.NewTokenCache(*tokenNamePtr, tokenPtr)
		driverConfig.CoreConfig.TokenCache.CurrentTokenNamePtr = tokenNamePtr

		if eUtils.RefLength(driverConfigBase.CoreConfig.AppRoleConfigPtr) > 0 {
			appRoleConfigPtr = driverConfigBase.CoreConfig.AppRoleConfigPtr
		} else {
			appRole := "configpub.yml"
			appRoleConfigPtr = &appRole
		}

		eUtils.CheckError(driverConfigBase.CoreConfig, err, true)
	}

	autoErr := eUtils.AutoAuth(driverConfigBase, secretIDPtr, appRoleIDPtr, tokenNamePtr, tokenPtr, envPtr, addrPtr, envCtxPtr, appRoleConfigPtr, *pingPtr)
	eUtils.CheckError(driverConfigBase.CoreConfig, autoErr, true)

	if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
		var err error
		*envPtr, err = eUtils.LoginToLocal()
		fmt.Println(*envPtr)
		eUtils.CheckError(driverConfigBase.CoreConfig, err, true)
	}

	if driverConfig != nil && driverConfig.CoreConfig.IsShell {
		driverConfig.CoreConfig.Log.Printf("Connecting to vault @ %s\n", *addrPtr)
		driverConfig.CoreConfig.Log.Printf("Uploading templates in %s to vault\n", *dirPtr)
	} else {
		fmt.Printf("Connecting to vault @ %s\n", *addrPtr)
		fmt.Printf("Uploading templates in %s to vault\n", *dirPtr)
	}

	mod, err := helperkv.NewModifier(*insecurePtr, driverConfigBase.CoreConfig.TokenCache.GetToken(fmt.Sprintf("vault_pub_token_%s", *envPtr)), addrPtr, *envPtr, nil, true, driverConfigBase.CoreConfig.Log)
	if mod != nil {
		defer mod.Release()
	}
	eUtils.CheckError(driverConfigBase.CoreConfig, err, true)
	mod.Env = *envPtr

	warn, err := il.UploadTemplateDirectory(driverConfigBase.CoreConfig, mod, *dirPtr, filterTemplatePtr)
	if err != nil {
		if strings.Contains(err.Error(), "x509: certificate") {
			os.Exit(-1)
		}
	}

	eUtils.CheckError(driverConfigBase.CoreConfig, err, true)
	eUtils.CheckWarnings(driverConfigBase.CoreConfig, warn, true)
}

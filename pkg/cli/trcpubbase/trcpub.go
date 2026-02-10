package trcpubbase

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/memonly"
	"github.com/trimble-oss/tierceron-core/v2/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig"
	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig/cache"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/kernelopts"
	il "github.com/trimble-oss/tierceron/pkg/trcinit/initlib"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

func PrintVersion() {
	fmt.Fprintln(os.Stderr, "Version: "+"1.29")
}

// Reads in template files in specified directory
// Template directory should contain directories for each service
// Templates are uploaded to templates/<service>/<file name>/template-file
// The file is saved under the data key, and the extension under the ext key
// Vault automatically encodes the file into base64

func CommonMain(envPtr *string,
	envCtxPtr *string,
	tokenNamePtr *string,
	flagset *flag.FlagSet,
	argLines []string,
	driverConfig *config.DriverConfig,
) {
	if driverConfig == nil || driverConfig.CoreConfig == nil || driverConfig.CoreConfig.TokenCache == nil {
		driverConfig = &config.DriverConfig{
			CoreConfig: &coreconfig.CoreConfig{
				ExitOnFailure: true,
				TokenCache:    cache.NewTokenCacheEmpty(),
			},
		}
	}

	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	var tokenPtr *string = nil
	var addrPtr *string = nil
	var helpPtr *bool = nil
	if flagset == nil {
		PrintVersion()
		progName := "trcpub"
		if len(argLines) > 0 {
			progName = argLines[0]
		}
		// Use ContinueOnError in shell/kernelz mode to avoid exiting, otherwise ExitOnError
		errorHandling := flag.ExitOnError
		if driverConfig.IsShellCommand || kernelopts.BuildOptions.IsKernelZ() {
			errorHandling = flag.ContinueOnError
		}
		flagset = flag.NewFlagSet(progName, errorHandling)
		flagset.Usage = func() {
			fmt.Fprintf(flagset.Output(), "Usage:\n")
			flagset.PrintDefaults()
		}
		helpPtr = flagset.Bool("h", false, "Display help")
		flagset.String("env", "dev", "Environment to configure")
		flagset.String("addr", "", "API endpoint for the vault")
		flagset.String("token", "", "Vault access token")
		flagset.String("secretID", "", "Secret for app role ID")
		flagset.String("appRoleID", "", "Public app role ID")
		flagset.String("tokenName", "", "Token name used by this "+coreopts.BuildOptions.GetFolderPrefix(nil)+"pub to access the vault")
	} else {
		tokenPtr = flagset.String("token", "", "Vault access token")
		addrPtr = flagset.String("addr", "", "API endpoint for the vault")
	}
	dirPtr := flagset.String("dir", coreopts.BuildOptions.GetFolderPrefix(nil)+"_templates", "Directory containing template files for vault")
	pingPtr := flagset.Bool("ping", false, "Ping vault.")
	insecurePtr := flagset.Bool("insecure", false, "By default, every ssl connection this tool makes is verified secure.  This option allows to tool to continue with server connections considered insecure.")
	logFilePtr := flagset.String("log", "./"+coreopts.BuildOptions.GetFolderPrefix(nil)+"pub.log", "Output path for log files")
	roleEntityPtr := flagset.String("approle", "configpub.yml", "Name of auth config file - example.yml (optional)")
	filterTemplatePtr := flagset.String("templateFilter", "", "Specifies which templates to filter")

	// If running from trcshcmd (IsShellCommand), redirect output to io/STDIO in memfs
	var outWriter io.Writer = os.Stderr
	if driverConfig.IsShellCommand && driverConfig.MemFs != nil {
		var stdioFile io.ReadWriteCloser
		var err error
		// Check if io directory exists
		if _, statErr := driverConfig.MemFs.Stat("io"); statErr == nil {
			// Directory exists, open file and seek to end for append
			stdioFile, err = driverConfig.MemFs.Open("io/STDIO")
			if err == nil {
				if seeker, ok := stdioFile.(io.Seeker); ok {
					seeker.Seek(0, io.SeekEnd)
				}
			}
		} else {
			// Directory doesn't exist, use WriteToMemFile to create it
			emptyData := []byte{}
			driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &emptyData, "io/STDIO")
			stdioFile, err = driverConfig.MemFs.Open("io/STDIO")
		}
		if err == nil {
			outWriter = stdioFile
			defer stdioFile.Close()
			// Redirect flagset output to the same writer for help messages
			flagset.SetOutput(outWriter)
		}
	}

	var parseErr error
	if driverConfig == nil || !driverConfig.IsShellSubProcess || (driverConfig.IsShellCommand || kernelopts.BuildOptions.IsKernelZ()) {
		parseErr = flagset.Parse(argLines[1:])
	} else {
		parseErr = flagset.Parse(nil)
	}

	// If help flag was used, return early (help output already written)
	if parseErr == flag.ErrHelp {
		return
	}

	// Check if -h flag was explicitly set
	if helpPtr != nil && *helpPtr {
		flagset.Usage()
		return
	}

	if eUtils.RefLength(addrPtr) > 0 {
		driverConfig.CoreConfig.TokenCache.SetVaultAddress(addrPtr)
	} else {
		if eUtils.RefLength(driverConfig.CoreConfig.TokenCache.VaultAddressPtr) == 0 {
			fmt.Fprintln(os.Stderr, "Please set the addr flag")
			if !driverConfig.IsShellCommand && !kernelopts.BuildOptions.IsKernelZ() {
				eUtils.LogSyncAndExit(driverConfig.CoreConfig.Log, "Please set the addr flag", 1)
			}
			return
		}
	}
	if envPtr == nil {
		env := "dev"
		envPtr = &env
	}
	envBasis := eUtils.GetEnvBasis(*envPtr)

	var driverConfigBase *config.DriverConfig
	if driverConfig.CoreConfig.IsShell {
		driverConfigBase = driverConfig
		*insecurePtr = driverConfigBase.CoreConfig.Insecure

		if eUtils.RefLength(driverConfigBase.CoreConfig.CurrentRoleEntityPtr) > 0 {
			roleEntityPtr = driverConfigBase.CoreConfig.CurrentRoleEntityPtr
		}
	} else {
		// If logging production directory does not exist and is selected log to local directory
		if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/"+coreopts.BuildOptions.GetFolderPrefix(nil)+"pub.log" {
			*logFilePtr = "./" + coreopts.BuildOptions.GetFolderPrefix(nil) + "pub.log"
		}
		f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
		logger := log.New(f, "[INIT]", log.LstdFlags)
		driverConfigBase = driverConfig
		driverConfigBase.CoreConfig.Insecure = false
		driverConfigBase.CoreConfig.Log = logger
		if eUtils.RefLength(tokenNamePtr) == 0 && eUtils.RefLength(tokenPtr) > 0 {
			tokenName := fmt.Sprintf("vault_pub_token_%s", envBasis)
			tokenNamePtr = &tokenName
		}
		driverConfigBase.CoreConfig.TokenCache.AddToken(*tokenNamePtr, tokenPtr)
		driverConfig.CoreConfig.CurrentTokenNamePtr = tokenNamePtr

		if eUtils.RefLength(driverConfigBase.CoreConfig.CurrentRoleEntityPtr) > 0 {
			roleEntityPtr = driverConfigBase.CoreConfig.CurrentRoleEntityPtr
		} else {
			appRole := "configpub.yml"
			roleEntityPtr = &appRole
		}

		eUtils.CheckError(driverConfigBase.CoreConfig, err, true)
	}

	autoErr := eUtils.AutoAuth(driverConfigBase, tokenNamePtr, &tokenPtr, &envBasis, envCtxPtr, roleEntityPtr, *pingPtr)
	eUtils.CheckError(driverConfigBase.CoreConfig, autoErr, true)

	if envPtr != nil && len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
		var err error
		*envPtr, err = eUtils.LoginToLocal()
		fmt.Fprintln(outWriter, *envPtr)
		eUtils.CheckError(driverConfigBase.CoreConfig, err, true)
	}

	if driverConfig != nil && driverConfig.CoreConfig.IsShell {
		driverConfig.CoreConfig.Log.Printf("Connecting to vault @ %s\n", *driverConfigBase.CoreConfig.TokenCache.VaultAddressPtr)
		driverConfig.CoreConfig.Log.Printf("Uploading templates in %s to vault\n", *dirPtr)
	} else {
		fmt.Fprintf(outWriter, "Connecting to vault @ %s\n", *driverConfigBase.CoreConfig.TokenCache.VaultAddressPtr)
		fmt.Fprintf(outWriter, "Uploading templates in %s to vault\n", *dirPtr)
	}

	mod, err := helperkv.NewModifierFromCoreConfig(driverConfigBase.CoreConfig,
		fmt.Sprintf("vault_pub_token_%s", envBasis),
		envBasis,
		true)
	if mod != nil {
		defer mod.Release()
	}
	eUtils.CheckError(driverConfigBase.CoreConfig, err, true)
	mod.Env = envBasis

	warn, err := il.UploadTemplateDirectory(nil, driverConfigBase.CoreConfig, mod, *dirPtr, filterTemplatePtr)
	if err != nil {
		if strings.Contains(err.Error(), "x509: certificate") {
			fmt.Fprintf(outWriter, "Template upload failure %s\n", err.Error())
			if !driverConfig.IsShellCommand && !kernelopts.BuildOptions.IsKernelZ() {
				eUtils.LogSyncAndExit(driverConfig.CoreConfig.Log, fmt.Sprintf("Template upload failure %s", err.Error()), -1)
			}
			return
		}
	}

	eUtils.CheckError(driverConfigBase.CoreConfig, err, true)
	eUtils.CheckWarnings(driverConfigBase.CoreConfig, warn, true)
}

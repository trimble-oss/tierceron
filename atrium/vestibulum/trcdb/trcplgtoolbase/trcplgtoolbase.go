package trcplgtoolbase

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron-core/v2/buildopts/plugincoreopts"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/pluginutil/certify"
	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/kernelopts"
	"github.com/trimble-oss/tierceron/pkg/capauth"
	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"
	"github.com/trimble-oss/tierceron/pkg/core/util/docker"
	"github.com/trimble-oss/tierceron/pkg/core/util/hive"
	"github.com/trimble-oss/tierceron/pkg/core/util/repository"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"

	trcapimgmtbase "github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/trcapimgmtbase"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/trccertmgmtbase"
	trcgitmgmtbase "github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/trcgitmgmtbase"
)

func CommonMain(envPtr *string,
	envCtxPtr *string,
	tokenNamePtr *string,
	regionPtr *string,
	flagset *flag.FlagSet,
	argLines []string,
	trcshDriverConfig *capauth.TrcshDriverConfig,
	mainPluginHandler ...*hive.PluginHandler,
) error {
	var flagEnvPtr *string
	var tokenPtr *string
	var addrPtr *string
	var logFilePtr *string

	// Main functions are as follows:
	if flagset == nil {
		if trcshDriverConfig != nil && trcshDriverConfig.DriverConfig != nil {
			eUtils.LogInfo(trcshDriverConfig.DriverConfig.CoreConfig, "Version: "+"1.06")
		} else {
			fmt.Fprintln(os.Stderr, "Version: 1.06")
		}
		flagset = flag.NewFlagSet(argLines[0], flag.ContinueOnError)
		// set and ignore..
		flagEnvPtr = flagset.String("env", "dev", "Environment to configure")
		flagset.String("addr", "", "API endpoint for the vault")
		flagset.String("token", "", "Vault access token")
		flagset.String("region", "", "Region to be processed") // If this is blank -> use context otherwise override context.
		flagset.String("log", "./"+coreopts.BuildOptions.GetFolderPrefix(nil)+"plgtool.log", "Output path for log files")
		flagset.Usage = func() {
			fmt.Fprintf(flagset.Output(), "Usage of %s:\n", argLines[0])
			flagset.PrintDefaults()
		}
	} else {
		tokenPtr = flagset.String("token", "", "Vault access token")
		addrPtr = flagset.String("addr", "", "API endpoint for the vault")
		if flagset.Lookup("log") == nil {
			logFilePtr = flagset.String("log", "./"+coreopts.BuildOptions.GetFolderPrefix(nil)+"config.log", "Output path for log file")
		}
	}
	defineServicePtr := flagset.Bool("defineService", false, "Specified when defining a service.")
	certifyImagePtr := flagset.Bool("certify", false, "Used to certifies vault plugin.")
	certifyInfoImagePtr := flagset.Bool("certifyInfo", false, "Used to certifies vault plugin.")
	// These functions only valid for pluginType trcshservice
	pluginservicestartPtr := flagset.Bool("pluginservicestart", false, "To start a trcshell kernel service for a particular plugin.")
	pluginservicestopPtr := flagset.Bool("pluginservicestop", false, "To stop a trcshell kernel service for a particular plugin.")
	winservicestopPtr := flagset.Bool("winservicestop", false, "To stop a windows service for a particular plugin.")
	winservicestartPtr := flagset.Bool("winservicestart", false, "To start a windows service for a particular plugin.")
	codebundledeployPtr := flagset.Bool("codebundledeploy", false, "To deploy a code bundle.")
	agentdeployPtr := flagset.Bool("agentdeploy", false, "To initiate deployment on agent.")
	projectservicePtr := flagset.String("projectservice", "", "Provide template path in form project/service")
	buildImagePtr := flagset.String("buildImage", "", "Path to Dockerfile to build")
	pushImagePtr := flagset.Bool("pushImage", false, "Push an image to the registry.")
	outputDestinationPtr := flagset.String("o", "", "Command output destination")
	pushAliasPtr := flagset.String("pushAlias", "", "Image name:tag to push to registry, separated by commas (eg: egg:plant,egg:salad,egg:bar).")

	// Common flags...
	startDirPtr := flagset.String("startDir", coreopts.BuildOptions.GetFolderPrefix(nil)+"_templates", "Template directory")
	insecurePtr := flagset.Bool("insecure", false, "By default, every ssl connection this tool makes is verified secure.  This option allows to tool to continue with server connections considered insecure.")

	// defineService flags...
	deployrootPtr := flagset.String("deployroot", "", "Optional path for deploying services to.")
	deploysubpathPtr := flagset.String("deploysubpath", "", "Subpath under root to deliver code bundles.")
	serviceNamePtr := flagset.String("serviceName", "", "Optional name of service to use in managing service.")
	pathParamPtr := flagset.String("pathParam", "", "Optional path placeholder replacement to use in managing service.")
	codeBundlePtr := flagset.String("codeBundle", "", "Code bundle to deploy.")
	expandTargetPtr := flagset.Bool("expandTarget", false, "Used to unzip files at deploy path")
	trcbootstrapPtr := flagset.String("trcbootstrap", "/deploy/deploy.trc", "Used to unzip files at deploy path")
	instancesPtr := flagset.String("instances", "", "Used to specify pod instances for deployment")

	// Common plugin flags...
	pluginNamePtr := flagset.String("pluginName", "", "Used to certify vault plugin")
	pluginNameAliasPtr := flagset.String("pluginNameAlias", "", "Name used to define an alias for a plugin")
	pluginTypePtr := flagset.String("pluginType", "vault", "Used to indicate type of plugin.  Default is vault.")

	// Certify flags...
	sha256Ptr := flagset.String("sha256", "", "Used to certify vault plugin") // This has to match the image that is pulled -> then we write the vault.
	checkDeployedPtr := flagset.Bool("checkDeployed", false, "Used to check if plugin has been copied, deployed, & certified")
	checkCopiedPtr := flagset.Bool("checkCopied", false, "Used to check if plugin has been copied & certified")

	// NewRelic flags...
	newrelicAppNamePtr := flagset.String("newRelicAppName", "", "App name for New Relic")
	newrelicLicenseKeyPtr := flagset.String("newRelicLicenseKey", "", "License key for New Relic")

	certifyInit := false

	// APIM flags
	updateAPIMPtr := flagset.Bool("updateAPIM", false, "Used to update Azure APIM")

	// Repository commands
	// We need to keep the getCmd flag for compatibility with existing checks in the code
	excludePtr := flagset.String("exclude", "trc_templates", "Comma-delimited list of directories to exclude from download")

	// Cert flags
	certPathPtr := flagset.String("certPath", "", "Path to certificate to push to Azure")
	isGetCommand := false
	repoName := ""
	isRunnableKernelPlugin := false
	if !trcshDriverConfig.DriverConfig.CoreConfig.IsShell {
		isRunnableKernelPlugin = trcshDriverConfig.DriverConfig != nil &&
			trcshDriverConfig.DriverConfig.DeploymentConfig != nil &&
			(*trcshDriverConfig.DriverConfig.DeploymentConfig)["trctype"] != nil &&
			((*trcshDriverConfig.DriverConfig.DeploymentConfig)["trctype"] == "trcshpluginservice" ||
				(kernelopts.BuildOptions.IsKernel() && (*trcshDriverConfig.DriverConfig.DeploymentConfig)["trctype"] == "trcflowpluginservice"))

		args := argLines[1:]
		argOffset := 1
		// Check for commands before validating flags
		if len(args) > 1 && args[0] == "get" {
			// This is the "get <repo>" command pattern, don't validate first two args as flags
			for i := 2; i < len(args); i++ {
				s := args[i]
				if s[0] != '-' {
					fmt.Fprintln(os.Stderr, "Wrong flag syntax: ", s)
					return fmt.Errorf("wrong flag syntax: %s", s)
				}
			}
			// Look for "get" command and the repo URL immediately following it in argLines
			isGetCommand = true
			repoName = args[1]

			argOffset = 3
		} else {
			// Standard flag validation
			for i := 0; i < len(args); i++ {
				s := args[i]
				if s[0] != '-' {
					fmt.Fprintln(os.Stderr, "Wrong flag syntax: ", s)
					return fmt.Errorf("wrong flag syntax: %s", s)
				}
			}
		}
		err := flagset.Parse(argLines[argOffset:])
		if err != nil {
			return err
		}

		// Prints usage if no flags are specified
		if flagset.NFlag() == 0 {
			flagset.Usage()
			return errors.New("invalid input parameters")
		}

		if eUtils.RefLength(tokenNamePtr) == 0 {
			tokenNamePtr = new(string)
			*tokenNamePtr = fmt.Sprintf("config_token_plugin%s", *envPtr)
		}
		trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.AddToken(*tokenNamePtr, tokenPtr)
		trcshDriverConfig.DriverConfig.CoreConfig.CurrentTokenNamePtr = tokenNamePtr
	} else {
		argOffset := 1
		if len(argLines) > 1 && (argLines[0] == "get" || (len(argLines) > 2 && argLines[1] == "get")) {
			// Determine the correct get command position and repoName position
			getIndex := 0
			if argLines[0] != "get" {
				getIndex = 1
			}

			// This is the "get <repo>" command pattern, don't validate first two args as flags
			for i := getIndex + 2; i < len(argLines); i++ {
				s := argLines[i]
				if s[0] != '-' {
					fmt.Fprintln(os.Stderr, "Wrong flag syntax: ", s)
					return fmt.Errorf("wrong flag syntax: %s", s)
				}
			}

			// Look for "get" command and the repo URL immediately following it in argLines
			isGetCommand = true
			repoName = argLines[getIndex+1]

			argOffset = getIndex + 2
		}
		err := flagset.Parse(argLines[argOffset:])
		if err != nil {
			return err
		}
		trcshDriverConfig.DriverConfig.CoreConfig.CurrentTokenNamePtr = tokenNamePtr
	}

	if trcshDriverConfig.DriverConfig.CoreConfig.Log == nil && logFilePtr != nil {
		f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error creating log file: "+*logFilePtr)
			return errors.New("Error creating log file: " + *logFilePtr)
		}
		logger := log.New(f, "["+coreopts.BuildOptions.GetFolderPrefix(nil)+"config]", log.LstdFlags)
		trcshDriverConfig.DriverConfig.CoreConfig.Log = logger
	}

	if !isRunnableKernelPlugin {
		if eUtils.RefLength(addrPtr) == 0 {
			eUtils.ReadAuthParts(trcshDriverConfig.DriverConfig, false)
		} else {
			trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.SetVaultAddress(addrPtr)
		}
	}

	if trcshDriverConfig != nil && trcshDriverConfig.DriverConfig != nil && trcshDriverConfig.DriverConfig.DeploymentConfig != nil && (*trcshDriverConfig.DriverConfig.DeploymentConfig)["trcpluginalias"] != nil {
		// Prefer internal definition of alias
		*pluginNameAliasPtr = (*trcshDriverConfig.DriverConfig.DeploymentConfig)["trcpluginalias"].(string)
	}

	if (len(*newrelicAppNamePtr) == 0 && len(*newrelicLicenseKeyPtr) != 0) || (len(*newrelicAppNamePtr) != 0 && len(*newrelicLicenseKeyPtr) == 0) {
		fmt.Fprintln(os.Stderr, "Must use -newrelicAppName && -newrelicLicenseKey flags together to use -certify flag")
		return errors.New("must use -newrelicAppName && -newrelicLicenseKey flags together to use -certify flag")
	}
	if len(*pluginNamePtr) == 0 && len(trcshDriverConfig.PluginName) > 0 {
		*pluginNamePtr = trcshDriverConfig.PluginName
	}

	if *certifyImagePtr && (len(*pluginNamePtr) == 0 || len(*sha256Ptr) == 0) {
		fmt.Fprintln(os.Stderr, "Must use -pluginName && -sha256 flags to use -certify flag")
		return errors.New("must use -pluginName && -sha256 flags to use -certify flag")
	}
	if *certifyInfoImagePtr && (len(*pluginNamePtr) == 0) {
		fmt.Fprintln(os.Stderr, "Must use -pluginName flag to use -certifyInfo flag")
		return errors.New("must use -pluginName flag to use -certifyInfo flag")
	}

	if *checkDeployedPtr && (len(*pluginNamePtr) == 0) {
		fmt.Fprintln(os.Stderr, "Must use -pluginName flag to use -checkDeployed flag")
		return errors.New("must use -pluginName flag to use -checkDeployed flag")
	}

	if *defineServicePtr && (len(*pluginNamePtr) == 0) {
		fmt.Fprintln(os.Stderr, "Must use -pluginName flag to use -defineService flag")
		return errors.New("must use -pluginName flag to use -defineService flag")
	}

	if *defineServicePtr && !regexp.MustCompile(`^[a-z0-9._/-]+$`).MatchString(*pluginNamePtr) {
		fmt.Fprintln(os.Stderr, "pluginName can only include lowercase alphanumeric characters, periods, dashes, underscores, and forward slashes")
		return errors.New("pluginName can only include lowercase alphanumeric characters, periods, dashes, underscores, and forward slashes")
	}

	if *pushImagePtr && (len(*pluginNamePtr) == 0) {
		fmt.Fprintln(os.Stderr, "Must use -pluginName flag to use -pushimage flag")
		return errors.New("must use -pluginName flag to use -pushimage flag")
	}

	if len(*buildImagePtr) > 0 && len(*pluginNamePtr) == 0 {
		fmt.Fprintln(os.Stderr, "Must use -pluginName flag to use -buildImage flag")
		return errors.New("must use -pluginName flag to use -buildImage flag")
	}

	if len(*buildImagePtr) > 0 && len(strings.Split(*pluginNamePtr, ":")[0]) > 128 {
		fmt.Fprintln(os.Stderr, "Image tag cannot be longer than 128 characters")
		return errors.New("image tag cannot be longer than 128 characters")
	}

	if len(*pushAliasPtr) > 0 && !*pushImagePtr {
		fmt.Fprintln(os.Stderr, "Must use -pushImage flag to use -pushAlias flag")
		return errors.New("must use -pushImage flag to use -pushAlias flag")
	}

	if len(*certPathPtr) > 0 && !*updateAPIMPtr {
		fmt.Fprintln(os.Stderr, "Must use -updateAPIM flag to use -certPath flag")
		return errors.New("must use -updateAPIM flag to use -certPath flag")
	}

	if trcshDriverConfig != nil && len(trcshDriverConfig.DriverConfig.PathParam) > 0 {
		// Prefer internal definition of alias
		*pathParamPtr = trcshDriverConfig.DriverConfig.PathParam
	}

	if len(*pathParamPtr) > 0 {
		r, _ := regexp.Compile("^[a-zA-Z0-9_]*$")
		if !r.MatchString(*pathParamPtr) {
			fmt.Fprintln(os.Stderr, "-pathParam can only contain alphanumberic characters or underscores")
			return errors.New("-pathParam can only contain alphanumberic characters or underscores")
		}
	}

	if *agentdeployPtr || *winservicestopPtr || *winservicestartPtr || *codebundledeployPtr || *pluginservicestopPtr || *pluginservicestartPtr {
		*pluginTypePtr = "trcshservice"
	}
	if *certifyInfoImagePtr {
		*pluginTypePtr = "trcshpluginservice"
	}

	if !*updateAPIMPtr && len(*buildImagePtr) == 0 && !*pushImagePtr && !isGetCommand {
		switch *pluginTypePtr {
		case "vault": // A vault plugin
			if trcshDriverConfig.DriverConfig.CoreConfig.IsShell {
				// TODO: do we want to support Deployment certifications in the pipeline at some point?
				// If so this is a config check to remove.
				fmt.Fprintf(os.Stderr, "Plugin type %s not supported in trcsh.\n", *pluginTypePtr)
				return fmt.Errorf("plugin type %s not supported in trcsh", *pluginTypePtr)
			}
			if *codebundledeployPtr {
				fmt.Fprintf(os.Stderr, "codebundledeploy not supported for plugin type %s in trcsh\n", *pluginTypePtr)
				return fmt.Errorf("codebundledeploy not supported for plugin type %s in trcsh", *pluginTypePtr)
			}

		case "agent": // A deployment agent tool.
			if trcshDriverConfig.DriverConfig.CoreConfig.IsShell {
				// TODO: do we want to support Deployment certifications in the pipeline at some point?
				// If so this is a config check to remove.
				fmt.Fprintf(os.Stderr, "Plugin type %s not supported in trcsh.\n", *pluginTypePtr)
				return fmt.Errorf("plugin type %s not supported in trcsh", *pluginTypePtr)
			}
			if *codebundledeployPtr {
				fmt.Fprintf(os.Stderr, "codebundledeploy not supported for plugin type %s in trcsh\n", *pluginTypePtr)
				return fmt.Errorf("codebundledeploy not supported for plugin type %s in trcsh", *pluginTypePtr)
			}
		case "trccmdtool": // A trc command line tool.
		case "trcshservice": // A trcshservice managed microservice
		case "trcshkubeservice":
		case "trcshpluginservice":
		case "trcshmutabilispraefecto":
		case "trcshcmdtoolplugin":
		case "trcflowpluginservice":
		default:
			if !*agentdeployPtr {
				fmt.Fprintln(os.Stderr, "Unsupported plugin type: "+*pluginTypePtr)
				return fmt.Errorf("unsupported plugin type: %s", *pluginTypePtr)
			} else {
				fmt.Fprintf(os.Stderr, "\nBeginning agent deployment for %s..\n", *pluginNamePtr)
			}
		}
	}

	// if *pluginTypePtr != "vault" {
	// 	*regionPtr = ""
	// }

	var currentRoleEntityPtr *string
	var trcshDriverConfigBase *capauth.TrcshDriverConfig
	if trcshDriverConfig != nil {
		trcshDriverConfigBase = trcshDriverConfig

		if !trcshDriverConfig.DriverConfig.CoreConfig.IsShell {
			if *agentdeployPtr {
				fmt.Fprintln(os.Stderr, "Unsupported agentdeploy outside trcsh")
				return errors.New("unsupported agentdeploy outside trcsh")
			}
			trcshDriverConfigBase.DriverConfig.CoreConfig.Insecure = *insecurePtr
			trcshDriverConfigBase.DriverConfig.StartDir = []string{*startDirPtr}
			trcshDriverConfigBase.DriverConfig.SubSectionValue = strings.Split(*pluginNamePtr, ":")[0]
		} else {
			if *pluginNameAliasPtr != "" {
				trcshDriverConfigBase.DriverConfig.SubSectionValue = *pluginNameAliasPtr
			} else {
				trcshDriverConfigBase.DriverConfig.SubSectionValue = strings.Split(*pluginNamePtr, ":")[0]
			}
			currentRoleEntityPtr = trcshDriverConfigBase.DriverConfig.CoreConfig.CurrentRoleEntityPtr
			*insecurePtr = trcshDriverConfigBase.DriverConfig.CoreConfig.Insecure
		}

	} else {
		fmt.Fprintln(os.Stderr, "Unsupported agentdeploy outside trcsh")
		return errors.New("unsupported agentdeploy outside trcsh")
	}

	if eUtils.RefLength(trcshDriverConfigBase.DriverConfig.CoreConfig.TokenCache.GetToken(*tokenNamePtr)) == 0 {
		autoErr := eUtils.AutoAuth(trcshDriverConfigBase.DriverConfig, tokenNamePtr, &tokenPtr, envPtr, envCtxPtr, currentRoleEntityPtr, false)
		if autoErr != nil {
			eUtils.LogErrorMessage(trcshDriverConfigBase.DriverConfig.CoreConfig, "Auth failure: "+autoErr.Error(), false)
			return errors.New("auth failure")
		}
	}
	trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Certify begin gathering certify configs\n")

	regions := []string{}

	pluginConfig := map[string]any{}
	pluginConfig = buildopts.BuildOptions.ProcessPluginEnvConfig(pluginConfig) // contains logNamespace for InitVaultMod
	if pluginConfig == nil {
		fmt.Fprintln(os.Stderr, "Error: Could not find plugin config")
		return errors.New("could not find plugin config")
	}
	pluginConfig["env"] = *envPtr
	pluginConfig["vaddress"] = *trcshDriverConfigBase.DriverConfig.CoreConfig.TokenCache.VaultAddressPtr
	if trcshDriverConfigBase.DriverConfig.CoreConfig.TokenCache.GetToken(*tokenNamePtr) != nil {
		pluginConfig["tokenptr"] = trcshDriverConfigBase.DriverConfig.CoreConfig.TokenCache.GetToken(*tokenNamePtr)
	}
	pluginConfig["ExitOnFailure"] = true
	if *regionPtr != "" {
		pluginConfig["regions"] = []string{*regionPtr}
	}
	driverConfig, mod, vault, err := eUtils.InitVaultModForTool(pluginConfig, trcshDriverConfigBase.DriverConfig)
	trcshConfig := &capauth.TrcshDriverConfig{
		DriverConfig: driverConfig,
	}
	trcshConfig.FeatherCtlCb = trcshDriverConfigBase.FeatherCtlCb
	trcshConfig.FeatherCtx = trcshDriverConfigBase.FeatherCtx
	if trcshConfig.FeatherCtx != nil && flagEnvPtr != nil && trcshConfig.FeatherCtx.Env != nil && strings.HasPrefix(*flagEnvPtr, *trcshConfig.FeatherCtx.Env) {
		// take on the environment context of the provided flag... like dev-1
		// Note: This updates the FeatherCtx.Env which is used later in FeatherCtlCb at line ~1250
		trcshConfig.FeatherCtx.Env = flagEnvPtr
	}

	if err != nil {
		trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Println("Error: " + err.Error() + " - 1")
		trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Println("Failed to init mod for deploy update")
		return err
	}
	driverConfig.StartDir = []string{*startDirPtr}
	if *pluginNameAliasPtr != "" {
		trcshDriverConfigBase.DriverConfig.SubSectionValue = *pluginNameAliasPtr
	} else if *pluginNamePtr != "" {
		trcshDriverConfigBase.DriverConfig.SubSectionValue = strings.Split(*pluginNamePtr, ":")[0]
	} else if deployPlugin, ok := (*trcshDriverConfigBase.DriverConfig.DeploymentConfig)["trcplugin"]; ok {
		if subsv, k := deployPlugin.(string); k {
			trcshDriverConfigBase.DriverConfig.SubSectionValue = subsv
			*pluginNamePtr = subsv
		}
	}

	var pluginHandler *hive.PluginHandler = nil
	var kernelPluginHandler *hive.PluginHandler = nil
	if *pluginNamePtr == "" {
		if deployPlugin, ok := (*trcshDriverConfigBase.DriverConfig.DeploymentConfig)["trcplugin"]; ok {
			if dep, k := deployPlugin.(string); k {
				*pluginNamePtr = dep
			} else {
				trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Println("Unexpected type for plugin name.")
			}
		} else {
			trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Println("Unable to set plugin name.")
		}
	}

	if isRunnableKernelPlugin {
		if len(mainPluginHandler) > 0 && mainPluginHandler[0] != nil && mainPluginHandler[0].Services != nil {
			kernelPluginHandler = mainPluginHandler[0]
			pluginHandler = kernelPluginHandler.GetPluginHandler(*pluginNamePtr, trcshDriverConfigBase.DriverConfig)
			pluginHandler.KernelId = kernelPluginHandler.KernelId
		}
	}

	mod.Env = *envPtr
	if trcshDriverConfigBase.DriverConfig.CoreConfig.Log != nil {
		trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Certify mod initialized\n")
	}

	if strings.HasPrefix(*envPtr, "staging") || strings.HasPrefix(*envPtr, "prod") || strings.HasPrefix(*envPtr, "dev") {
		supportedRegions := eUtils.GetSupportedProdRegions()
		if *regionPtr != "" {
			for _, supportedRegion := range supportedRegions {
				if *regionPtr == supportedRegion {
					regions = append(regions, *regionPtr)
					break
				}
			}
			if len(regions) == 0 {
				fmt.Fprintln(os.Stderr, "Unsupported region: "+*regionPtr)
				return fmt.Errorf("unsupported region: %s", *regionPtr)
			}
		}
		trcshDriverConfigBase.DriverConfig.CoreConfig.Regions = regions
	}

	// Handle both command-style "get <repo>" and flag-style "-get"
	if isGetCommand {
		// Validate required parameters
		if repoName == "" {
			fmt.Fprintln(os.Stderr, "Repository URL is required for get operation. Use 'trcplgtool get <repo>'")
			return errors.New("repository URL is required for get operation")
		}

		// Parse exclude directories
		excludeDirs := []string{}
		if len(*excludePtr) > 0 {
			excludeDirs = strings.Split(*excludePtr, ",")
		}

		// Execute the clone repository function
		err := trcgitmgmtbase.CloneRepository(repoName, "" /* targetDir */, envPtr, tokenNamePtr, driverConfig, mod, excludeDirs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Repository get operation failed: %s\n", err)
			return err
		}

		fmt.Fprintf(os.Stderr, "Successfully downloaded repository: %s\n", repoName)
		return nil
	}

	if *updateAPIMPtr {
		var apimError error
		if len(*certPathPtr) > 0 {
			apimError = trccertmgmtbase.CommonMain(certPathPtr, driverConfig, mod)
		} else {
			apimError = trcapimgmtbase.CommonMain(envPtr, nil, tokenNamePtr, regionPtr, startDirPtr, driverConfig, mod)
		}
		if apimError != nil {
			fmt.Fprintln(os.Stderr, apimError.Error())
			fmt.Fprintln(os.Stderr, "Couldn't update APIM...proceeding with build")
		}
		return nil
	}

	// Get existing configs if they exist...
	pluginToolConfig, plcErr := trcvutils.GetPluginToolConfig(trcshDriverConfigBase.DriverConfig, mod, coreopts.BuildOptions.InitPluginConfig(map[string]any{}), *defineServicePtr)
	if plcErr != nil {
		fmt.Fprintln(os.Stderr, plcErr.Error())
		return plcErr
	}
	if *certifyImagePtr {
		trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Certify begin activities\n")
	}
	if *certifyInfoImagePtr {
		trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Certify info begin activities\n")
	}

	if len(*sha256Ptr) > 0 {
		fileInfo, statErr := os.Stat(*sha256Ptr)
		if statErr == nil {
			if fileInfo.Mode().IsRegular() {
				file, fileOpenErr := os.Open(*sha256Ptr)
				if fileOpenErr != nil {
					fmt.Fprintln(os.Stderr, fileOpenErr.Error())
					return fileOpenErr
				}

				// Close the file when we're done.
				defer file.Close()

				// Create a reader to the file.
				reader := bufio.NewReader(file)

				pluginImage, imageErr := io.ReadAll(reader)
				if imageErr != nil {
					fmt.Fprintln(os.Stderr, "Failed to read image:"+imageErr.Error())
					return imageErr
				}
				sha256Bytes := sha256.Sum256(pluginImage)
				*sha256Ptr = fmt.Sprintf("%x", sha256Bytes)
			} else {
				fmt.Fprintln(os.Stderr, "Irregular image")
				return errors.New("irregular image")
			}
		} else {
			fmt.Fprintln(os.Stderr, "Failure to stat image:"+statErr.Error())
			return statErr
		}
	}

	if !*codebundledeployPtr && len(*sha256Ptr) > 0 {
		pluginToolConfig["trcsha256"] = *sha256Ptr
	}
	pluginToolConfig["pluginNamePtr"] = *pluginNamePtr
	pluginToolConfig["serviceNamePtr"] = *serviceNamePtr
	pluginToolConfig["instancesPtr"] = *instancesPtr
	pluginToolConfig["projectservicePtr"] = *projectservicePtr
	pluginToolConfig["deployrootPtr"] = *deployrootPtr
	pluginToolConfig["deploysubpathPtr"] = *deploysubpathPtr
	pluginToolConfig["codeBundlePtr"] = *codeBundlePtr
	pluginToolConfig["pathParamPtr"] = *pathParamPtr
	pluginToolConfig["expandTargetPtr"] = *expandTargetPtr // is a bool that gets converted to a string for writeout/certify
	pluginToolConfig["newrelicAppName"] = *newrelicAppNamePtr
	pluginToolConfig["newrelicLicenseKey"] = *newrelicLicenseKeyPtr
	pluginToolConfig["buildImagePtr"] = *buildImagePtr
	pluginToolConfig["pushAliasPtr"] = *pushAliasPtr
	pluginToolConfig["trcbootstrapPtr"] = *trcbootstrapPtr

	if _, ok := pluginToolConfig["trcplugin"].(string); !ok {
		if *defineServicePtr || *pushImagePtr {
			pluginToolConfig["trcplugin"] = pluginToolConfig["pluginNamePtr"].(string)
		}
		if _, ok := pluginToolConfig["serviceNamePtr"].(string); ok && len(pluginToolConfig["serviceNamePtr"].(string)) > 0 {
			pluginToolConfig["trcservicename"] = pluginToolConfig["serviceNamePtr"].(string)
		}
		if _, ok := pluginToolConfig["trcbootstrapPtr"].(string); ok && len(pluginToolConfig["trcbootstrapPtr"].(string)) > 0 {
			pluginToolConfig["trcbootstrap"] = pluginToolConfig["trcbootstrapPtr"].(string)
		}
		if *certifyImagePtr {
			certifyInit = true
		}

		if *defineServicePtr &&
			!*winservicestopPtr &&
			!*winservicestartPtr &&
			!*codebundledeployPtr &&
			!*certifyImagePtr {

			if trcshDriverConfigBase.DriverConfig.CoreConfig.IsShell {
				eUtils.LogSyncAndExit(trcshDriverConfig.DriverConfig.CoreConfig.Log, "Service definition not supported in trcsh.", -1)
			}
			if _, ok := pluginToolConfig["deployrootPtr"].(string); ok {
				pluginToolConfig["trcdeployroot"] = pluginToolConfig["deployrootPtr"].(string)
			}
			if _, ok := pluginToolConfig["deploysubpathPtr"]; ok {
				pluginToolConfig["trcdeploysubpath"] = pluginToolConfig["deploysubpathPtr"]
			}
			if _, ok := pluginToolConfig["codeBundlePtr"].(string); ok {
				pluginToolConfig["trccodebundle"] = pluginToolConfig["codeBundlePtr"].(string)
			}
			if _, ok := pluginToolConfig["projectServicePtr"].(string); ok {
				pluginToolConfig["trcprojectservice"] = pluginToolConfig["projectServicePtr"].(string)
			}
			if pathParam, ok := pluginToolConfig["pathParamPtr"].(string); ok && pathParam != "" {
				pluginToolConfig["trcpathparam"] = pluginToolConfig["pathParamPtr"].(string)
			}
			if expandTarget, ok := pluginToolConfig["expandTargetPtr"].(bool); ok && expandTarget { // only writes out if expandTarget = true
				pluginToolConfig["trcexpandtarget"] = "true"
			}
			if nrAppName, ok := pluginToolConfig["newrelicAppName"].(string); ok && nrAppName != "" {
				pluginToolConfig["newrelic_app_name"] = pluginToolConfig["newrelicAppName"].(string)
			}
			if nrLicenseKey, ok := pluginToolConfig["newrelicLicenseKey"].(string); ok && nrLicenseKey != "" {
				pluginToolConfig["newrelic_license_key"] = pluginToolConfig["newrelicLicenseKey"].(string)
			}
		}
	}

	if len(*buildImagePtr) > 0 || *pushImagePtr || *certifyImagePtr {
		if val, ok := pluginToolConfig["trcplugin"]; !ok || len(val.(string)) == 0 {
			err := errors.New("trcplugin not defined, cannot continue")
			fmt.Fprintln(os.Stderr, err)
			return err
		}
	}

	if len(*buildImagePtr) > 0 {
		fmt.Fprintln(os.Stderr, "Building image using local docker repository...")
		err := docker.BuildDockerImage(trcshDriverConfigBase.DriverConfig, *buildImagePtr, *pluginNamePtr)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return err
		} else {
			fmt.Fprintln(os.Stderr, "Image successfully built")
		}
	}

	if *pushImagePtr {
		if len(pluginToolConfig["pushAliasPtr"].(string)) > 0 {
			aliases := strings.Split(pluginToolConfig["pushAliasPtr"].(string), ",")
			for _, alias := range aliases {
				if strings.Split(alias, ":")[0] != strings.Split(pluginToolConfig["pluginNamePtr"].(string), ":")[0] {
					err := errors.New("pushAlias can only alias image tags, not image names")
					fmt.Fprintln(os.Stderr, err)
					return err
				}
			}
		}

		trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Pushing image to registry...")
		err := repository.PushImage(trcshDriverConfigBase.DriverConfig, pluginToolConfig)
		if err != nil || (pluginToolConfig["imagesha256"] == nil && pluginToolConfig["trcsha256"] == nil) {
			trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Push image failed: %v", err)
		} else {
			trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Image successfully pushed")
			// Read from Certify
			var pluginName string
			var releaseTag string

			if pluginNameAliasPtr != nil {
				splitPluginAliasVersion := strings.Split(*pluginNameAliasPtr, ":")
				if len(splitPluginAliasVersion) > 1 {
					pluginName = splitPluginAliasVersion[0]
					releaseTag = splitPluginAliasVersion[1]
					pluginToolConfig["trcplugin"] = pluginName
				}
			}

			if !trcshDriverConfigBase.DriverConfig.IsShellSubProcess {
				trcshDriverConfigBase.DriverConfig.StartDir = []string{""}
			}

			pluginVaultPath := fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", pluginName)
			writeMap, readErr := mod.ReadData(pluginVaultPath)
			if readErr != nil || len(writeMap) == 0 {
				writeMap = make(map[string]any)
			}
			if releaseTag != "" {
				writeMap["trcrelease"] = releaseTag // Already validated by pushImage process.
				writeMap["trcplugin"] = pluginName
				if _, ok := writeMap["trctype"]; !ok {
					*pluginTypePtr = "trcshpluginservice"
				}
			}

			_, err = mod.Write(pluginVaultPath, certify.WriteMapUpdate(writeMap, pluginToolConfig, *defineServicePtr, *pluginTypePtr, *pathParamPtr), trcshDriverConfigBase.DriverConfig.CoreConfig.Log)
			if err != nil {
				trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Failed to write staging Certify entry: %v", err)
			} else {
				trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Image successfully pushed")
			}
			return nil
		}
	}

	if len(*buildImagePtr) > 0 {
		// We don't want to allow other functionality due to us not validating the
		// plugintype
		return nil
	}

	// Define Service Image
	if *defineServicePtr {
		eUtils.LogInfo(trcshDriverConfig.DriverConfig.CoreConfig, fmt.Sprintf("Connecting to vault @ %s\n", *trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.VaultAddressPtr))
		writeMap := make(map[string]any)
		writeMap["trcplugin"] = *pluginNamePtr
		writeMap["trctype"] = *pluginTypePtr
		writeMap["trcprojectservice"] = *projectservicePtr
		if _, ok := pluginToolConfig["trcdeployroot"]; ok {
			writeMap["trcdeployroot"] = pluginToolConfig["trcdeployroot"]
		}
		if _, ok := pluginToolConfig["trcdeploysubpath"]; ok {
			writeMap["trcdeploysubpath"] = pluginToolConfig["trcdeploysubpath"]
		}
		if _, ok := pluginToolConfig["trcservicename"]; ok {
			writeMap["trcservicename"] = pluginToolConfig["trcservicename"]
		}
		if _, ok := pluginToolConfig["trccodebundle"]; ok {
			writeMap["trccodebundle"] = pluginToolConfig["trccodebundle"]
		}
		if _, ok := pluginToolConfig["trcprojectservice"]; ok {
			writeMap["trcprojectservice"] = pluginToolConfig["trcprojectservice"]
		}
		if _, ok := pluginToolConfig["trcbootstrap"]; ok {
			writeMap["trcbootstrap"] = pluginToolConfig["trcbootstrap"]
		}

		if pathParam, ok := pluginToolConfig["trcpathparam"].(string); ok && pathParam != "" {
			writeMap["trcpathparam"] = pluginToolConfig["trcpathparam"]
		}
		if expandTarget, ok := pluginToolConfig["trcexpandtarget"].(string); ok && expandTarget == "true" {
			writeMap["trcexpandtarget"] = expandTarget
		}
		if nrAppName, ok := pluginToolConfig["newrelicAppName"].(string); ok && nrAppName != "" {
			writeMap["newrelic_app_name"] = pluginToolConfig["newrelicAppName"].(string)
		}
		if nrLicenseKey, ok := pluginToolConfig["newrelicLicenseKey"].(string); ok && nrLicenseKey != "" {
			writeMap["newrelic_license_key"] = pluginToolConfig["newrelicLicenseKey"].(string)
		}
		if instances, ok := pluginToolConfig["instancesPtr"].(string); ok && instances != "" {
			writeMap["instances"] = instances
		}

		_, err = mod.Write(pluginToolConfig["pluginpath"].(string), writeMap, trcshDriverConfigBase.DriverConfig.CoreConfig.Log)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		fmt.Fprintln(os.Stderr, "Deployment definition applied to vault and is ready for deployments.")
	} else if *winservicestopPtr {
		fmt.Fprintf(os.Stderr, "Stopping service %s\n", pluginToolConfig["trcservicename"].(string))
		cmd := exec.Command("net", "stop", pluginToolConfig["trcservicename"].(string))
		err := cmd.Run()
		if err != nil && strings.Contains(err.Error(), "2185") {
			// Only break if service isn't defined...
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		cmdKill := exec.Command("taskkill", "/F", "/T", "/FI", fmt.Sprintf("\"SERVICES eq %s\"", pluginToolConfig["trcservicename"].(string)))
		cmdKill.Run()
		fmt.Fprintf(os.Stderr, "Service stopped: %s\n", pluginToolConfig["trcservicename"].(string))

	} else if *winservicestartPtr {
		fmt.Fprintf(os.Stderr, "Starting service %s\n", pluginToolConfig["trcservicename"].(string))
		//		cmd := exec.Command("sc", "start", pluginToolConfig["trcservicename"].(string))
		cmd := exec.Command("net", "start", pluginToolConfig["trcservicename"].(string))
		err := cmd.Run()
		if err != nil && strings.Contains(err.Error(), "2185") {
			// Only break if service isn't defined...
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		fmt.Fprintf(os.Stderr, "Service started: %s\n", pluginToolConfig["trcservicename"].(string))
	} else if *codebundledeployPtr {
		if plugincoreopts.BuildOptions.IsPluginHardwired() {
			var deployRoot string
			if deploySubPath, ok := pluginToolConfig["trcdeploysubpath"]; ok {
				deployRoot = filepath.Join(pluginToolConfig["trcdeployroot"].(string), deploySubPath.(string))
			} else {
				deployRoot = pluginToolConfig["trcdeployroot"].(string)
			}
			if _, err = os.Stat(deployRoot); err != nil && !os.IsPermission(err) {
				err = os.MkdirAll(deployRoot, 0o700)
				if err != nil && !os.IsPermission(err) {
					fmt.Fprintln(os.Stderr, err.Error())
					fmt.Fprintln(os.Stderr, "Could not prepare needed directory for deployment.")
				}
			}

			eUtils.LogInfo(trcshDriverConfigBase.DriverConfig.CoreConfig, fmt.Sprintf("Skipping codebundledeploy for hardwired: %s\n", pluginToolConfig["trcplugin"].(string)))
			if isRunnableKernelPlugin {
				return nil
			}
		}
		if pluginToolConfig["trcsha256"] == nil || len(pluginToolConfig["trcsha256"].(string)) == 0 {
			if trcshDriverConfigBase.DriverConfig.DeploymentConfig != nil && (*trcshDriverConfigBase.DriverConfig.DeploymentConfig)["trcsha256"] != nil && len((*trcshDriverConfigBase.DriverConfig.DeploymentConfig)["trcsha256"].(string)) > 0 {
				pluginToolConfig["trcsha256"] = (*trcshDriverConfigBase.DriverConfig.DeploymentConfig)["trcsha256"]
			}
		}

		if pluginToolConfig["trcsha256"] != nil && len(pluginToolConfig["trcsha256"].(string)) > 0 {
			err := repository.GetImageAndShaFromDownload(trcshDriverConfigBase.DriverConfig, pluginToolConfig)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Image download failure.")
				if trcshDriverConfigBase.FeatherCtx != nil {
					trcshDriverConfigBase.FeatherCtx.Log.Printf("Image download failure: %s", err.Error())
				} else {
					fmt.Fprintln(os.Stderr, err.Error())
				}
				return err
			}
		}

		if pluginToolConfig["trcsha256"] != nil &&
			pluginToolConfig["imagesha256"] != nil &&
			pluginToolConfig["trcsha256"].(string) == pluginToolConfig["imagesha256"].(string) {
			// Write the image to the destination...
			var deployPath string
			var deployRoot string
			if deploySubPath, ok := pluginToolConfig["trcdeploysubpath"]; ok {
				deployRoot = filepath.Join(pluginToolConfig["trcdeployroot"].(string), deploySubPath.(string))
			} else {
				deployRoot = pluginToolConfig["trcdeployroot"].(string)
			}

			// check if there is a place holder, if there is replace it
			if strings.Contains(deployRoot, "{{.trcpathparam}}") {
				if pathParam, ok := pluginToolConfig["trcpathparam"].(string); ok && pathParam != "" {
					r, _ := regexp.Compile("^[a-zA-Z0-9_]*$")
					if !r.MatchString(pathParam) {
						fmt.Fprintln(os.Stderr, "trcpathparam can only contain alphanumberic characters or underscores")
						return errors.New("trcpathparam can only contain alphanumberic characters or underscores")
					}
					deployRoot = strings.Replace(deployRoot, "{{.trcpathparam}}", pathParam, -1)
				} else {
					return errors.New("unable to replace path placeholder with pathParam")
				}
			}
			deployPath = filepath.Join(deployRoot, pluginToolConfig["trccodebundle"].(string))

			if eUtils.IsWindows() || !plugincoreopts.BuildOptions.IsPluginHardwired() {
				fmt.Fprintf(os.Stderr, "Deploying image to: %s\n", deployPath)
				if _, err = os.Stat(deployRoot); err != nil && !os.IsPermission(err) {
					err = os.MkdirAll(deployRoot, 0o700)
					if err != nil && !os.IsPermission(err) {
						fmt.Fprintln(os.Stderr, err.Error())
						fmt.Fprintln(os.Stderr, "Could not prepare needed directory for deployment.")
						return err
					}
				}
				if rif, ok := pluginToolConfig["rawImageFile"]; ok {
					err = os.WriteFile(deployPath, rif.([]byte), 0o700)
					if err != nil {
						fmt.Fprintln(os.Stderr, err.Error())
						fmt.Fprintln(os.Stderr, "Image write failure.")
						return err
					}
				}

				if expandTarget, ok := pluginToolConfig["trcexpandtarget"].(string); ok && expandTarget == "true" {
					// TODO: provide archival of existing directory.
					if ok, errList := trcvutils.UncompressZipFile(deployPath); !ok {
						fmt.Fprintf(os.Stderr, "Uncompressing zip file in place failed. %v\n", errList)
						return errList[0]
					} else {
						os.Remove(deployPath)
					}
				} else {
					if strings.HasSuffix(deployPath, ".war") {
						explodedWarPath := strings.TrimSuffix(deployPath, ".war")
						fmt.Fprintf(os.Stderr, "Checking exploded war path: %s\n", explodedWarPath)
						if _, err := os.Stat(explodedWarPath); err == nil {
							if depRoot, ok := pluginToolConfig["trcdeployroot"]; ok {
								deployRoot = depRoot.(string)
							}
							archiveDirPath := filepath.Join(deployRoot, "archive")
							fmt.Fprintf(os.Stderr, "Verifying archive directory: %s\n", archiveDirPath)
							err := os.MkdirAll(archiveDirPath, 0o700)
							if err == nil {
								currentTime := time.Now()
								formattedTime := fmt.Sprintf("%d-%02d-%02d_%02d-%02d-%02d", currentTime.Year(), currentTime.Month(), currentTime.Day(), currentTime.Hour(), currentTime.Minute(), currentTime.Second())
								archiveRoot := filepath.Join(pluginToolConfig["trcdeployroot"].(string), "archive", formattedTime)
								fmt.Fprintf(os.Stderr, "Verifying archive backup directory: %s\n", archiveRoot)
								err := os.MkdirAll(archiveRoot, 0o700)
								if err == nil {
									archivePath := filepath.Join(archiveRoot, pluginToolConfig["trccodebundle"].(string))
									archivePath = strings.TrimSuffix(archivePath, ".war")
									fmt.Fprintf(os.Stderr, "Archiving: %s to %s\n", explodedWarPath, archivePath)
									os.Rename(explodedWarPath, archivePath)
								}
							}
						}
					}
				}
				fmt.Fprintf(os.Stderr, "Image deployed to: %s\n", deployPath)
			}
		} else {
			errMessage := fmt.Sprintf("image not certified.  cannot deploy image for %s", pluginToolConfig["trcplugin"])
			if trcshDriverConfigBase.FeatherCtx != nil {
				fmt.Fprintf(os.Stderr, "%s\n", errMessage)
				trcshDriverConfigBase.FeatherCtx.Log.Print(errMessage)
			} else {
				fmt.Fprintf(os.Stderr, "%s\n", errMessage)
			}
			return errors.New(errMessage)
		}
		if ptcsha256, ok := pluginToolConfig["trcsha256"]; ok && isRunnableKernelPlugin {
			trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Println("Starting verification of plugin module.")
			h := sha256.New()
			pathToSO := hive.LoadPluginPath(trcshDriverConfigBase.DriverConfig, pluginToolConfig)
			f, err := os.OpenFile(pathToSO, os.O_RDONLY, 0o600)
			if err != nil {
				trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Could not load plugin due to bad deploy path in certification: %s\n", pathToSO)
				return err
			}
			defer f.Close()
			err = memprotectopts.SetChattr(f)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return err
			}
			if _, err := io.Copy(h, f); err != nil {
				fmt.Fprintf(os.Stderr, "Unable to copy file: %s\n", err)
				trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Unable to copy file: %s\n", err)
				return err
			}
			sha := hex.EncodeToString(h.Sum(nil))
			if plugincoreopts.BuildOptions.IsPluginHardwired() || (ptcsha256.(string) == sha) {
				trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Println("Verified plugin module sha.")
				err = memprotectopts.UnsetChattr(f)
				if err != nil {
					return err
				}
				if pluginHandler != nil {
					if pluginHandler.State == 2 && sha == pluginHandler.Signature { // make sure this won't break...not set yet
						trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Tried to redeploy same failed plugin: %s\n", *pluginNamePtr)
						// do we want to remove from available services???
					} else {
						if s, ok := pluginToolConfig["trctype"].(string); ok && (s == "trcshpluginservice" || s == "trcflowpluginservice") {
							pluginHandler.LoadPluginMod(trcshDriverConfigBase.DriverConfig, pathToSO)
						}
						pluginHandler.Signature = sha
					}
				} else {
					fmt.Fprintf(os.Stderr, "Handler not initialized for plugin to start: %s\n", *pluginNamePtr)
					trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Handler not initialized for plugin to start: %s\n", *pluginNamePtr)
				}
			}
		}
	} else if *certifyImagePtr {
		// Certify Image
		carrierCertify := false
		// Certification always operates on env basis.
		mod.EnvBasis = driverConfig.CoreConfig.EnvBasis
		mod.Env = mod.EnvBasis
		if ptc, ok := pluginToolConfig["trcplugin"].(string); ok && strings.Contains(ptc, "carrier") {
			fmt.Fprintln(os.Stderr, "Skipping checking for existing image due to carrier deployment.")
			carrierCertify = true
		} else if !certifyInit {
			// Already certified...
			fmt.Fprintln(os.Stderr, "Checking for existing image.")
			err := repository.GetImageAndShaFromDownload(trcshDriverConfigBase.DriverConfig, pluginToolConfig)
			if _, ok := pluginToolConfig["imagesha256"].(string); err != nil || !ok {
				fmt.Fprintln(os.Stderr, "Invalid or nonexistent image on download.")
				if err != nil {
					fmt.Fprintln(os.Stderr, err.Error())
				}
				if err == nil {
					err = errors.New("invalid or nonexistent image on download")
				}
				return err
			}
		}
		if certifyInit || carrierCertify || pluginToolConfig["trcsha256"].(string) == pluginToolConfig["imagesha256"].(string) { // Comparing generated sha from image to sha from flag
			// ||
			//(pluginToolConfig["imagesha256"].(string) != "" && pluginToolConfig["trctype"].(string) == "trcshservice") {
			if !strings.Contains(pluginToolConfig["trcplugin"].(string), "carrier") {
				fmt.Fprintln(os.Stderr, "Valid image found.")
			}
			// SHA MATCHES
			eUtils.LogInfo(trcshDriverConfig.DriverConfig.CoreConfig, fmt.Sprintf("Connecting to vault @ %s\n", *trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.VaultAddressPtr))
			trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Println("Curator getting plugin settings for env: " + mod.Env)
			// The following confirms that this version of carrier has been certified to run...
			// It will bail if it hasn't.
			if _, pluginPathOk := pluginToolConfig["pluginpath"].(string); !pluginPathOk { // If region is set
				mod.SectionName = "trcplugin"
				mod.SectionKey = "/Index/"

				pluginSource := pluginToolConfig["trcplugin"].(string)
				if strings.HasPrefix(*pluginNamePtr, pluginSource) {
					pluginSource = *pluginNamePtr
				}
				mod.SubSectionValue = pluginSource
				trcshDriverConfigBase.DriverConfig.SubSectionValue = pluginSource

				if !trcshDriverConfigBase.DriverConfig.IsShellSubProcess {
					trcshDriverConfigBase.DriverConfig.StartDir = []string{""}
				}

				properties, err := trcvutils.NewProperties(trcshDriverConfigBase.DriverConfig.CoreConfig, vault, mod, mod.Env, "TrcVault", "Certify")
				if err != nil && !strings.Contains(err.Error(), "no data paths found when initing CDS") {
					fmt.Fprintln(os.Stderr, "Couldn't create properties for regioned certify:"+err.Error())
					return err
				}

				writeMap, replacedFields := properties.GetPluginData(*regionPtr, "Certify", "config", trcshDriverConfigBase.DriverConfig.CoreConfig.Log)

				pluginTarget := pluginToolConfig["trcplugin"].(string)
				if strings.HasPrefix(*pluginNamePtr, pluginTarget) {
					pluginTarget = *pluginNamePtr
				}
				writeErr := properties.WritePluginData(certify.WriteMapUpdate(writeMap, pluginToolConfig, *defineServicePtr, *pluginTypePtr, *pathParamPtr), replacedFields, mod, trcshDriverConfigBase.DriverConfig.CoreConfig.Log, *regionPtr, pluginTarget)
				if writeErr != nil {
					fmt.Fprintln(os.Stderr, writeErr)
					return err
				}
				fmt.Fprintln(os.Stderr, "Image certified.")
			} else { // Non region certify
				writeMap, readErr := mod.ReadData(pluginToolConfig["pluginpath"].(string))
				if readErr != nil {
					if trcshDriverConfig.DriverConfig.CoreConfig.TokenCache != nil {
						mod.EmptyCache()
						trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.Clear()
					}
					fmt.Fprintln(os.Stderr, readErr)
					return err
				}

				_, err = mod.Write(pluginToolConfig["pluginpath"].(string), certify.WriteMapUpdate(writeMap, pluginToolConfig, *defineServicePtr, *pluginTypePtr, *pathParamPtr), trcshDriverConfigBase.DriverConfig.CoreConfig.Log)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					return err
				}
				fmt.Fprintln(os.Stderr, "Image certified in vault and is ready for release.")
			}
		} else {
			fmt.Fprintln(os.Stderr, "Invalid or nonexistent image.")
			return err
		}
	} else if *certifyInfoImagePtr {
		eUtils.LogInfo(trcshDriverConfig.DriverConfig.CoreConfig, fmt.Sprintf("Connecting to vault @ %s\n", *trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.VaultAddressPtr))
		trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Println("Trcplgtool getting plugin settings for env: " + mod.Env)
		// The following confirms that this version of carrier has been certified to run...
		// It will bail if it hasn't.
		if _, pluginPathOk := pluginToolConfig["pluginpath"].(string); !pluginPathOk {

			type VersionRow struct {
				Env       string                 `json:"env"`
				Version   string                 `json:"version"`
				TrcSha256 string                 `json:"trcsha256"`
				Data      map[string]interface{} `json:"data"`
			}
			envs := []string{"dev", "QA", "staging"}
			grouped := make(map[string][]VersionRow) // trcsha256 -> []VersionRow

			//
			// Prepare modifier for reading plugin version data
			//
			mod.SectionName = "trcplugin"
			mod.SectionKey = "/Index/"

			pluginSource := pluginToolConfig["trcplugin"].(string)
			if strings.HasPrefix(*pluginNamePtr, pluginSource) {
				pluginSource = *pluginNamePtr
			}
			mod.SubSectionValue = pluginSource
			trcshDriverConfigBase.DriverConfig.SubSectionValue = pluginSource

			if !trcshDriverConfigBase.DriverConfig.IsShellSubProcess {
				trcshDriverConfigBase.DriverConfig.StartDir = []string{""}
			}
			mod.ProjectIndex = []string{"TrcVault"}
			driverConfig.VersionFilter = []string{"Certify"}
			//
			// End plugin modifier preparations
			//
			for _, env := range envs {
				mod.Env = env
				driverConfig.CoreConfig.EnvBasis = env
				versionMetadataMap := eUtils.GetProjectVersionInfo(driverConfig, mod)
				for _, versionMap := range versionMetadataMap {
					// Get sorted version keys (assume versionMap is map[string]any)
					var versionKeys []string
					for k := range versionMap {
						versionKeys = append(versionKeys, k)
					}
					// Sort versionKeys numerically so earliest (oldest) is first
					sort.Slice(versionKeys, func(i, j int) bool {
						vi, err1 := strconv.Atoi(versionKeys[i])
						vj, err2 := strconv.Atoi(versionKeys[j])
						if err1 == nil && err2 == nil {
							return vi < vj
						}
						// fallback to string compare if not numeric
						return versionKeys[i] < versionKeys[j]
					})
					// Take last 10 (or all if <10), but keep earliest first
					if len(versionKeys) > 10 {
						versionKeys = versionKeys[len(versionKeys)-10:]
					}
					for _, versionKey := range versionKeys {
						//							mod.SectionKey = "/"
						mod.Version = versionKey
						pluginMap, err := mod.ReadData(fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", *pluginNamePtr))
						if err != nil || pluginMap == nil {
							continue
						}
						trcsha, _ := pluginMap["trcsha256"].(string)
						row := VersionRow{
							Env:       env,
							Version:   versionKey,
							TrcSha256: trcsha,
							Data:      pluginMap,
						}
						if trcsha != "" {
							grouped[trcsha] = append(grouped[trcsha], row)
						}
					}
				}
			}
			// Prune grouped: keep only the most recent version for each sha256 in each environment
			pruned := make(map[string][]VersionRow)
			envs = []string{"dev", "QA", "staging"}
			for sha, rows := range grouped {
				envLatest := make(map[string]VersionRow)
				envVer := make(map[string]string)
				for _, r := range rows {
					env := r.Env
					v := r.Version
					vi, err1 := strconv.Atoi(v)
					vmax, err2 := strconv.Atoi(envVer[env])
					if _, ok := envVer[env]; !ok || (err1 == nil && err2 == nil && vi > vmax) || (err1 == nil && err2 != nil) || (err1 != nil && v > envVer[env]) {
						envLatest[env] = r
						envVer[env] = v
					}
				}
				for _, env := range envs {
					if latest, ok := envLatest[env]; ok {
						pruned[sha] = append(pruned[sha], latest)
					}
				}
			}
			// Sort pruned rows for each sha256 so higher dev version appears first
			// Safe sorting: build a new slice, sort, then assign
			for sha, rows := range pruned {
				devVersion := 0
				for _, r := range rows {
					if r.Env == "dev" {
						v, err := strconv.Atoi(r.Version)
						if err == nil && v > devVersion {
							devVersion = v
						}
					}
				}
				newRows := make([]VersionRow, len(rows))
				copy(newRows, rows)
				sort.SliceStable(newRows, func(i, j int) bool {
					var vi, vj int
					if newRows[i].Env == "dev" {
						vi, _ = strconv.Atoi(newRows[i].Version)
					} else {
						vi = devVersion
					}
					if newRows[j].Env == "dev" {
						vj, _ = strconv.Atoi(newRows[j].Version)
					} else {
						vj = devVersion
					}
					return vi > vj
				})
				pruned[sha] = newRows
			}
			// Convert pruned map into an ordered slice of groups so JSON output is
			// deterministic and can be sorted by the dev version for each group.
			type RowGroup struct {
				TrcSha string       `json:"trcsha256"`
				Rows   []VersionRow `json:"rows"`
			}

			// helper struct used for sorting by dev version
			type groupWithDev struct {
				sha       string
				rows      []VersionRow
				devNumber int
			}

			var groups []groupWithDev
			for sha, rows := range pruned {
				devNumber := 0
				for _, r := range rows {
					if r.Env == "dev" {
						if v, err := strconv.Atoi(r.Version); err == nil && v > devNumber {
							devNumber = v
						}
					}
				}
				groups = append(groups, groupWithDev{sha: sha, rows: rows, devNumber: devNumber})
			}

			// sort groups by devNumber descending (highest dev version first)
			sort.SliceStable(groups, func(i, j int) bool {
				return groups[i].devNumber > groups[j].devNumber
			})

			// build final ordered slice for JSON
			var outRows []RowGroup
			for _, g := range groups {
				outRows = append(outRows, RowGroup{TrcSha: g.sha, Rows: g.rows})
			}

			type OutputReport struct {
				Date string     `json:"date"`
				Rows []RowGroup `json:"rows"`
			}

			report := OutputReport{
				Date: time.Now().Format("2006-01-02_15-04-05"),
				Rows: outRows,
			}

			jsonBytes, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				fmt.Fprintln(os.Stderr, "Failed to marshal JSON report:", err)
				return err
			}
			if len(*outputDestinationPtr) > 0 {
				fname := fmt.Sprintf("trcplgtool_report_%s.json", report.Date)
				err = os.WriteFile(fname, jsonBytes, 0o644)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Failed to write JSON report:", err)
					return err
				}
				fmt.Fprintf(os.Stderr, "JSON report written to %s\n", fname)
			} else {
				fmt.Println(string(jsonBytes))
			}
		}
	} else if *agentdeployPtr {
		if trcshConfig.FeatherCtlCb != nil {
			err := trcshConfig.FeatherCtlCb(trcshConfig.FeatherCtx, *pluginNamePtr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Incorrect installation: %s\n", err.Error())
				return err
			}
		} else {
			fmt.Fprintln(os.Stderr, "Incorrect trcplgtool utilization")
			return err
		}
	} else if *pluginservicestartPtr && isRunnableKernelPlugin {
		if pluginHandler != nil && pluginHandler.State != 2 && kernelPluginHandler != nil {
			if kernelPluginHandler.ConfigContext == nil || kernelPluginHandler.ConfigContext.ChatReceiverChan == nil {
				fmt.Fprintf(os.Stderr, "Unable to access chat channel configuration data for %s\n", *pluginNamePtr)
				driverConfig.CoreConfig.Log.Printf("Unable to access chat channel configuration data for %s\n", *pluginNamePtr)
			} else {
				trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Starting plugin service: %s\n", *pluginNamePtr)
				pluginHandler.PluginserviceStart(trcshDriverConfigBase.DriverConfig, pluginToolConfig)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Handler not initialized for plugin to start: %s\n", *pluginNamePtr)
			trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Handler not initialized for plugin to start: %s\n", *pluginNamePtr)
		}
	} else if *pluginservicestopPtr && isRunnableKernelPlugin {
		if pluginHandler != nil && pluginHandler.State != 2 {
			pluginHandler.PluginserviceStop(trcshDriverConfigBase.DriverConfig)
		} else {
			fmt.Fprintf(os.Stderr, "Handler not initialized for plugin to shutdown: %s\n", *pluginNamePtr)
			trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Handler not initialized for plugin to shutdown: %s\n", *pluginNamePtr)
		}
	}
	// Checks if image has been copied & deployed
	if *checkDeployedPtr {
		if (pluginToolConfig["copied"] != nil && pluginToolConfig["copied"].(bool)) &&
			(pluginToolConfig["deployed"] != nil && pluginToolConfig["deployed"].(bool)) &&
			(pluginToolConfig["trcsha256"] != nil && pluginToolConfig["trcsha256"].(string) == *sha256Ptr) { // Compare vault sha with provided sha
			fmt.Fprintln(os.Stderr, "Plugin has been copied, deployed & certified.")
			return nil
		}

		err := repository.GetImageAndShaFromDownload(trcshDriverConfigBase.DriverConfig, pluginToolConfig)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return err
		}

		if *sha256Ptr == pluginToolConfig["imagesha256"].(string) { // Compare repo image sha with provided sha
			fmt.Fprintln(os.Stderr, "Latest plugin image sha matches provided plugin sha.  It has been certified.")
		} else {
			fmt.Fprintln(os.Stderr, "Provided plugin sha is not deployable.")
			return errors.New("provided plugin sha is not deployable")
		}

		fmt.Fprintln(os.Stderr, "Plugin has not been copied or deployed.")
		return nil
	}

	if *checkCopiedPtr {
		if pluginToolConfig["copied"].(bool) && pluginToolConfig["trcsha256"].(string) == *sha256Ptr { // Compare vault sha with provided sha
			fmt.Fprintln(os.Stderr, "Plugin has been copied & certified.")
			return nil
		}

		err := repository.GetImageAndShaFromDownload(trcshDriverConfigBase.DriverConfig, pluginToolConfig)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return err
		}

		if *sha256Ptr == pluginToolConfig["imagesha256"].(string) { // Compare repo image sha with provided sha
			fmt.Fprintln(os.Stderr, "Latest plugin image sha matches provided plugin sha.  It has been certified.")
		} else {
			fmt.Fprintln(os.Stderr, "Provided plugin sha is not certified.")
			return errors.New("provided plugin sha is not certified")
		}

		fmt.Fprintln(os.Stderr, "Plugin has not been copied or deployed.")
		return nil
	}
	return nil
}

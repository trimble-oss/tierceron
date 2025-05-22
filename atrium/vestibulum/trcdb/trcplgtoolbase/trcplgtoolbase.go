package trcplgtoolbase

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/trimble-oss/tierceron/atrium/vestibulum/pluginutil/certify"
	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/kernelopts"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/buildopts/pluginopts"
	"github.com/trimble-oss/tierceron/pkg/capauth"
	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"
	"github.com/trimble-oss/tierceron/pkg/core/util/docker"
	"github.com/trimble-oss/tierceron/pkg/core/util/hive"
	"github.com/trimble-oss/tierceron/pkg/core/util/repository"
	"github.com/trimble-oss/tierceron/pkg/utils"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"

	trcapimgmtbase "github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/trcapimgmtbase"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/trccertmgmtbase"
)

func CommonMain(envPtr *string,
	envCtxPtr *string,
	tokenNamePtr *string,
	regionPtr *string,
	flagset *flag.FlagSet,
	argLines []string,
	trcshDriverConfig *capauth.TrcshDriverConfig,
	mainPluginHandler ...*hive.PluginHandler) error {

	var flagEnvPtr *string
	var tokenPtr *string
	var addrPtr *string
	// Main functions are as follows:
	if flagset == nil {
		fmt.Println("Version: " + "1.05")
		flagset = flag.NewFlagSet(argLines[0], flag.ContinueOnError)
		// set and ignore..
		flagEnvPtr = flagset.String("env", "dev", "Environment to configure")
		flagset.String("addr", "", "API endpoint for the vault")
		flagset.String("token", "", "Vault access token")
		flagset.String("region", "", "Region to be processed") //If this is blank -> use context otherwise override context.
		flagset.Usage = func() {
			fmt.Fprintf(flagset.Output(), "Usage of %s:\n", argLines[0])
			flagset.PrintDefaults()
		}
	} else {
		tokenPtr = flagset.String("token", "", "Vault access token")
		addrPtr = flagset.String("addr", "", "API endpoint for the vault")
	}
	defineServicePtr := flagset.Bool("defineService", false, "Service is defined.")
	certifyImagePtr := flagset.Bool("certify", false, "Used to certifies vault plugin.")
	// These functions only valid for pluginType trcshservice
	pluginservicestartPtr := flagset.Bool("pluginservicestart", false, "To start a trcshell kernel service for a particular plugin.")
	pluginservicestopPtr := flagset.Bool("pluginservicestop", false, "To stop a trcshell kernel service for a particular plugin.")
	winservicestopPtr := flagset.Bool("winservicestop", false, "To stop a windows service for a particular plugin.")
	winservicestartPtr := flagset.Bool("winservicestart", false, "To start a windows service for a particular plugin.")
	codebundledeployPtr := flagset.Bool("codebundledeploy", false, "To deploy a code bundle.")
	agentdeployPtr := flagset.Bool("agentdeploy", false, "To initiate deployment on agent.")
	projectservicePtr := flagset.String("projectservice", "", "Provide template root path in form project/service")
	buildImagePtr := flagset.String("buildImage", "", "Path to Dockerfile to build")
	pushImagePtr := flagset.Bool("pushImage", false, "Push an image to the registry.")
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

	// Common plugin flags...
	pluginNamePtr := flagset.String("pluginName", "", "Used to certify vault plugin")
	pluginNameAliasPtr := flagset.String("pluginNameAlias", "", "Name used to define an alias for a plugin")
	pluginTypePtr := flagset.String("pluginType", "vault", "Used to indicate type of plugin.  Default is vault.")

	// Certify flags...
	sha256Ptr := flagset.String("sha256", "", "Used to certify vault plugin") //This has to match the image that is pulled -> then we write the vault.
	checkDeployedPtr := flagset.Bool("checkDeployed", false, "Used to check if plugin has been copied, deployed, & certified")
	checkCopiedPtr := flagset.Bool("checkCopied", false, "Used to check if plugin has been copied & certified")

	// NewRelic flags...
	newrelicAppNamePtr := flagset.String("newRelicAppName", "", "App name for New Relic")
	newrelicLicenseKeyPtr := flagset.String("newRelicLicenseKey", "", "License key for New Relic")
	logFilePtr := flagset.String("log", "./"+coreopts.BuildOptions.GetFolderPrefix(nil)+"plgtool.log", "Output path for log files")

	certifyInit := false

	//APIM flags
	updateAPIMPtr := flagset.Bool("updateAPIM", false, "Used to update Azure APIM")

	// Cert flags
	certPathPtr := flagset.String("certPath", "", "Path to certificate to push to Azure")

	if !trcshDriverConfig.DriverConfig.CoreConfig.IsShell {
		args := argLines[1:]
		for i := 0; i < len(args); i++ {
			s := args[i]
			if s[0] != '-' {
				fmt.Println("Wrong flag syntax: ", s)
				return fmt.Errorf("wrong flag syntax: %s", s)
			}
		}
		err := flagset.Parse(argLines[1:])
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
		err := flagset.Parse(argLines)
		if err != nil {
			return err
		}
		err = flagset.Parse(argLines[1:])
		if err != nil {
			return err
		}
		trcshDriverConfig.DriverConfig.CoreConfig.CurrentTokenNamePtr = tokenNamePtr
	}
	if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/"+coreopts.BuildOptions.GetFolderPrefix(nil)+"plgtool.log" {
		*logFilePtr = "./" + coreopts.BuildOptions.GetFolderPrefix(nil) + "plgtool.log"
	}
	f, errLog := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if errLog != nil {
		fmt.Printf("Could not open logfile: %s\n", *logFilePtr)
		os.Exit(1)
	}

	trcshDriverConfig.DriverConfig.CoreConfig.Log = log.New(f, "[INIT]", log.LstdFlags)

	if utils.RefLength(addrPtr) == 0 {
		eUtils.ReadAuthParts(trcshDriverConfig.DriverConfig, false)
	} else {
		trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.SetVaultAddress(addrPtr)
	}

	if trcshDriverConfig != nil && trcshDriverConfig.DriverConfig.DeploymentConfig["trcpluginalias"] != nil {
		// Prefer internal definition of alias
		*pluginNameAliasPtr = trcshDriverConfig.DriverConfig.DeploymentConfig["trcpluginalias"].(string)
	}

	if (len(*newrelicAppNamePtr) == 0 && len(*newrelicLicenseKeyPtr) != 0) || (len(*newrelicAppNamePtr) != 0 && len(*newrelicLicenseKeyPtr) == 0) {
		fmt.Println("Must use -newrelicAppName && -newrelicLicenseKey flags together to use -certify flag")
		return errors.New("must use -newrelicAppName && -newrelicLicenseKey flags together to use -certify flag")
	}

	if *certifyImagePtr && (len(*pluginNamePtr) == 0 || len(*sha256Ptr) == 0) {
		fmt.Println("Must use -pluginName && -sha256 flags to use -certify flag")
		return errors.New("must use -pluginName && -sha256 flags to use -certify flag")
	}

	if *checkDeployedPtr && (len(*pluginNamePtr) == 0) {
		fmt.Println("Must use -pluginName flag to use -checkDeployed flag")
		return errors.New("must use -pluginName flag to use -checkDeployed flag")
	}

	if *defineServicePtr && (len(*pluginNamePtr) == 0) {
		fmt.Println("Must use -pluginName flag to use -defineService flag")
		return errors.New("must use -pluginName flag to use -defineService flag")
	}

	if *defineServicePtr && !regexp.MustCompile(`^[a-z0-9._/-]+$`).MatchString(*pluginNamePtr) {
		fmt.Println("pluginName can only include lowercase alphanumeric characters, periods, dashes, underscores, and forward slashes")
		return errors.New("pluginName can only include lowercase alphanumeric characters, periods, dashes, underscores, and forward slashes")
	}

	if *pushImagePtr && (len(*pluginNamePtr) == 0) {
		fmt.Println("Must use -pluginName flag to use -pushimage flag")
		return errors.New("must use -pluginName flag to use -pushimage flag")
	}

	if len(*buildImagePtr) > 0 && len(*pluginNamePtr) == 0 {
		fmt.Println("Must use -pluginName flag to use -buildImage flag")
		return errors.New("must use -pluginName flag to use -buildImage flag")
	}

	if len(*buildImagePtr) > 0 && len(strings.Split(*pluginNamePtr, ":")[0]) > 128 {
		fmt.Println("Image tag cannot be longer than 128 characters")
		return errors.New("image tag cannot be longer than 128 characters")
	}

	if len(*pushAliasPtr) > 0 && !*pushImagePtr {
		fmt.Println("Must use -pushImage flag to use -pushAlias flag")
		return errors.New("must use -pushImage flag to use -pushAlias flag")
	}

	if len(*certPathPtr) > 0 && !*updateAPIMPtr {
		fmt.Println("Must use -updateAPIM flag to use -certPath flag")
		return errors.New("must use -updateAPIM flag to use -certPath flag")
	}

	if trcshDriverConfig != nil && len(trcshDriverConfig.DriverConfig.PathParam) > 0 {
		// Prefer internal definition of alias
		*pathParamPtr = trcshDriverConfig.DriverConfig.PathParam
	}

	if len(*pathParamPtr) > 0 {
		r, _ := regexp.Compile("^[a-zA-Z0-9_]*$")
		if !r.MatchString(*pathParamPtr) {
			fmt.Println("-pathParam can only contain alphanumberic characters or underscores")
			return errors.New("-pathParam can only contain alphanumberic characters or underscores")
		}
	}
	if *agentdeployPtr || *winservicestopPtr || *winservicestartPtr || *codebundledeployPtr || *pluginservicestopPtr || *pluginservicestartPtr {
		*pluginTypePtr = "trcshservice"
	}

	if !*updateAPIMPtr && len(*buildImagePtr) == 0 && !*pushImagePtr {
		switch *pluginTypePtr {
		case "vault": // A vault plugin
			if trcshDriverConfig.DriverConfig.CoreConfig.IsShell {
				// TODO: do we want to support Deployment certifications in the pipeline at some point?
				// If so this is a config check to remove.
				fmt.Printf("Plugin type %s not supported in trcsh.\n", *pluginTypePtr)
				return fmt.Errorf("plugin type %s not supported in trcsh", *pluginTypePtr)
			}
			if *codebundledeployPtr {
				fmt.Printf("codebundledeploy not supported for plugin type %s in trcsh\n", *pluginTypePtr)
				return fmt.Errorf("codebundledeploy not supported for plugin type %s in trcsh", *pluginTypePtr)
			}

		case "agent": // A deployment agent tool.
			if trcshDriverConfig.DriverConfig.CoreConfig.IsShell {
				// TODO: do we want to support Deployment certifications in the pipeline at some point?
				// If so this is a config check to remove.
				fmt.Printf("Plugin type %s not supported in trcsh.\n", *pluginTypePtr)
				return fmt.Errorf("plugin type %s not supported in trcsh", *pluginTypePtr)
			}
			if *codebundledeployPtr {
				fmt.Printf("codebundledeploy not supported for plugin type %s in trcsh\n", *pluginTypePtr)
				return fmt.Errorf("codebundledeploy not supported for plugin type %s in trcsh", *pluginTypePtr)
			}
		case "trccmdtool": // A trc command line tool.
		case "trcshservice": // A trcshservice managed microservice
		case "trcshkubeservice":
		case "trcshpluginservice":
		case "trcshmutabilispraefecto":
		case "trcshcmdtoolplugin":
		default:
			if !*agentdeployPtr {
				fmt.Println("Unsupported plugin type: " + *pluginTypePtr)
				return fmt.Errorf("unsupported plugin type: %s", *pluginTypePtr)
			} else {
				fmt.Printf("\nBeginning agent deployment for %s..\n", *pluginNamePtr)
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
				fmt.Println("Unsupported agentdeploy outside trcsh")
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
		fmt.Println("Unsupported agentdeploy outside trcsh")
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

	pluginConfig := map[string]interface{}{}
	pluginConfig = buildopts.BuildOptions.ProcessPluginEnvConfig(pluginConfig) //contains logNamespace for InitVaultMod
	if pluginConfig == nil {
		fmt.Println("Error: Could not find plugin config")
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
	if trcshConfig.FeatherCtx != nil && flagEnvPtr != nil && strings.HasPrefix(*flagEnvPtr, *trcshConfig.FeatherCtx.Env) {
		// take on the environment context of the provided flag... like dev-1
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
	} else if deploy_plugin, ok := trcshDriverConfigBase.DriverConfig.DeploymentConfig["trcplugin"]; ok {
		if subsv, k := deploy_plugin.(string); k {
			trcshDriverConfigBase.DriverConfig.SubSectionValue = subsv
			*pluginNamePtr = subsv
		}
	}

	var pluginHandler *hive.PluginHandler = nil
	var kernelPluginHandler *hive.PluginHandler = nil
	if *pluginNamePtr == "" {
		if deploy_plugin, ok := trcshDriverConfigBase.DriverConfig.DeploymentConfig["trcplugin"]; ok {
			if dep, k := deploy_plugin.(string); k {
				*pluginNamePtr = dep
			} else {
				trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Println("Unexpected type for plugin name.")
			}
		} else {
			trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Println("Unable to set plugin name.")
		}
	}
	if kernelopts.BuildOptions.IsKernel() {
		if len(mainPluginHandler) > 0 && mainPluginHandler[0] != nil && mainPluginHandler[0].Services != nil {
			kernelPluginHandler = mainPluginHandler[0]
			pluginHandler = kernelPluginHandler.GetPluginHandler(*pluginNamePtr, trcshDriverConfigBase.DriverConfig)
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
				fmt.Println("Unsupported region: " + *regionPtr)
				return fmt.Errorf("unsupported region: %s", *regionPtr)
			}
		}
		trcshDriverConfigBase.DriverConfig.CoreConfig.Regions = regions
	}

	if *updateAPIMPtr {
		var apimError error
		if len(*certPathPtr) > 0 {
			apimError = trccertmgmtbase.CommonMain(certPathPtr, driverConfig, mod)
		} else {
			apimError = trcapimgmtbase.CommonMain(envPtr, nil, tokenNamePtr, regionPtr, startDirPtr, driverConfig, mod)
		}
		if apimError != nil {
			fmt.Println(apimError.Error())
			fmt.Println("Couldn't update APIM...proceeding with build")
		}
		return nil
	}

	// Get existing configs if they exist...
	pluginToolConfig, plcErr := trcvutils.GetPluginToolConfig(trcshDriverConfigBase.DriverConfig, mod, coreopts.BuildOptions.InitPluginConfig(map[string]interface{}{}), *defineServicePtr)
	if plcErr != nil {
		fmt.Println(plcErr.Error())
		return plcErr
	}
	if *certifyImagePtr {
		trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Certify begin activities\n")
	}

	if len(*sha256Ptr) > 0 {
		fileInfo, statErr := os.Stat(*sha256Ptr)
		if statErr == nil {
			if fileInfo.Mode().IsRegular() {
				file, fileOpenErr := os.Open(*sha256Ptr)
				if fileOpenErr != nil {
					fmt.Println(fileOpenErr.Error())
					return fileOpenErr
				}

				// Close the file when we're done.
				defer file.Close()

				// Create a reader to the file.
				reader := bufio.NewReader(file)

				pluginImage, imageErr := io.ReadAll(reader)
				if imageErr != nil {
					fmt.Println("Failed to read image:" + imageErr.Error())
					return imageErr
				}
				sha256Bytes := sha256.Sum256(pluginImage)
				*sha256Ptr = fmt.Sprintf("%x", sha256Bytes)
			} else {
				fmt.Println("Irregular image")
				return errors.New("irregular image")
			}
		} else {
			fmt.Println("Failure to stat image:" + statErr.Error())
			return statErr
		}
	}

	if !*codebundledeployPtr && len(*sha256Ptr) > 0 {
		pluginToolConfig["trcsha256"] = *sha256Ptr
	}
	pluginToolConfig["pluginNamePtr"] = *pluginNamePtr
	pluginToolConfig["serviceNamePtr"] = *serviceNamePtr
	pluginToolConfig["projectservicePtr"] = *projectservicePtr
	pluginToolConfig["deployrootPtr"] = *deployrootPtr
	pluginToolConfig["deploysubpathPtr"] = *deploysubpathPtr
	pluginToolConfig["codeBundlePtr"] = *codeBundlePtr
	pluginToolConfig["pathParamPtr"] = *pathParamPtr
	pluginToolConfig["expandTargetPtr"] = *expandTargetPtr //is a bool that gets converted to a string for writeout/certify
	pluginToolConfig["newrelicAppName"] = *newrelicAppNamePtr
	pluginToolConfig["newrelicLicenseKey"] = *newrelicLicenseKeyPtr
	pluginToolConfig["buildImagePtr"] = *buildImagePtr
	pluginToolConfig["pushAliasPtr"] = *pushAliasPtr
	pluginToolConfig["trcbootstrapPtr"] = trcbootstrapPtr

	if _, ok := pluginToolConfig["trcplugin"].(string); !ok {
		if *defineServicePtr {
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
				fmt.Println("Service definition not supported in trcsh.")
				os.Exit(-1)
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
			if expandTarget, ok := pluginToolConfig["expandTargetPtr"].(bool); ok && expandTarget { //only writes out if expandTarget = true
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
			err := errors.New("trcplugin not defined, can not continue")
			fmt.Println(err)
			return err
		}
	}

	if len(*buildImagePtr) > 0 {
		fmt.Println("Building image using local docker repository...")
		err := docker.BuildDockerImage(trcshDriverConfigBase.DriverConfig, *buildImagePtr, *pluginNamePtr)
		if err != nil {
			fmt.Println(err.Error())
			return err
		} else {
			fmt.Println("Image successfully built")
		}
	}

	if *pushImagePtr {
		if len(pluginToolConfig["pushAliasPtr"].(string)) > 0 {
			aliases := strings.Split(pluginToolConfig["pushAliasPtr"].(string), ",")
			for _, alias := range aliases {
				if strings.Split(alias, ":")[0] != strings.Split(pluginToolConfig["pluginNamePtr"].(string), ":")[0] {
					err := errors.New("pushAlias can only alias image tags, not image names")
					fmt.Println(err)
					return err
				}
			}
		}

		fmt.Println("Pushing image to registry...")
		err := repository.PushImage(trcshDriverConfigBase.DriverConfig, pluginToolConfig)
		if err != nil {
			fmt.Println(err.Error())
		} else {
			fmt.Println("Image successfully pushed")
		}
	}

	if len(*buildImagePtr) > 0 {
		// We don't want to allow other functionality due to us not validating the
		// plugintype
		return nil
	}

	//Define Service Image
	if *defineServicePtr {
		fmt.Printf("Connecting to vault @ %s\n", *trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.VaultAddressPtr)
		writeMap := make(map[string]interface{})
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

		_, err = mod.Write(pluginToolConfig["pluginpath"].(string), writeMap, trcshDriverConfigBase.DriverConfig.CoreConfig.Log)
		if err != nil {
			fmt.Println(err)
			return err
		}
		fmt.Println("Deployment definition applied to vault and is ready for deployments.")
	} else if *winservicestopPtr {
		fmt.Printf("Stopping service %s\n", pluginToolConfig["trcservicename"].(string))
		cmd := exec.Command("net", "stop", pluginToolConfig["trcservicename"].(string))
		err := cmd.Run()
		if err != nil && strings.Contains(err.Error(), "2185") {
			// Only break if service isn't defined...
			fmt.Println(err)
			return err
		}
		cmdKill := exec.Command("taskkill", "/F", "/T", "/FI", fmt.Sprintf("\"SERVICES eq %s\"", pluginToolConfig["trcservicename"].(string)))
		cmdKill.Run()
		fmt.Printf("Service stopped: %s\n", pluginToolConfig["trcservicename"].(string))

	} else if *winservicestartPtr {
		fmt.Printf("Starting service %s\n", pluginToolConfig["trcservicename"].(string))
		//		cmd := exec.Command("sc", "start", pluginToolConfig["trcservicename"].(string))
		cmd := exec.Command("net", "start", pluginToolConfig["trcservicename"].(string))
		err := cmd.Run()
		if err != nil && strings.Contains(err.Error(), "2185") {
			// Only break if service isn't defined...
			fmt.Println(err)
			return err
		}
		fmt.Printf("Service started: %s\n", pluginToolConfig["trcservicename"].(string))
	} else if *codebundledeployPtr {
		if pluginopts.BuildOptions.IsPluginHardwired() {
			var deployRoot string
			if deploySubPath, ok := pluginToolConfig["trcdeploysubpath"]; ok {
				deployRoot = filepath.Join(pluginToolConfig["trcdeployroot"].(string), deploySubPath.(string))
			} else {
				deployRoot = pluginToolConfig["trcdeployroot"].(string)
			}
			if _, err = os.Stat(deployRoot); err != nil && !os.IsPermission(err) {
				err = os.MkdirAll(deployRoot, 0700)
				if err != nil && !os.IsPermission(err) {
					fmt.Println(err.Error())
					fmt.Println("Could not prepare needed directory for deployment.")
				}
			}

			fmt.Printf("Skipping codebundledeploy for hardwired: %s\n", pluginToolConfig["trcplugin"].(string))
			return nil
		}
		if pluginToolConfig["trcsha256"] == nil || len(pluginToolConfig["trcsha256"].(string)) == 0 {
			if trcshDriverConfigBase.DriverConfig.DeploymentConfig != nil && trcshDriverConfigBase.DriverConfig.DeploymentConfig["trcsha256"] != nil && len(trcshDriverConfigBase.DriverConfig.DeploymentConfig["trcsha256"].(string)) > 0 {
				pluginToolConfig["trcsha256"] = trcshDriverConfigBase.DriverConfig.DeploymentConfig["trcsha256"]
			}
		}

		if pluginToolConfig["trcsha256"] != nil && len(pluginToolConfig["trcsha256"].(string)) > 0 {
			err := repository.GetImageAndShaFromDownload(trcshDriverConfigBase.DriverConfig, pluginToolConfig)
			if err != nil {
				fmt.Println("Image download failure.")
				if trcshDriverConfigBase.FeatherCtx != nil {
					trcshDriverConfigBase.FeatherCtx.Log.Printf("Image download failure: %s", err.Error())
				} else {
					fmt.Println(err.Error())
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

			//check if there is a place holder, if there is replace it
			if strings.Contains(deployRoot, "{{.trcpathparam}}") {
				if pathParam, ok := pluginToolConfig["trcpathparam"].(string); ok && pathParam != "" {
					r, _ := regexp.Compile("^[a-zA-Z0-9_]*$")
					if !r.MatchString(pathParam) {
						fmt.Println("trcpathparam can only contain alphanumberic characters or underscores")
						return errors.New("trcpathparam can only contain alphanumberic characters or underscores")
					}
					deployRoot = strings.Replace(deployRoot, "{{.trcpathparam}}", pathParam, -1)
				} else {
					return errors.New("Unable to replace path placeholder with pathParam.")
				}
			}
			deployPath = filepath.Join(deployRoot, pluginToolConfig["trccodebundle"].(string))

			if !pluginopts.BuildOptions.IsPluginHardwired() {
				fmt.Printf("Deploying image to: %s\n", deployPath)
				if _, err = os.Stat(deployRoot); err != nil && !os.IsPermission(err) {
					err = os.MkdirAll(deployRoot, 0700)
					if err != nil && !os.IsPermission(err) {
						fmt.Println(err.Error())
						fmt.Println("Could not prepare needed directory for deployment.")
						return err
					}
				}
				if rif, ok := pluginToolConfig["rawImageFile"]; ok {
					err = os.WriteFile(deployPath, rif.([]byte), 0700)
					if err != nil {
						fmt.Println(err.Error())
						fmt.Println("Image write failure.")
						return err
					}
				}

				if expandTarget, ok := pluginToolConfig["trcexpandtarget"].(string); ok && expandTarget == "true" {
					// TODO: provide archival of existing directory.
					if ok, errList := trcvutils.UncompressZipFile(deployPath); !ok {
						fmt.Printf("Uncompressing zip file in place failed. %v\n", errList)
						return errList[0]
					} else {
						os.Remove(deployPath)
					}
				} else {
					if strings.HasSuffix(deployPath, ".war") {
						explodedWarPath := strings.TrimSuffix(deployPath, ".war")
						fmt.Printf("Checking exploded war path: %s\n", explodedWarPath)
						if _, err := os.Stat(explodedWarPath); err == nil {
							if depRoot, ok := pluginToolConfig["trcdeployroot"]; ok {
								deployRoot = depRoot.(string)
							}
							archiveDirPath := filepath.Join(deployRoot, "archive")
							fmt.Printf("Verifying archive directory: %s\n", archiveDirPath)
							err := os.MkdirAll(archiveDirPath, 0700)
							if err == nil {
								currentTime := time.Now()
								formattedTime := fmt.Sprintf("%d-%02d-%02d_%02d-%02d-%02d", currentTime.Year(), currentTime.Month(), currentTime.Day(), currentTime.Hour(), currentTime.Minute(), currentTime.Second())
								archiveRoot := filepath.Join(pluginToolConfig["trcdeployroot"].(string), "archive", formattedTime)
								fmt.Printf("Verifying archive backup directory: %s\n", archiveRoot)
								err := os.MkdirAll(archiveRoot, 0700)
								if err == nil {
									archivePath := filepath.Join(archiveRoot, pluginToolConfig["trccodebundle"].(string))
									archivePath = strings.TrimSuffix(archivePath, ".war")
									fmt.Printf("Archiving: %s to %s\n", explodedWarPath, archivePath)
									os.Rename(explodedWarPath, archivePath)
								}
							}
						}
					}
				}
				fmt.Printf("Image deployed to: %s\n", deployPath)
			}
		} else {
			errMessage := fmt.Sprintf("image not certified.  cannot deploy image for %s", pluginToolConfig["trcplugin"])
			if trcshDriverConfigBase.FeatherCtx != nil {
				fmt.Printf("%s\n", errMessage)
				trcshDriverConfigBase.FeatherCtx.Log.Print(errMessage)
			} else {
				fmt.Printf("%s\n", errMessage)
			}
			return errors.New(errMessage)
		}
		if ptcsha256, ok := pluginToolConfig["trcsha256"]; ok && kernelopts.BuildOptions.IsKernel() {
			trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Println("Starting verification of plugin module.")
			h := sha256.New()
			pathToSO := hive.LoadPluginPath(trcshDriverConfigBase.DriverConfig, pluginToolConfig)
			f, err := os.OpenFile(pathToSO, os.O_RDONLY, 0600)
			if err != nil {
				return err
			}
			defer f.Close()
			err = memprotectopts.SetChattr(f)
			if err != nil {
				fmt.Println(err)
				return err
			}
			if _, err := io.Copy(h, f); err != nil {
				fmt.Printf("Unable to copy file: %s\n", err)
				trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Unable to copy file: %s\n", err)
				return err
			}
			sha := hex.EncodeToString(h.Sum(nil))
			if pluginopts.BuildOptions.IsPluginHardwired() || (ptcsha256.(string) == sha) {
				trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Println("Verified plugin module sha.")
				err = memprotectopts.UnsetChattr(f)
				if err != nil {
					return err
				}
				if pluginHandler != nil {
					if pluginHandler.State == 2 && sha == pluginHandler.Signature { //make sure this won't break...not set yet
						trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Tried to redeploy same failed plugin: %s\n", *pluginNamePtr)
						// do we want to remove from available services???
					} else {
						if s, ok := pluginToolConfig["trctype"].(string); ok && s == "trcshpluginservice" {
							pluginHandler.LoadPluginMod(trcshDriverConfigBase.DriverConfig, pathToSO)
						}
						pluginHandler.Signature = sha
					}
				} else {
					fmt.Printf("Handler not initialized for plugin to start: %s\n", *pluginNamePtr)
					trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Handler not initialized for plugin to start: %s\n", *pluginNamePtr)
				}
			}
		}
	} else if *certifyImagePtr {
		//Certify Image
		carrierCertify := false
		// Certification always operates on env basis.
		mod.EnvBasis = driverConfig.CoreConfig.EnvBasis
		mod.Env = mod.EnvBasis
		if ptc, ok := pluginToolConfig["trcplugin"].(string); ok && strings.Contains(ptc, "carrier") {
			fmt.Println("Skipping checking for existing image due to carrier deployment.")
			carrierCertify = true
		} else if !certifyInit {
			// Already certified...
			fmt.Println("Checking for existing image.")
			err := repository.GetImageAndShaFromDownload(trcshDriverConfigBase.DriverConfig, pluginToolConfig)
			if _, ok := pluginToolConfig["imagesha256"].(string); err != nil || !ok {
				fmt.Println("Invalid or nonexistent image on download.")
				if err != nil {
					fmt.Println(err.Error())
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
				fmt.Println("Valid image found.")
			}
			//SHA MATCHES
			fmt.Printf("Connecting to vault @ %s\n", *trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.VaultAddressPtr)
			trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Println("TrcCarrierUpdate getting plugin settings for env: " + mod.Env)
			// The following confirms that this version of carrier has been certified to run...
			// It will bail if it hasn't.
			if _, pluginPathOk := pluginToolConfig["pluginpath"].(string); !pluginPathOk { //If region is set
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
					fmt.Println("Couldn't create properties for regioned certify:" + err.Error())
					return err
				}

				writeMap, replacedFields := properties.GetPluginData(*regionPtr, "Certify", "config", trcshDriverConfigBase.DriverConfig.CoreConfig.Log)

				pluginTarget := pluginToolConfig["trcplugin"].(string)
				if strings.HasPrefix(*pluginNamePtr, pluginTarget) {
					pluginTarget = *pluginNamePtr
				}
				writeErr := properties.WritePluginData(certify.WriteMapUpdate(writeMap, pluginToolConfig, *defineServicePtr, *pluginTypePtr, *pathParamPtr), replacedFields, mod, trcshDriverConfigBase.DriverConfig.CoreConfig.Log, *regionPtr, pluginTarget)
				if writeErr != nil {
					fmt.Println(writeErr)
					return err
				}
				fmt.Println("Image certified.")
			} else { //Non region certify
				writeMap, readErr := mod.ReadData(pluginToolConfig["pluginpath"].(string))
				if readErr != nil {
					if trcshDriverConfig.DriverConfig.CoreConfig.TokenCache != nil {
						mod.EmptyCache()
						trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.Clear()
					}
					fmt.Println(readErr)
					return err
				}

				_, err = mod.Write(pluginToolConfig["pluginpath"].(string), certify.WriteMapUpdate(writeMap, pluginToolConfig, *defineServicePtr, *pluginTypePtr, *pathParamPtr), trcshDriverConfigBase.DriverConfig.CoreConfig.Log)
				if err != nil {
					fmt.Println(err)
					return err
				}
				fmt.Println("Image certified in vault and is ready for release.")
			}
		} else {
			fmt.Println("Invalid or nonexistent image.")
			return err
		}
	} else if *agentdeployPtr {
		if trcshConfig.FeatherCtlCb != nil {
			err := trcshConfig.FeatherCtlCb(trcshConfig.FeatherCtx, *pluginNamePtr)
			if err != nil {
				fmt.Printf("Incorrect installation: %s\n", err.Error())
				return err
			}
		} else {
			fmt.Println("Incorrect trcplgtool utilization")
			return err
		}
	} else if *pluginservicestartPtr && kernelopts.BuildOptions.IsKernel() {
		if pluginHandler != nil && pluginHandler.State != 2 && kernelPluginHandler != nil {
			if kernelPluginHandler.ConfigContext == nil || kernelPluginHandler.ConfigContext.ChatReceiverChan == nil {
				fmt.Printf("Unable to access chat channel configuration data for %s\n", *pluginNamePtr)
				driverConfig.CoreConfig.Log.Printf("Unable to access chat channel configuration data for %s\n", *pluginNamePtr)
			} else {
				trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Starting plugin service: %s\n", *pluginNamePtr)
				pluginHandler.PluginserviceStart(trcshDriverConfigBase.DriverConfig, pluginToolConfig, kernelPluginHandler.ConfigContext.ChatReceiverChan)
			}
		} else {
			fmt.Printf("Handler not initialized for plugin to start: %s\n", *pluginNamePtr)
			trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Handler not initialized for plugin to start: %s\n", *pluginNamePtr)
		}
	} else if *pluginservicestopPtr && kernelopts.BuildOptions.IsKernel() {
		if pluginHandler != nil && pluginHandler.State != 2 {
			pluginHandler.PluginserviceStop(trcshDriverConfigBase.DriverConfig)
		} else {
			fmt.Printf("Handler not initialized for plugin to shutdown: %s\n", *pluginNamePtr)
			trcshDriverConfigBase.DriverConfig.CoreConfig.Log.Printf("Handler not initialized for plugin to shutdown: %s\n", *pluginNamePtr)
		}
	}
	//Checks if image has been copied & deployed
	if *checkDeployedPtr {
		if (pluginToolConfig["copied"] != nil && pluginToolConfig["copied"].(bool)) &&
			(pluginToolConfig["deployed"] != nil && pluginToolConfig["deployed"].(bool)) &&
			(pluginToolConfig["trcsha256"] != nil && pluginToolConfig["trcsha256"].(string) == *sha256Ptr) { //Compare vault sha with provided sha
			fmt.Println("Plugin has been copied, deployed & certified.")
			return nil
		}

		err := repository.GetImageAndShaFromDownload(trcshDriverConfigBase.DriverConfig, pluginToolConfig)
		if err != nil {
			fmt.Println(err.Error())
			return err
		}

		if *sha256Ptr == pluginToolConfig["imagesha256"].(string) { //Compare repo image sha with provided sha
			fmt.Println("Latest plugin image sha matches provided plugin sha.  It has been certified.")
		} else {
			fmt.Println("Provided plugin sha is not deployable.")
			return errors.New("provided plugin sha is not deployable")
		}

		fmt.Println("Plugin has not been copied or deployed.")
		return nil
	}

	if *checkCopiedPtr {
		if pluginToolConfig["copied"].(bool) && pluginToolConfig["trcsha256"].(string) == *sha256Ptr { //Compare vault sha with provided sha
			fmt.Println("Plugin has been copied & certified.")
			return nil
		}

		err := repository.GetImageAndShaFromDownload(trcshDriverConfigBase.DriverConfig, pluginToolConfig)
		if err != nil {
			fmt.Println(err.Error())
			return err
		}

		if *sha256Ptr == pluginToolConfig["imagesha256"].(string) { //Compare repo image sha with provided sha
			fmt.Println("Latest plugin image sha matches provided plugin sha.  It has been certified.")
		} else {
			fmt.Println("Provided plugin sha is not certified.")
			return errors.New("provided plugin sha is not certified")
		}

		fmt.Println("Plugin has not been copied or deployed.")
		return nil
	}
	return nil
}

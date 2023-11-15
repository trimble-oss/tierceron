package trcplgtoolbase

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	trcvutils "github.com/trimble-oss/tierceron/trcvault/util"
	"github.com/trimble-oss/tierceron/trcvault/util/repository"
	eUtils "github.com/trimble-oss/tierceron/utils"
)

func CommonMain(envPtr *string,
	addrPtr *string,
	tokenPtr *string,
	envCtxPtr *string,
	secretIDPtr *string,
	appRoleIDPtr *string,
	tokenNamePtr *string,
	regionPtr *string,
	flagset *flag.FlagSet,
	argLines []string,
	c *eUtils.DriverConfig) error {

	// Main functions are as follows:
	if flagset == nil {
		flagset = flag.NewFlagSet(argLines[0], flag.ContinueOnError)
		// set and ignore..
		flagset.String("env", "dev", "Environment to configure")
		flagset.String("addr", "", "API endpoint for the vault")
		flagset.String("token", "", "Vault access token")
		flagset.String("region", "", "Region to be processed") //If this is blank -> use context otherwise override context.
		flagset.Usage = flag.Usage
	}
	defineServicePtr := flagset.Bool("defineService", false, "Service is defined.")
	certifyImagePtr := flagset.Bool("certify", false, "Used to certifies vault plugin.")
	// These functions only valid for pluginType trcshservice
	winservicestopPtr := flagset.Bool("winservicestop", false, "To stop a windows service for a particular plugin.")
	winservicestartPtr := flagset.Bool("winservicestart", false, "To start a windows service for a particular plugin.")
	codebundledeployPtr := flagset.Bool("codebundledeploy", false, "To deploy a code bundle.")
	agentdeployPtr := flagset.Bool("agentdeploy", false, "To initiate deployment on agent.")
	projectservicePtr := flagset.String("projectservice", "", "Provide template root path in form project/service")

	// Common flags...
	startDirPtr := flagset.String("startDir", coreopts.GetFolderPrefix(nil)+"_templates", "Template directory")
	insecurePtr := flagset.Bool("insecure", false, "By default, every ssl connection is secure.  Allows to continue with server connections considered insecure.")
	logFilePtr := flagset.String("log", "./"+coreopts.GetFolderPrefix(nil)+"plgtool.log", "Output path for log files")

	// defineService flags...
	deployrootPtr := flagset.String("deployroot", "", "Optional path for deploying services to.")
	serviceNamePtr := flagset.String("serviceName", "", "Optional name of service to use in managing service.")
	codeBundlePtr := flagset.String("codeBundle", "", "Code bundle to deploy.")

	// Common plugin flags...
	pluginNamePtr := flagset.String("pluginName", "", "Used to certify vault plugin")
	pluginTypePtr := flagset.String("pluginType", "vault", "Used to indicate type of plugin.  Default is vault.")

	// Certify flags...
	sha256Ptr := flagset.String("sha256", "", "Used to certify vault plugin") //This has to match the image that is pulled -> then we write the vault.
	checkDeployedPtr := flagset.Bool("checkDeployed", false, "Used to check if plugin has been copied, deployed, & certified")
	checkCopiedPtr := flagset.Bool("checkCopied", false, "Used to check if plugin has been copied & certified")

	certifyInit := false

	if c == nil || !c.IsShellSubProcess {
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
	} else {
		err := flagset.Parse(argLines)
		if err != nil {
			return err
		}
		err = flagset.Parse(argLines[1:])
		if err != nil {
			return err
		}
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

	if strings.Contains(*pluginNamePtr, ".") {
		fmt.Println("-pluginName cannot contain reserved character '.'")
		return errors.New("-pluginName cannot contain reserved character '.'")
	}

	if *agentdeployPtr || *winservicestopPtr || *winservicestartPtr || *codebundledeployPtr {
		*pluginTypePtr = "trcshservice"
	}

	switch *pluginTypePtr {
	case "vault": // A vault plugin
		if c != nil {
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
		if c != nil {
			// TODO: do we want to support Deployment certifications in the pipeline at some point?
			// If so this is a config check to remove.
			fmt.Printf("Plugin type %s not supported in trcsh.\n", *pluginTypePtr)
			return fmt.Errorf("plugin type %s not supported in trcsh", *pluginTypePtr)
		}
		if *codebundledeployPtr {
			fmt.Printf("codebundledeploy not supported for plugin type %s in trcsh\n", *pluginTypePtr)
			return fmt.Errorf("codebundledeploy not supported for plugin type %s in trcsh", *pluginTypePtr)
		}
	case "trcshservice": // A trcshservice managed microservice
	default:
		if !*agentdeployPtr {
			fmt.Println("Unsupported plugin type: " + *pluginTypePtr)
			return fmt.Errorf("unsupported plugin type: %s", *pluginTypePtr)
		} else {
			fmt.Printf("\nBeginning agent deployment for %s..\n", *pluginNamePtr)
		}
	}

	if *pluginTypePtr != "vault" {
		*regionPtr = ""
	}

	var appRoleConfigPtr *string
	var configBase *eUtils.DriverConfig
	var logger *log.Logger
	if c != nil {
		configBase = c
		logger = c.Log
		configBase.SubSectionValue = *pluginNamePtr
		appRoleConfigPtr = &(configBase.AppRoleConfig)
		*insecurePtr = configBase.Insecure
	} else {
		if *agentdeployPtr {
			fmt.Println("Unsupported agentdeploy outside trcsh")
			return errors.New("unsupported agentdeploy outside trcsh")
		}

		// If logging production directory does not exist and is selected log to local directory
		if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/"+coreopts.GetFolderPrefix(nil)+"plgtool.log" {
			*logFilePtr = "./" + coreopts.GetFolderPrefix(nil) + "plgtool.log"
		}
		f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		logger = log.New(f, "[INIT]", log.LstdFlags)

		configBase = &eUtils.DriverConfig{Insecure: *insecurePtr, Log: logger, ExitOnFailure: true, StartDir: []string{*startDirPtr}, SubSectionValue: *pluginNamePtr}
		appRoleConfigPtr = new(string)
		if err != nil {
			return err
		}
	}

	//
	if tokenNamePtr == nil || *tokenNamePtr == "" {
		autoErr := eUtils.AutoAuth(configBase, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, envCtxPtr, *appRoleConfigPtr, false)
		if autoErr != nil {
			eUtils.LogErrorMessage(configBase, "Auth failure: "+autoErr.Error(), false)
			return errors.New("auth failure")
		}
	}

	regions := []string{}

	pluginConfig := map[string]interface{}{}
	pluginConfig = buildopts.ProcessPluginEnvConfig(pluginConfig) //contains logNamespace for InitVaultMod
	if pluginConfig == nil {
		fmt.Println("Error: Could not find plugin config")
		return errors.New("could not find plugin config")
	}
	pluginConfig["env"] = *envPtr
	pluginConfig["vaddress"] = *addrPtr
	if tokenPtr != nil {
		pluginConfig["token"] = *tokenPtr
	}
	pluginConfig["ExitOnFailure"] = true
	if *regionPtr != "" {
		pluginConfig["regions"] = []string{*regionPtr}
	}
	config, mod, vault, err := eUtils.InitVaultModForPlugin(pluginConfig, logger)
	config.FeatherCtlCb = configBase.FeatherCtlCb
	if err != nil {
		logger.Println("Error: " + err.Error() + " - 1")
		logger.Println("Failed to init mod for deploy update")
		return err
	}
	config.StartDir = []string{*startDirPtr}
	config.SubSectionValue = *pluginNamePtr
	mod.Env = *envPtr
	eUtils.CheckError(config, err, true)

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
		configBase.Regions = regions
	}

	// Get existing configs if they exist...
	pluginToolConfig, plcErr := trcvutils.GetPluginToolConfig(configBase, mod, coreopts.ProcessDeployPluginEnvConfig(map[string]interface{}{}), *defineServicePtr)
	if plcErr != nil {
		fmt.Println(plcErr.Error())
		return plcErr
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
			}
		}
	}

	pluginToolConfig["trcsha256"] = *sha256Ptr
	pluginToolConfig["pluginNamePtr"] = *pluginNamePtr
	pluginToolConfig["deployrootPtr"] = *deployrootPtr
	pluginToolConfig["serviceNamePtr"] = *serviceNamePtr
	pluginToolConfig["projectservicePtr"] = *projectservicePtr
	pluginToolConfig["codeBundlePtr"] = *codeBundlePtr

	if _, ok := pluginToolConfig["trcplugin"].(string); !ok {
		pluginToolConfig["trcplugin"] = pluginToolConfig["pluginNamePtr"].(string)
		if *certifyImagePtr {
			certifyInit = true
		}

		if *defineServicePtr &&
			!*winservicestopPtr &&
			!*winservicestartPtr &&
			!*codebundledeployPtr &&
			!*certifyImagePtr {

			if c != nil {
				fmt.Println("Service definition not supported in trcsh.")
				os.Exit(-1)
			}
			if _, ok := pluginToolConfig["deployrootPtr"].(string); ok {
				pluginToolConfig["trcdeployroot"] = pluginToolConfig["deployrootPtr"].(string)
			}

			if _, ok := pluginToolConfig["serviceNamePtr"].(string); ok {
				pluginToolConfig["trcservicename"] = pluginToolConfig["serviceNamePtr"].(string)
			}
			if _, ok := pluginToolConfig["codeBundlePtr"].(string); ok {
				pluginToolConfig["trccodebundle"] = pluginToolConfig["codeBundlePtr"].(string)
			}
			if _, ok := pluginToolConfig["projectServicePtr"].(string); ok {
				pluginToolConfig["trcprojectservice"] = pluginToolConfig["projectServicePtr"].(string)
			}
		}
	}

	//Define Service Image
	if *defineServicePtr {
		fmt.Printf("Connecting to vault @ %s\n", *addrPtr)
		writeMap := make(map[string]interface{})
		writeMap["trcplugin"] = *pluginNamePtr
		writeMap["trctype"] = *pluginTypePtr
		writeMap["trcprojectservice"] = *projectservicePtr

		if _, ok := pluginToolConfig["trcdeployroot"]; ok {
			writeMap["trcdeployroot"] = pluginToolConfig["trcdeployroot"]
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
		_, err = mod.Write(pluginToolConfig["pluginpath"].(string), writeMap, configBase.Log)
		if err != nil {
			fmt.Println(err)
			return err
		}
		fmt.Println("Deployment definition applied to vault and is ready for deployments.")
	} else if *winservicestopPtr {
		fmt.Printf("Stopping service %s\n", pluginToolConfig["trcservicename"].(string))
		cmd := exec.Command("sc", "stop", pluginToolConfig["trcservicename"].(string))
		err := cmd.Run()
		if err != nil {
			fmt.Println(err)
			return err
		}
		fmt.Printf("Service stopped: %s\n", pluginToolConfig["trcservicename"].(string))

	} else if *winservicestartPtr {
		fmt.Printf("Starting service %s\n", pluginToolConfig["trcservicename"].(string))
		cmd := exec.Command("sc", "start", pluginToolConfig["trcservicename"].(string))
		err := cmd.Run()
		if err != nil {
			fmt.Println(err)
			return err
		}
		fmt.Printf("Service started: %s\n", pluginToolConfig["trcservicename"].(string))
	} else if *codebundledeployPtr {
		if pluginToolConfig["trcsha256"] == nil || len(pluginToolConfig["trcsha256"].(string)) == 0 {
			if configBase.DeploymentConfig != nil && configBase.DeploymentConfig["trcsha256"] != nil && len(configBase.DeploymentConfig["trcsha256"].(string)) > 0 {
				pluginToolConfig["trcsha256"] = configBase.DeploymentConfig["trcsha256"]
			}
		}
		if pluginToolConfig["trcsha256"] != nil && len(pluginToolConfig["trcsha256"].(string)) > 0 {
			err := repository.GetImageAndShaFromDownload(configBase, pluginToolConfig)
			if err != nil {
				fmt.Println("Image download failure.")
				fmt.Println(err.Error())
				return err
			}
		}

		if pluginToolConfig["trcsha256"] != nil &&
			pluginToolConfig["imagesha256"] != nil &&
			pluginToolConfig["trcsha256"].(string) == pluginToolConfig["imagesha256"].(string) {
			// Write the image to the destination...
			deployPath := fmt.Sprintf("%s\\%s", pluginToolConfig["trcdeployroot"].(string), pluginToolConfig["trccodebundle"].(string))
			fmt.Printf("Deploying image to: %s\n", deployPath)

			err = os.WriteFile(deployPath, pluginToolConfig["rawImageFile"].([]byte), 0644)
			if err != nil {
				fmt.Println(err.Error())
				fmt.Println("Image write failure.")
				return err
			}
			fmt.Println("Image deployed.")
		} else {
			fmt.Printf("Image not certified.  Cannot deploy image for %s\n", pluginToolConfig["trcplugin"])
		}
	} else if *certifyImagePtr {
		//Certify Image
		if !certifyInit {
			// Already certified...
			fmt.Println("Checking for existing image.")
			err := repository.GetImageAndShaFromDownload(configBase, pluginToolConfig)
			if _, ok := pluginToolConfig["imagesha256"].(string); err != nil || !ok {
				fmt.Println("Invalid or nonexistent image.")
				if err != nil {
					fmt.Println(err.Error())
				}
				return err
			}
		}

		if certifyInit ||
			pluginToolConfig["trcsha256"].(string) == pluginToolConfig["imagesha256"].(string) { // Comparing generated sha from image to sha from flag
			// ||
			//(pluginToolConfig["imagesha256"].(string) != "" && pluginToolConfig["trctype"].(string) == "trcshservice") {
			fmt.Println("Valid image found.")
			//SHA MATCHES
			fmt.Printf("Connecting to vault @ %s\n", *addrPtr)
			logger.Println("TrcCarrierUpdate getting plugin settings for env: " + mod.Env)
			// The following confirms that this version of carrier has been certified to run...
			// It will bail if it hasn't.
			if _, pluginPathOk := pluginToolConfig["pluginpath"].(string); !pluginPathOk { //If region is set
				mod.SectionName = "trcplugin"
				mod.SectionKey = "/Index/"
				mod.SubSectionValue = pluginToolConfig["trcplugin"].(string)

				properties, err := trcvutils.NewProperties(config, vault, mod, mod.Env, "TrcVault", "Certify")
				if err != nil {
					fmt.Println("Couldn't create properties for regioned certify:" + err.Error())
					return err
				}

				writeMap, replacedFields := properties.GetPluginData(*regionPtr, "Certify", "config", logger)

				writeErr := properties.WritePluginData(WriteMapUpdate(writeMap, pluginToolConfig, *defineServicePtr, *pluginTypePtr), replacedFields, mod, config.Log, *regionPtr, pluginToolConfig["trcplugin"].(string))
				if writeErr != nil {
					fmt.Println(writeErr)
					return err
				}
			} else { //Non region certify
				writeMap, readErr := mod.ReadData(pluginToolConfig["pluginpath"].(string))
				if readErr != nil {
					fmt.Println(readErr)
					return err
				}

				_, err = mod.Write(pluginToolConfig["pluginpath"].(string), WriteMapUpdate(writeMap, pluginToolConfig, *defineServicePtr, *pluginTypePtr), configBase.Log)
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
		if config.FeatherCtlCb != nil {
			err := config.FeatherCtlCb(*pluginNamePtr)
			if err != nil {
				fmt.Println("Incorrect installation")
				return err
			}
		} else {
			fmt.Println("Incorrect trcplgtool utilization")
			return err
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

		err := repository.GetImageAndShaFromDownload(configBase, pluginToolConfig)
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

		err := repository.GetImageAndShaFromDownload(configBase, pluginToolConfig)
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

func WriteMapUpdate(writeMap map[string]interface{}, pluginToolConfig map[string]interface{}, defineServicePtr bool, pluginTypePtr string) map[string]interface{} {
	if pluginTypePtr != "trcshservice" {
		writeMap["trcplugin"] = pluginToolConfig["trcplugin"].(string)
		writeMap["trctype"] = pluginTypePtr
		if pluginToolConfig["instances"] == nil {
			pluginToolConfig["instances"] = "0"
		}
		writeMap["instances"] = pluginToolConfig["instances"].(string)
	}
	if defineServicePtr {
		writeMap["trccodebundle"] = pluginToolConfig["trccodebundle"].(string)
		writeMap["trcservicename"] = pluginToolConfig["trcservicename"].(string)
		writeMap["trcprojectservice"] = pluginToolConfig["trcprojectservice"].(string)
		writeMap["trcdeployroot"] = pluginToolConfig["trcdeployroot"].(string)
	}
	if _, imgShaOk := pluginToolConfig["imagesha256"].(string); imgShaOk {
		writeMap["trcsha256"] = pluginToolConfig["imagesha256"].(string) // Pull image sha from registry...
	} else {
		writeMap["trcsha256"] = pluginToolConfig["trcsha256"].(string) // Pull image sha from registry...
	}
	writeMap["copied"] = false
	writeMap["deployed"] = false
	return writeMap
}

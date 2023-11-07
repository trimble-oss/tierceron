package trcplgtoolbase

import (
	"bufio"
	"crypto/sha256"
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
	regionPtr *string,
	c *eUtils.DriverConfig) {

	// Main functions are as follows:
	defineServicePtr := flag.Bool("defineService", false, "Service is defined.")
	certifyImagePtr := flag.Bool("certify", false, "Used to certifies vault plugin.")
	// These functions only valid for pluginType trcshservice
	winservicestopPtr := flag.Bool("winservicestop", false, "To stop a windows service for a particular plugin.")
	winservicestartPtr := flag.Bool("winservicestart", false, "To start a windows service for a particular plugin.")
	codebundledeployPtr := flag.Bool("codebundledeploy", false, "To deploy a code bundle.")
	agentdeployPtr := flag.Bool("agentdeploy", false, "To initiate deployment on agent.")

	// Common flags...
	startDirPtr := flag.String("startDir", coreopts.GetFolderPrefix(nil)+"_templates", "Template directory")
	insecurePtr := flag.Bool("insecure", false, "By default, every ssl connection is secure.  Allows to continue with server connections considered insecure.")
	logFilePtr := flag.String("log", "./"+coreopts.GetFolderPrefix(nil)+"plgtool.log", "Output path for log files")

	// defineService flags...
	deployrootPtr := flag.String("deployroot", "", "Optional path for deploying services to.")
	serviceNamePtr := flag.String("serviceName", "", "Optional name of service to use in managing service.")
	codeBundlePtr := flag.String("codeBundle", "", "Code bundle to deploy.")

	// Common plugin flags...
	pluginNamePtr := flag.String("pluginName", "", "Used to certify vault plugin")
	pluginTypePtr := flag.String("pluginType", "vault", "Used to indicate type of plugin.  Default is vault.")

	// Certify flags...
	sha256Ptr := flag.String("sha256", "", "Used to certify vault plugin") //This has to match the image that is pulled -> then we write the vault.
	checkDeployedPtr := flag.Bool("checkDeployed", false, "Used to check if plugin has been copied, deployed, & certified")
	checkCopiedPtr := flag.Bool("checkCopied", false, "Used to check if plugin has been copied & certified")

	certifyInit := false

	if c == nil || !c.IsShellSubProcess {
		args := os.Args[1:]
		for i := 0; i < len(args); i++ {
			s := args[i]
			if s[0] != '-' {
				fmt.Println("Wrong flag syntax: ", s)
				os.Exit(1)
			}
		}
		flag.Parse()

		// Prints usage if no flags are specified
		if flag.NFlag() == 0 {
			flag.Usage()
			os.Exit(1)
		}
	} else {
		flag.CommandLine.Parse(os.Args)
		flag.Parse()
	}

	if *certifyImagePtr && (len(*pluginNamePtr) == 0 || len(*sha256Ptr) == 0) {
		fmt.Println("Must use -pluginName && -sha256 flags to use -certify flag")
		os.Exit(1)
	}

	if *checkDeployedPtr && (len(*pluginNamePtr) == 0) {
		fmt.Println("Must use -pluginName flag to use -checkDeployed flag")
		os.Exit(1)
	}

	if *defineServicePtr && (len(*pluginNamePtr) == 0) {
		fmt.Println("Must use -pluginName flag to use -defineService flag")
		os.Exit(1)
	}

	if strings.Contains(*pluginNamePtr, ".") {
		fmt.Println("-pluginName cannot contain reserved character '.'")
		os.Exit(1)
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
			os.Exit(-1)
		}
	case "agent": // A deployment agent tool.
		if c != nil {
			// TODO: do we want to support Deployment certifications in the pipeline at some point?
			// If so this is a config check to remove.
			fmt.Printf("Plugin type %s not supported in trcsh.\n", *pluginTypePtr)
			os.Exit(-1)
		}
	case "trcshservice": // A trcshservice managed microservice
	default:
		if !*agentdeployPtr {
			fmt.Println("Unsupported plugin type: " + *pluginTypePtr)
			os.Exit(1)
		}
	}

	if *pluginTypePtr != "vault" {
		*regionPtr = ""
	}

	var configBase *eUtils.DriverConfig
	var logger *log.Logger
	if c != nil {
		configBase = c
		logger = c.Log
		configBase.SubSectionValue = *pluginNamePtr
		*insecurePtr = configBase.Insecure
	} else {
		if *agentdeployPtr {
			fmt.Println("Unsupported agentdeploy outside trcsh")
			os.Exit(1)
		}

		// If logging production directory does not exist and is selected log to local directory
		if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/"+coreopts.GetFolderPrefix(nil)+"plgtool.log" {
			*logFilePtr = "./" + coreopts.GetFolderPrefix(nil) + "plgtool.log"
		}
		f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		logger = log.New(f, "[INIT]", log.LstdFlags)

		configBase = &eUtils.DriverConfig{Insecure: *insecurePtr, Log: logger, ExitOnFailure: true, StartDir: []string{*startDirPtr}, SubSectionValue: *pluginNamePtr}
		eUtils.CheckError(configBase, err, true)
	}

	regions := []string{}

	pluginConfig := map[string]interface{}{}
	pluginConfig = buildopts.ProcessPluginEnvConfig(pluginConfig) //contains logNamespace for InitVaultMod
	if pluginConfig == nil {
		fmt.Println("Error: Could not find plugin config")
		os.Exit(1)
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
		os.Exit(1)
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
				os.Exit(1)
			}
		}
		configBase.Regions = regions
	}

	// Get existing configs if they exist...
	pluginToolConfig, plcErr := trcvutils.GetPluginToolConfig(configBase, mod, coreopts.ProcessDeployPluginEnvConfig(map[string]interface{}{}), *defineServicePtr)
	if plcErr != nil {
		fmt.Println(plcErr.Error())
		os.Exit(1)
	}

	if len(*sha256Ptr) > 0 {
		fileInfo, statErr := os.Stat(*sha256Ptr)
		if statErr == nil {
			if fileInfo.Mode().IsRegular() {
				file, fileOpenErr := os.Open(*sha256Ptr)
				if fileOpenErr != nil {
					fmt.Println(fileOpenErr)
					return
				}

				// Close the file when we're done.
				defer file.Close()

				// Create a reader to the file.
				reader := bufio.NewReader(file)

				pluginImage, imageErr := io.ReadAll(reader)
				if imageErr != nil {
					fmt.Println("Failed to read image:" + imageErr.Error())
					return
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
		}
	}

	//Define Service Image
	if *defineServicePtr {
		fmt.Printf("Connecting to vault @ %s\n", *addrPtr)
		writeMap := make(map[string]interface{})
		writeMap["trcplugin"] = *pluginNamePtr
		writeMap["trctype"] = *pluginTypePtr

		if _, ok := pluginToolConfig["trcdeployroot"]; ok {
			writeMap["trcdeployroot"] = pluginToolConfig["trcdeployroot"]
		}
		if _, ok := pluginToolConfig["trcservicename"]; ok {
			writeMap["trcservicename"] = pluginToolConfig["trcservicename"]
		}
		if _, ok := pluginToolConfig["trccodebundle"]; ok {
			writeMap["trccodebundle"] = pluginToolConfig["trccodebundle"]
		}
		_, err = mod.Write(pluginToolConfig["pluginpath"].(string), writeMap, configBase.Log)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("Deployment definition applied to vault and is ready for deployments.")
	} else if *winservicestopPtr {
		fmt.Printf("Stopping service %s\n", pluginToolConfig["trcservicename"].(string))
		cmd := exec.Command("sc", "stop", pluginToolConfig["trcservicename"].(string))
		err := cmd.Run()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("Service stopped: %s\n", pluginToolConfig["trcservicename"].(string))

	} else if *winservicestartPtr {
		fmt.Printf("Starting service %s\n", pluginToolConfig["trcservicename"].(string))
		cmd := exec.Command("sc", "start", pluginToolConfig["trcservicename"].(string))
		err := cmd.Run()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("Service started: %s\n", pluginToolConfig["trcservicename"].(string))
	} else if *codebundledeployPtr {
		if pluginToolConfig["trcsha256"] != nil && len(pluginToolConfig["trcsha256"].(string)) > 0 {
			err := repository.GetImageAndShaFromDownload(configBase, pluginToolConfig)
			if err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
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
				os.Exit(1)
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
				os.Exit(1)
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
					os.Exit(1)
				}

				writeMap, replacedFields := properties.GetPluginData(*regionPtr, "Certify", "config", logger)

				writeErr := properties.WritePluginData(WriteMapUpdate(writeMap, pluginToolConfig, *defineServicePtr, *pluginTypePtr), replacedFields, mod, config.Log, *regionPtr, pluginToolConfig["trcplugin"].(string))
				if writeErr != nil {
					fmt.Println(writeErr)
					os.Exit(1)
				}
			} else { //Non region certify
				writeMap, readErr := mod.ReadData(pluginToolConfig["pluginpath"].(string))
				if readErr != nil {
					fmt.Println(readErr)
					os.Exit(1)
				}

				_, err = mod.Write(pluginToolConfig["pluginpath"].(string), WriteMapUpdate(writeMap, pluginToolConfig, *defineServicePtr, *pluginTypePtr), configBase.Log)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				fmt.Println("Image certified in vault and is ready for release.")
			}
		} else {
			fmt.Println("Invalid or nonexistent image.")
			os.Exit(1)
		}
	} else if *agentdeployPtr {
		if config.FeatherCtlCb != nil {
			err := config.FeatherCtlCb(*pluginNamePtr)
			if err != nil {
				fmt.Println("Incorrect installation")
				os.Exit(1)
			}
		} else {
			fmt.Println("Incorrect trcplgtool utilization")
			os.Exit(1)
		}
	}

	//Checks if image has been copied & deployed
	if *checkDeployedPtr {
		if (pluginToolConfig["copied"] != nil && pluginToolConfig["copied"].(bool)) &&
			(pluginToolConfig["deployed"] != nil && pluginToolConfig["deployed"].(bool)) &&
			(pluginToolConfig["trcsha256"] != nil && pluginToolConfig["trcsha256"].(string) == *sha256Ptr) { //Compare vault sha with provided sha
			fmt.Println("Plugin has been copied, deployed & certified.")
			os.Exit(0)
		}

		err := repository.GetImageAndShaFromDownload(configBase, pluginToolConfig)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		if *sha256Ptr == pluginToolConfig["imagesha256"].(string) { //Compare repo image sha with provided sha
			fmt.Println("Latest plugin image sha matches provided plugin sha.  It has been certified.")
		} else {
			fmt.Println("Provided plugin sha is not deployable.")
			os.Exit(1)
		}

		fmt.Println("Plugin has not been copied or deployed.")
		os.Exit(2)
	}

	if *checkCopiedPtr {
		if pluginToolConfig["copied"].(bool) && pluginToolConfig["trcsha256"].(string) == *sha256Ptr { //Compare vault sha with provided sha
			fmt.Println("Plugin has been copied & certified.")
			os.Exit(0)
		}

		err := repository.GetImageAndShaFromDownload(configBase, pluginToolConfig)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		if *sha256Ptr == pluginToolConfig["imagesha256"].(string) { //Compare repo image sha with provided sha
			fmt.Println("Latest plugin image sha matches provided plugin sha.  It has been certified.")
		} else {
			fmt.Println("Provided plugin sha is not certified.")
			os.Exit(1)
		}

		fmt.Println("Plugin has not been copied or deployed.")
		os.Exit(2)
	}
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

package trcplgtoolbase

import (
	"bufio"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	trcvutils "github.com/trimble-oss/tierceron/trcvault/util"
	"github.com/trimble-oss/tierceron/trcvault/util/repository"
	eUtils "github.com/trimble-oss/tierceron/utils"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
)

func CommonMain(envPtr *string,
	addrPtr *string,
	tokenPtr *string,
	regionPtr *string,
	c *eUtils.DriverConfig) {
	startDirPtr := flag.String("startDir", coreopts.GetFolderPrefix(nil)+"_templates", "Template directory")
	insecurePtr := flag.Bool("insecure", false, "By default, every ssl connection is secure.  Allows to continue with server connections considered insecure.")
	logFilePtr := flag.String("log", "./"+coreopts.GetFolderPrefix(nil)+"plgtool.log", "Output path for log files")
	certifyImagePtr := flag.Bool("certify", false, "Used to certifies vault plugin.")
	pluginNamePtr := flag.String("pluginName", "", "Used to certify vault plugin")
	pluginTypePtr := flag.String("pluginType", "vault", "Used to indicate type of plugin.  Default is vault.")
	deployrootPtr := flag.String("deployroot", "", "Optional path for deploying services to.")
	serviceNamePtr := flag.String("serviceName", "", "Optional name of service to use in managing service.")
	codeBundlePtr := flag.String("codeBundle", "", "Code bundle to deploy.")
	defineServicePtr := flag.Bool("defineService", false, "Service is defined.")

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
		flag.CommandLine.Parse(nil)
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

	switch *pluginTypePtr {
	case "vault": // A vault plugin
	case "agent": // A deployment agent tool.
	case "trcshservice": // A trcshservice managed microservice
	default:
		fmt.Println("Unsupported plugin type: " + *pluginTypePtr)
		os.Exit(1)
	}

	var configBase *eUtils.DriverConfig
	if c != nil {
		configBase = c
		*insecurePtr = configBase.Insecure
	} else {
		// If logging production directory does not exist and is selected log to local directory
		if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/"+coreopts.GetFolderPrefix(nil)+"plgtool.log" {
			*logFilePtr = "./" + coreopts.GetFolderPrefix(nil) + "plgtool.log"
		}
		f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		logger := log.New(f, "[INIT]", log.LstdFlags)

		configBase = &eUtils.DriverConfig{Insecure: *insecurePtr, Log: logger, ExitOnFailure: true, StartDir: []string{*startDirPtr}, SubSectionValue: *pluginNamePtr}
		eUtils.CheckError(configBase, err, true)
	}

	regions := []string{}

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

	//Grabbing configs
	mod, err := helperkv.NewModifier(*insecurePtr, *tokenPtr, *addrPtr, *envPtr, nil, true, configBase.Log)
	if mod != nil {
		defer mod.Release()
	}
	if err != nil {
		eUtils.CheckError(configBase, err, true)
	}
	mod.Env = *envPtr
	// Get existing configs if they exist...
	pluginToolConfig, plcErr := trcvutils.GetPluginToolConfig(configBase, mod, coreopts.ProcessDeployPluginEnvConfig(map[string]interface{}{}))
	if plcErr != nil {
		fmt.Println(plcErr.Error())
		os.Exit(1)
	}

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
	}

	//Certify Image
	if *certifyImagePtr {
		if !certifyInit {
			err := repository.GetImageAndShaFromDownload(configBase, pluginToolConfig)
			if err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}
		}

		if certifyInit || pluginToolConfig["trcsha256"].(string) == pluginToolConfig["imagesha256"].(string) { //Comparing generated sha from image to sha from flag
			fmt.Println("Valid image found.")
			//SHA MATCHES
			fmt.Printf("Connecting to vault @ %s\n", *addrPtr)
			writeMap := make(map[string]interface{})
			writeMap["trcplugin"] = pluginToolConfig["trcplugin"].(string)
			writeMap["trctype"] = *pluginTypePtr
			writeMap["trcsha256"] = pluginToolConfig["trcsha256"].(string)
			if pluginToolConfig["instances"] == nil {
				pluginToolConfig["instances"] = "0"
			}
			writeMap["instances"] = pluginToolConfig["instances"].(string)
			writeMap["copied"] = false
			writeMap["deployed"] = false
			_, err = mod.Write(pluginToolConfig["pluginpath"].(string), writeMap, configBase.Log)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			fmt.Println("Image certified in vault and is ready for release.")

		} else {
			fmt.Println("Invalid or nonexistent image.")
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

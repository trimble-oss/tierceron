package trcplugtool

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	trcname "tierceron/trcvault/opts/trcname"
	"tierceron/trcvault/util"
	"tierceron/trcvault/util/repository"
	eUtils "tierceron/utils"
	"tierceron/vaulthelper/kv"

	tcutil "VaultConfig.TenantConfig/util"
)

func PluginMain() {
	addrPtr := flag.String("addr", "", "API endpoint for the vault")
	tokenPtr := flag.String("token", "", "Vault access token")
	envPtr := flag.String("env", "dev", "Environement in vault")
	startDirPtr := flag.String("startDir", trcname.GetFolderPrefix()+"_templates", "Template directory")
	insecurePtr := flag.Bool("insecure", false, "By default, every ssl connection is secure.  Allows to continue with server connections considered insecure.")
	logFilePtr := flag.String("log", "./"+trcname.GetFolderPrefix()+"plgtool.log", "Output path for log files")
	certifyImagePtr := flag.Bool("certify", false, "Used to certifies vault plugin.")
	pluginNamePtr := flag.String("pluginName", "", "Used to certify vault plugin")
	sha256Ptr := flag.String("sha256", "", "Used to certify vault plugin") //This has to match the image that is pulled -> then we write the vault.
	checkDeployedPtr := flag.Bool("checkDeployed", false, "Used to check if plugin has been copied, deployed, & certified")
	certifyInit := false

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

	if *certifyImagePtr && (len(*pluginNamePtr) == 0 || len(*sha256Ptr) == 0) {
		fmt.Println("Must use -pluginName && -sha256 flags to use -certify flag")
		os.Exit(1)
	}

	if *checkDeployedPtr && (len(*pluginNamePtr) == 0) {
		fmt.Println("Must use -pluginName flag to use -checkDeployed flag")
		os.Exit(1)
	}

	// If logging production directory does not exist and is selected log to local directory
	if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/"+trcname.GetFolderPrefix()+"plgtool.log" {
		*logFilePtr = "./" + trcname.GetFolderPrefix() + "plgtool.log"
	}
	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)

	logger := log.New(f, "[INIT]", log.LstdFlags)
	config := &eUtils.DriverConfig{Insecure: *insecurePtr, Log: logger, ExitOnFailure: true, StartDir: []string{*startDirPtr}, SubSectionValue: *pluginNamePtr}

	eUtils.CheckError(config, err, true)

	//Grabbing configs
	mod, err := kv.NewModifier(*insecurePtr, *tokenPtr, *addrPtr, *envPtr, nil, logger)
	if err != nil {
		eUtils.CheckError(config, err, true)
	}
	mod.Env = *envPtr
	pluginToolConfig, plcErr := util.GetPluginToolConfig(config, mod, tcutil.ProcessDeployPluginEnvConfig(map[string]interface{}{}))
	if plcErr != nil {
		fmt.Println(plcErr.Error())
		os.Exit(1)
	}
	pluginToolConfig["ecrrepository"] = strings.Replace(pluginToolConfig["ecrrepository"].(string), "__imagename__", *pluginNamePtr, -1) //"https://" +
	pluginToolConfig["trcsha256"] = *sha256Ptr
	pluginToolConfig["pluginNamePtr"] = *pluginNamePtr

	if _, ok := pluginToolConfig["trcplugin"].(string); !ok {
		pluginToolConfig["trcplugin"] = pluginToolConfig["pluginNamePtr"].(string)
		if *certifyImagePtr {
			certifyInit = true
		}
	}
	//Certify Image
	if *certifyImagePtr {
		if !certifyInit {
			err := repository.GetImageAndShaFromDownload(pluginToolConfig)
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
			writeMap["trcsha256"] = pluginToolConfig["trcsha256"].(string)
			writeMap["copied"] = false
			writeMap["deployed"] = false
			pathSplit := strings.Split(mod.SectionPath, "/")
			_, err = mod.Write(pathSplit[0]+"/"+pathSplit[len(pathSplit)-1], writeMap)
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
		if pluginToolConfig["copied"].(bool) && pluginToolConfig["deployed"].(bool) && pluginToolConfig["trcsha256"].(string) == *sha256Ptr { //Compare vault sha with provided sha
			fmt.Println("Plugin has been copied, deployed & certified.")
			os.Exit(0)
		}

		err := repository.GetImageAndShaFromDownload(pluginToolConfig)
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
}

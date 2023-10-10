package deploy

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/trcvault/carrierfactory/capauth"
	"github.com/trimble-oss/tierceron/trcvault/factory"
	"github.com/trimble-oss/tierceron/trcvault/opts/prod"
	trcvutils "github.com/trimble-oss/tierceron/trcvault/util"
	"github.com/trimble-oss/tierceron/trcvault/util/repository"
	sys "github.com/trimble-oss/tierceron/vaulthelper/system"

	eUtils "github.com/trimble-oss/tierceron/utils"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
	//"kernel.org/pub/linux/libs/security/libcap/cap"
)

func init() {
	factory.StartPluginSettingEater()
}

var onceAuth sync.Once

func PluginDeployEnvFlow(pluginConfig map[string]interface{}, logger *log.Logger) error {
	logger.Println("PluginDeployInitFlow begun.")
	var err error

	onceAuth.Do(func() {
		logger.Println("Cap auth init. ")
		var config *eUtils.DriverConfig
		var goMod *helperkv.Modifier
		var vault *sys.Vault

		//Grabbing configs
		tempAddr := pluginConfig["vaddress"]
		tempToken := pluginConfig["token"]
		pluginConfig["vaddress"] = pluginConfig["caddress"]
		pluginConfig["token"] = pluginConfig["ctoken"]
		config, goMod, vault, err = eUtils.InitVaultModForPlugin(pluginConfig, logger)
		if vault != nil {
			defer vault.Close()
		}

		if goMod != nil {
			defer goMod.Release()
		}
		pluginConfig["vaddress"] = tempAddr
		pluginConfig["token"] = tempToken

		if err != nil {
			eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start.", false)
			return
		}

		err = capauth.Init(goMod, pluginConfig, logger)
		if err != nil {
			eUtils.LogErrorMessage(config, "Skipping cap auth init.", false)
			return
		}

		capauth.Memorize(pluginConfig, logger)

		// TODO: Support variables for different environments...
		go capauth.Start(logger)
		logger.Println("Cap auth init complete.")
	})

	logger.Println("PluginDeployInitFlow complete.")

	return err
}

func PluginDeployFlow(pluginConfig map[string]interface{}, logger *log.Logger) error {
	logger.Println("PluginDeployFlow begun.")
	var config *eUtils.DriverConfig
	var vault *sys.Vault
	var err error
	var pluginName string

	if pluginNameInterface, pluginNameOk := pluginConfig["trcplugin"]; pluginNameOk {
		pluginName = pluginNameInterface.(string)
	} else {
		logger.Println("Missing plugin name.")
		return errors.New("missing plugin name")
	}

	hostName, hostNameErr := os.Hostname()
	if hostNameErr != nil {
		return hostNameErr
	} else if hostName == "" {
		return errors.New("could not find hostname")
	}

	//Grabbing certification from vault
	if pluginConfig["caddress"].(string) == "" { //if no certification address found, it will try to certify against itself.
		return errors.New("could not find certification address")
	}
	if pluginConfig["ctoken"].(string) == "" { //if no certification address found, it will try to certify against itself.
		return errors.New("could not find certification token")
	}

	tempAddr := pluginConfig["vaddress"]
	tempToken := pluginConfig["token"]
	pluginConfig["vaddress"] = pluginConfig["caddress"]
	pluginConfig["token"] = pluginConfig["ctoken"]
	cConfig, cGoMod, _, err := eUtils.InitVaultModForPlugin(pluginConfig, logger)
	cConfig.SubSectionValue = pluginName
	if err != nil {
		eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start.", false)
		return err
	}

	vaultPluginSignature, ptcErr := trcvutils.GetPluginToolConfig(cConfig, cGoMod, pluginConfig)
	if ptcErr != nil {
		eUtils.LogErrorMessage(config, "PluginDeployFlow failure: plugin load failure: "+ptcErr.Error(), false)
	}
	pluginConfig["vaddress"] = tempAddr
	pluginConfig["token"] = tempToken
	//grabbing configs
	config, _, vault, err = eUtils.InitVaultModForPlugin(pluginConfig, logger)
	config.SubSectionValue = pluginName
	if vault != nil {
		defer vault.Close()
	}
	if err != nil {
		eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start.", false)
		return err
	}
	logger.Println("PluginDeployFlow begun for plugin: " + pluginName)
	insecure := false
	if ok, _ := pluginConfig["insecure"].(bool); ok {
		insecure = pluginConfig["insecure"].(bool)
	}
	config = &eUtils.DriverConfig{Insecure: insecure, Log: logger, ExitOnFailure: false, StartDir: []string{}, SubSectionValue: pluginName}

	if _, ok := vaultPluginSignature["trcplugin"]; !ok {
		// TODO: maybe delete plugin if it exists since there was no entry in vault...
		eUtils.LogErrorMessage(config, "PluginDeployFlow failure: plugin status load failure.", false)
	}

	if _, ok := vaultPluginSignature["acrrepository"].(string); !ok {
		// TODO: maybe delete plugin if it exists since there was no entry in vault...
		eUtils.LogErrorMessage(config, "PluginDeployFlow failure: plugin status load failure - no certification entry found.", false)
	}

	//Checks if this instance of carrier is allowed to deploy that certain plugin.
	if instanceList, ok := vaultPluginSignature["instances"].(string); !ok {
		eUtils.LogErrorMessage(config, "Plugin has no valid instances: "+vaultPluginSignature["trcplugin"].(string), false)
		return nil
	} else {
		hostName, hostNameErr := os.Hostname()
		if hostName != "" && hostNameErr == nil { //Figures out what instance this is
			re := regexp.MustCompile("-[0-9]+")
			hostNameRegex := re.FindAllString(hostName, 1)

			var instanceIndex string
			if len(hostNameRegex) > 0 {
				instanceIndex = strings.TrimPrefix(hostNameRegex[0], "-")
			} else {
				instanceIndex = "0"
			}

			instances := strings.Split(instanceList, ",") //Checks whether this instance is allowed to run plugin
			instanceFound := false
			for _, instance := range instances {
				if strings.TrimSuffix(strings.TrimPrefix(instance, "\""), ("\"")) == instanceIndex {
					instanceFound = true
					break
				}
			}

			if instanceFound {
				logger.Println("Plugin found for this instance: " + vaultPluginSignature["trcplugin"].(string))
				vaultPluginSignature["deployed"] = false
				vaultPluginSignature["copied"] = false //Resets copied & deployed in memory to reset deployment for this instance.
			} else {
				eUtils.LogErrorMessage(config, "Plugin not found for this instance: "+vaultPluginSignature["trcplugin"].(string), false)
				return nil
			}
		} else {
			eUtils.LogErrorMessage(config, "Unable to determine this instance's index for deployment: "+vaultPluginSignature["trcplugin"].(string)+" Error:"+hostNameErr.Error(), false)
			return nil
		}
	}

	if deployedVal, ok := vaultPluginSignature["deployed"].(bool); ok && deployedVal {
		eUtils.LogErrorMessage(config, "Plugin has already been deployed and copied: "+vaultPluginSignature["trcplugin"].(string), false)
		return nil
	}

	pluginDownloadNeeded := false
	pluginCopied := false
	var agentPath string

	pluginExtension := ""
	if prod.IsProd() {
		pluginExtension = "-prod"
	}

	switch vaultPluginSignature["trctype"] {
	case "agent":
		agentPath = "/home/azuredeploy/bin/" + vaultPluginSignature["trcplugin"].(string)
	case "service":
		agentPath = vaultPluginSignature["trcpluginpath"].(string) + vaultPluginSignature["trcplugin"].(string)
	default:
		agentPath = "/etc/opt/vault/plugins/" + vaultPluginSignature["trcplugin"].(string) + pluginExtension
	}

	if _, err := os.Stat(agentPath); errors.Is(err, os.ErrNotExist) {
		pluginDownloadNeeded = true
		logger.Println("Attempting to download new image.")
	} else {
		if imageFile, err := os.Open(agentPath); err == nil {
			logger.Println("Found image for: " + vaultPluginSignature["trcplugin"].(string))

			sha256 := sha256.New()

			defer imageFile.Close()
			if _, err := io.Copy(sha256, imageFile); err != nil {
				eUtils.LogErrorMessage(config, "PluginDeployFlow failure: Could not sha256 image from file system.", false)
			}

			filesystemsha256 := fmt.Sprintf("%x", sha256.Sum(nil))
			if filesystemsha256 != vaultPluginSignature["trcsha256"] { //Sha256 from file system matches in vault
				pluginDownloadNeeded = true
			} else {
				eUtils.LogErrorMessage(config, "Certified plugin already exists in file system - continuing with vault plugin status update", false)
			}
		} else {
			pluginDownloadNeeded = true
			logger.Println("Attempting to download new image.")
		}
	}

	if pluginDownloadNeeded {
		logger.Println("PluginDeployFlow new plugin image found: " + pluginName)

		// 1.c.i. Download new image from ECR.
		// 1.c.ii. Sha256 of new executable.
		// 1.c.ii.- if Sha256 of new executable === sha256 from vault.
		downloadErr := repository.GetImageAndShaFromDownload(config, vaultPluginSignature)
		if downloadErr != nil {
			eUtils.LogErrorMessage(config, pluginName+": Could not get download image: "+downloadErr.Error(), false)
			vaultPluginSignature["imagesha256"] = "invalidurl"
		}
		if vaultPluginSignature["imagesha256"] == vaultPluginSignature["trcsha256"] { //Sha256 from download matches in vault
			err = os.WriteFile(agentPath, vaultPluginSignature["rawImageFile"].([]byte), 0644)
			vaultPluginSignature["rawImageFile"] = nil

			if err != nil {
				eUtils.LogErrorMessage(config, pluginName+": PluginDeployFlow failure: Could not write out download image.", false)
			}

			if vaultPluginSignature["trctype"] == "agent" {
				azureDeployGroup, azureDeployGroupErr := user.LookupGroup("azuredeploy")
				if azureDeployGroupErr != nil {
					return errors.Join(errors.New("group lookup failure"), azureDeployGroupErr)
				}
				azureDeployGID, azureGIDConvErr := strconv.Atoi(azureDeployGroup.Gid)
				if azureGIDConvErr != nil {
					return errors.Join(errors.New("group ID lookup failure"), azureGIDConvErr)
				}
				os.Chown(agentPath, -1, azureDeployGID)
			}

			if imageFile, err := os.Open(agentPath); err == nil {
				chdModErr := imageFile.Chmod(0750)
				if chdModErr != nil {
					eUtils.LogErrorMessage(config, pluginName+": PluginDeployFlow failure: Could not give permission to image in file system.  Bailing..", false)
					return nil
				}
			} else {
				eUtils.LogErrorMessage(config, pluginName+": PluginDeployFlow failure: Could not open image in file system to give permissions.", false)
				return nil
			}

			// TODO: setcap more directly using kernel lib if possible...
			//"kernel.org/pub/linux/libs/security/libcap/cap"

			//				capSet, err := cap.GetFile("/etc/opt/vault/plugins/" + vaultPluginSignature["trcplugin"].(string))
			//				cap.GetFd
			//				capSet.SetFlag(cap.Permitted, true)
			cmd := exec.Command("setcap", "cap_ipc_lock=+ep", agentPath)
			output, err := cmd.CombinedOutput()
			if err != nil {
				eUtils.LogErrorMessage(config, fmt.Sprint(err)+": "+string(output), false)
				eUtils.LogErrorMessage(config, pluginName+": PluginDeployFlow failure: Could not set needed capabilities.", false)
			}

			pluginCopied = true
			eUtils.LogInfo(config, pluginName+": Image has been copied.")
		} else {
			eUtils.LogErrorMessage(config, fmt.Sprintf("%s: PluginDeployFlow failure: Refusing to copy since vault certification does not match plugin sha256 signature.  Downloaded: %s, Expected: %s", pluginName, vaultPluginSignature["imagesha256"], vaultPluginSignature["trcsha256"]), false)
		}
	}

	if (!pluginDownloadNeeded && !pluginCopied) || (pluginDownloadNeeded && pluginCopied) { // No download needed because it's already there, but vault may be wrong.
		if vaultPluginSignature["copied"].(bool) && !vaultPluginSignature["deployed"].(bool) { //If status hasn't changed, don't update
			eUtils.LogInfo(config, pluginName+": Not updating plugin image to vault as status is the same for plugin: "+pluginName)
		}

		eUtils.LogInfo(config, pluginName+": Updating plugin image to vault.")
		if pluginSHA, pluginSHAOk := vaultPluginSignature["trcsha256"]; !pluginSHAOk || pluginSHA.(string) == "" {
			eUtils.LogInfo(config, "Plugin is not registered with carrier: "+pluginName)
			return nil
		}
		factory.PushPluginSha(config, pluginConfig, vaultPluginSignature)
		writeMap := make(map[string]interface{})
		writeMap["trcplugin"] = vaultPluginSignature["trcplugin"].(string)
		writeMap["trcsha256"] = vaultPluginSignature["trcsha256"].(string)
		writeMap["instances"] = vaultPluginSignature["instances"].(string)
		if trcType, trcTypeOk := vaultPluginSignature["trctype"]; trcTypeOk {
			writeMap["trctype"] = trcType.(string)
		} else {
			writeMap["trctype"] = "vault"
		}
		writeMap["copied"] = false
		writeMap["deployed"] = false
		if writeMap["trctype"].(string) == "agent" {
			writeMap["deployed"] = true
		}
		_, err = cGoMod.Write("super-secrets/Index/TrcVault/trcplugin/"+writeMap["trcplugin"].(string)+"/Certify", writeMap, config.Log)
		if err != nil {
			logger.Println(pluginName + ": PluginDeployFlow failure: Failed to write plugin state: " + err.Error())
		}
		eUtils.LogInfo(config, pluginName+": Plugin image config in vault has been updated.")
	} else {
		if !pluginDownloadNeeded && pluginCopied {
			eUtils.LogInfo(config, pluginName+": Not updating plugin image to vault as status is the same for  plugin: "+pluginName)
			// Already copied... Just echo back the sha256...
		}
	}
	// ALways set this so it completes if there is a sha256 available...
	// This will also release any clients attempting to communicate with carrier.
	factory.PushPluginSha(config, pluginConfig, vaultPluginSignature)

	logger.Println("PluginDeployFlow complete.")

	return nil
}

// Updated deployed to true for any plugin
func PluginDeployedUpdate(config *eUtils.DriverConfig, mod *helperkv.Modifier, vault *sys.Vault, pluginNameList []string, cPath []string, logger *log.Logger) error {
	logger.Println("PluginDeployedUpdate start.")

	hostName, hostNameErr := os.Hostname()
	if hostNameErr != nil {
		return hostNameErr
	} else if hostName == "" {
		return errors.New("Could not find hostname.")
	}

	hostRegion := coreopts.GetRegion(hostName)
	//mod.Regions = append(mod.Regions, hostRegion)
	projects, services, _ := eUtils.GetProjectServices(cPath)

	hostRegion = "west"
	mod.Regions = append(mod.Regions, hostRegion)
	pluginNameList = append(pluginNameList, "trc-vault-plugin") //delete this line later

	for _, pluginName := range pluginNameList {
		for i := 0; i < len(projects); i++ {
			if services[i] == "Certify" {
				mod.SectionName = "trcplugin"
				mod.SectionKey = "/Index/"
				mod.SubSectionValue = pluginName

			}
			properties, err := trcvutils.NewProperties(config, vault, mod, config.Env, projects[i], services[i])
			if err != nil {
				return err
			}
			properties.GetConfigValues(services[i], "copied")
		}
	}

	for _, pluginName := range pluginNameList {
		pluginData, err := mod.ReadData("super-secrets/Index/TrcVault/trcplugin/" + pluginName + "/Certify")
		if err != nil {
			return err
		}
		if pluginData == nil {
			pluginData = make(map[string]interface{})
			pluginData["trcplugin"] = pluginName

			var agentPath string
			pluginExtension := ""
			if prod.IsProd() {
				pluginExtension = "-prod"
			}

			if pluginData["trctype"] == "agent" {
				agentPath = "/home/azuredeploy/bin/" + pluginName
			} else {
				agentPath = "/etc/opt/vault/plugins/" + pluginName + pluginExtension
			}

			logger.Println("Checking file.")
			if imageFile, err := os.Open(agentPath); err == nil {
				sha256 := sha256.New()

				defer imageFile.Close()
				if _, err := io.Copy(sha256, imageFile); err != nil {
					continue
				}

				filesystemsha256 := fmt.Sprintf("%x", sha256.Sum(nil))
				pluginData["trcsha256"] = filesystemsha256
				pluginData["copied"] = false
				pluginData["instances"] = "0"

				if pluginData["trctype"].(string) == "agent" {
					pluginData["deployed"] = false
				}
			}
		}

		if copied, okCopied := pluginData["copied"]; !okCopied || !copied.(bool) {
			logger.Println("Cannot certify plugin.  Plugin not copied: " + pluginName)
			continue
		}

		if deployed, okDeployed := pluginData["deployed"]; !okDeployed || deployed.(bool) {
			continue
		}

		writeMap := make(map[string]interface{})
		writeMap["trcplugin"] = pluginData["trcplugin"]
		writeMap["trctype"] = pluginData["trctype"]
		writeMap["trcsha256"] = pluginData["trcsha256"]
		writeMap["copied"] = pluginData["copied"]
		writeMap["instances"] = pluginData["instances"]
		writeMap["deployed"] = false

		_, err = mod.Write("super-secrets/Index/TrcVault/trcplugin/"+pluginName+"/Certify", writeMap, logger)
		if err != nil {
			return err
		}
	}
	logger.Println("PluginDeployedUpdate complete.")
	return nil
}

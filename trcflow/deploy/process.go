package deploy

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"sync"

	"github.com/trimble-oss/tierceron/trcvault/carrierfactory/capauth"
	"github.com/trimble-oss/tierceron/trcvault/factory"
	"github.com/trimble-oss/tierceron/trcvault/opts/insecure"
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

func PluginDeployFlow(pluginConfig map[string]interface{}, logger *log.Logger) error {
	logger.Println("PluginDeployFlow begun.")
	var config *eUtils.DriverConfig
	var goMod *helperkv.Modifier
	var vault *sys.Vault
	var err error
	var pluginName string

	if pluginNameInterface, pluginNameOk := pluginConfig["trcplugin"]; pluginNameOk {
		pluginName = pluginNameInterface.(string)
	} else {
		logger.Println("Missing plugin name.")
		return errors.New("missing plugin name")
	}

	//Grabbing configs
	config, goMod, vault, err = eUtils.InitVaultModForPlugin(pluginConfig, logger)
	if vault != nil {
		defer vault.Close()
	}
	if err != nil {
		eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start.", false)
		return err
	}

	onceAuth.Do(func() {
		logger.Println("Cap auth init. ")
		capauth.Init(goMod, pluginConfig, logger)
		// TODO: Support multiple environments...
		capauth.Memorize(pluginConfig, logger)
		go capauth.Start(logger)
		logger.Println("Cap auth init complete.")
	})

	logger.Println("PluginDeployFlow begun for plugin: " + pluginName)
	config = &eUtils.DriverConfig{Insecure: pluginConfig["insecure"].(bool), Log: logger, ExitOnFailure: false, StartDir: []string{}, SubSectionValue: pluginName}

	vaultPluginSignature, ptcErr := trcvutils.GetPluginToolConfig(config, goMod, pluginConfig)
	if ptcErr != nil {
		eUtils.LogErrorMessage(config, "PluginDeployFlow failure: plugin load failure: "+ptcErr.Error(), false)
	}

	if _, ok := vaultPluginSignature["trcplugin"]; !ok {
		// TODO: maybe delete plugin if it exists since there was no entry in vault...
		eUtils.LogErrorMessage(config, "PluginDeployFlow failure: plugin status load failure.", false)
	}

	if _, ok := vaultPluginSignature["ecrrepository"].(string); !ok {
		// TODO: maybe delete plugin if it exists since there was no entry in vault...
		eUtils.LogErrorMessage(config, "PluginDeployFlow failure: plugin status load failure - no certification entry found.", false)
	}

	// This should come from vault now....
	vaultPluginSignature["ecrrepository"] = strings.Replace(vaultPluginSignature["ecrrepository"].(string), "__imagename__", pluginName, -1) //"https://" +

	pluginDownloadNeeded := false
	pluginCopied := false
	var agentPath string
	if vaultPluginSignature["trctype"] == "agent" {
		agentPath = "/home/azuredeploy/bin/" + vaultPluginSignature["trcplugin"].(string)
	} else {
		agentPath = "/etc/opt/vault/plugins/" + vaultPluginSignature["trcplugin"].(string)
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
		downloadErr := repository.GetImageAndShaFromDownload(vaultPluginSignature)
		if downloadErr != nil {
			eUtils.LogErrorMessage(config, "Could not get download image: "+downloadErr.Error(), false)
		}
		if vaultPluginSignature["imagesha256"] == vaultPluginSignature["trcsha256"] { //Sha256 from download matches in vault
			err = ioutil.WriteFile(agentPath, vaultPluginSignature["rawImageFile"].([]byte), 0644)
			vaultPluginSignature["rawImageFile"] = nil

			if err != nil {
				eUtils.LogErrorMessage(config, "PluginDeployFlow failure: Could not write out download image.", false)
			}

			if imageFile, err := os.Open(agentPath); err == nil {
				chdModErr := imageFile.Chmod(0750)
				if chdModErr != nil {
					eUtils.LogErrorMessage(config, "PluginDeployFlow failure: Could not give permission to image in file system.", false)
				}
			} else {
				eUtils.LogErrorMessage(config, "PluginDeployFlow failure: Could not open image in file system to give permissions.", false)
			}

			if vaultPluginSignature["trctype"] == "agent" {
				azureDeployGroup, azureDeployGroupErr := user.LookupGroup("azuredeploy")
				if azureDeployGroupErr != nil {
					return errors.Join(errors.New("Group lookup failure"), azureDeployGroupErr)
				}
				azureDeployGID, azureGIDConvErr := strconv.Atoi(azureDeployGroup.Gid)
				if azureGIDConvErr != nil {
					return errors.Join(errors.New("Group ID lookup failure"), azureGIDConvErr)
				}
				os.Chown(agentPath, -1, azureDeployGID)
				os.Chmod(agentPath, 1750)
			}

			// TODO: setcap more directly using kernel lib if possible...
			//"kernel.org/pub/linux/libs/security/libcap/cap"

			//				capSet, err := cap.GetFile("/etc/opt/vault/plugins/" + vaultPluginSignature["trcplugin"].(string))
			//				cap.GetFd
			//				capSet.SetFlag(cap.Permitted, true)
			cmd := exec.Command("setcap", "cap_ipc_lock=+ep", agentPath)
			output, err := cmd.CombinedOutput()
			if !insecure.IsInsecure() && err != nil {
				eUtils.LogErrorMessage(config, fmt.Sprint(err)+": "+string(output), false)
				eUtils.LogErrorMessage(config, "PluginDeployFlow failure: Could not set needed capabilities.", false)
			}

			pluginCopied = true
			eUtils.LogInfo(config, "Image has been copied.")
		} else {
			fmt.Sprintf("PluginDeployFlow failure: Refusing to copy since vault certification does not match plugin sha256 signature.  Downloaded: %s, Expected: %s", vaultPluginSignature["imagesha256"], vaultPluginSignature["trcsha256"])
			eUtils.LogErrorMessage(config, "PluginDeployFlow failure: Refusing to copy since vault certification does not match plugin sha256 signature.", false)
		}
	}

	if (!pluginDownloadNeeded && !pluginCopied) || (pluginDownloadNeeded && pluginCopied) { // No download needed because it's already there, but vault may be wrong.
		if vaultPluginSignature["copied"].(bool) && !vaultPluginSignature["deployed"].(bool) { //If status hasn't changed, don't update
			eUtils.LogInfo(config, "Not updating plugin image to vault as status is the same for plugin: "+pluginName)
			factory.PushPluginSha(config, pluginConfig, vaultPluginSignature)
		}

		eUtils.LogInfo(config, "Updating plugin image to vault.")
		factory.PushPluginSha(config, pluginConfig, vaultPluginSignature)
		writeMap := make(map[string]interface{})
		writeMap["trcplugin"] = vaultPluginSignature["trcplugin"].(string)
		writeMap["trcsha256"] = vaultPluginSignature["trcsha256"].(string)
		if trcType, trcTypeOk := vaultPluginSignature["trctype"]; trcTypeOk {
			writeMap["trctype"] = trcType.(string)
		} else {
			writeMap["trctype"] = "vault"
		}
		writeMap["copied"] = true
		writeMap["deployed"] = false
		_, err = goMod.Write("super-secrets/Index/TrcVault/trcplugin/"+writeMap["trcplugin"].(string)+"/Certify", writeMap, config.Log)
		if err != nil {
			logger.Println("PluginDeployFlow failure: Failed to write plugin state: " + err.Error())
		}
		eUtils.LogInfo(config, "Plugin image config in vault has been updated.")
	} else {
		if !pluginDownloadNeeded && pluginCopied {
			eUtils.LogInfo(config, "Not updating plugin image to vault as status is the same for  plugin: "+pluginName)
			// Already copied... Just echo back the sha256...
			factory.PushPluginSha(config, pluginConfig, vaultPluginSignature)
		}
	}

	logger.Println("PluginDeployFlow complete.")

	return nil
}

// Updated deployed to true for any plugin
func PluginDeployedUpdate(mod *helperkv.Modifier, pluginNameList []string, logger *log.Logger) error {
	logger.Println("PluginDeployedUpdate start.")

	for _, pluginName := range pluginNameList {
		pluginData, err := mod.ReadData("super-secrets/Index/TrcVault/trcplugin/" + pluginName + "/Certify")
		if err != nil {
			return err
		}
		if pluginData == nil {
			if !prod.IsProd() && insecure.IsInsecure() {
				pluginData = make(map[string]interface{})
				pluginData["trcplugin"] = pluginName

				var agentPath string
				if pluginData["trctype"] == "agent" {
					agentPath = "/home/azuredeploy/bin/" + pluginName
				} else {
					agentPath = "/etc/opt/vault/plugins/" + pluginName
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
					pluginData["copied"] = true
				}
			} else {
				return errors.New("Plugin not certified.")
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
		writeMap["deployed"] = true

		_, err = mod.Write("super-secrets/Index/TrcVault/trcplugin/"+pluginName+"/Certify", writeMap, logger)
		if err != nil {
			return err
		}
	}
	logger.Println("PluginDeployedUpdate complete.")
	return nil
}

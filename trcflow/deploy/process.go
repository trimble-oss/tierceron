package deploy

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"tierceron/trcvault/util"
	"tierceron/trcvault/util/repository"
	"tierceron/utils"

	eUtils "tierceron/utils"
	helperkv "tierceron/vaulthelper/kv"
)

func PluginDeployFlow(pluginConfig map[string]interface{}, logger *log.Logger) error {
	logger.Println("PluginDeployFlow begun.")
	var config *eUtils.DriverConfig
	var goMod *helperkv.Modifier
	var err error

	//Grabbing configs
	config, goMod, _, err = eUtils.InitVaultModForPlugin(pluginConfig, logger)
	if err != nil {
		eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start.", false)
		return err
	}

	logger.Println("PluginDeployFlow begin processing plugins.")
	for _, pluginName := range pluginConfig["pluginNameList"].([]string) {
		logger.Println("PluginDeployFlow begun for plugin: " + pluginName)
		config = &eUtils.DriverConfig{Insecure: pluginConfig["insecure"].(bool), Log: logger, ExitOnFailure: true, StartDir: []string{}, SubSectionValue: pluginName}

		vaultPluginSignature, ptcErr := util.GetPluginToolConfig(config, goMod, pluginConfig)
		if ptcErr != nil {
			eUtils.LogErrorMessage(config, "PluginDeployFlow failure: vault load failure: "+ptcErr.Error(), false)
			continue
		}
		// This should come from vault now....
		vaultPluginSignature["ecrrepository"] = strings.Replace(vaultPluginSignature["ecrrepository"].(string), "__imagename__", pluginName, -1) //"https://" +

		pluginDownloadNeeded := false
		pluginCopied := false

		if imageFile, err := os.Open("/etc/opt/vault/plugins/" + vaultPluginSignature["trcplugin"].(string)); err == nil {
			sha256 := sha256.New()

			defer imageFile.Close()
			if _, err := io.Copy(sha256, imageFile); err != nil {
				eUtils.LogErrorMessage(config, "PluginDeployFlow failure: Could not sha256 image from file system.", false)
				return err
			}

			filesystemsha256 := fmt.Sprintf("%x", sha256.Sum(nil))
			if filesystemsha256 == vaultPluginSignature["trcsha256"] { //Sha256 from file system matches in vault
				// TODO: write success to vault...
				continue
			} else {
				pluginDownloadNeeded = true
			}
		} else {
			pluginDownloadNeeded = true
			logger.Println("Attempting to download new image.")
		}

		if pluginDownloadNeeded {
			logger.Println("PluginDeployFlow new plugin image found: " + pluginName)

			// 1.c.i. Download new image from ECR.
			// 1.c.ii. Sha256 of new executable.
			// 1.c.ii.- if Sha256 of new executable === sha256 from vault.
			downloadErr := repository.GetImageAndShaFromDownload(vaultPluginSignature)
			if downloadErr != nil {
				eUtils.LogErrorMessage(config, "Could not get download image: "+downloadErr.Error(), false)
				return downloadErr
			}
			if vaultPluginSignature["imagesha256"] == vaultPluginSignature["trcsha256"] { //Sha256 from download matches in vault
				err = ioutil.WriteFile("/etc/opt/vault/plugins/"+vaultPluginSignature["trcplugin"].(string), vaultPluginSignature["rawImageFile"].([]byte), 0644)
				if err != nil {
					eUtils.LogErrorMessage(config, "PluginDeployFlow failure: Could not write out download image.", false)
					return err
				}
				pluginCopied = true
				utils.LogInfo(config, "Image has been copied.")
			} else {
				eUtils.LogErrorMessage(config, "PluginDeployFlow failure: Refusing to copy since vault certification does not match plugin sha256 signature.", false)
				return errors.New("refusing to copy since vault certification does not match plugin sha256 signature")
			}
		}

		if (!pluginDownloadNeeded && !pluginCopied) || // No download needed because it's already there, but vault may be wrong.
			(pluginDownloadNeeded && pluginCopied) {
			writeMap := make(map[string]interface{})
			writeMap["trcplugin"] = vaultPluginSignature["trcplugin"].(string)
			writeMap["trcsha256"] = vaultPluginSignature["trcsha256"].(string)
			writeMap["copied"] = true
			writeMap["deployed"] = false
			_, err = goMod.Write("super-secrets/Index/TrcVault/trcplugin/"+writeMap["trcplugin"].(string)+"/Certify", writeMap)
			if err != nil {
				logger.Println("PluginDeployFlow failure: Failed to write plugin state: " + err.Error())
				return err
			}
			utils.LogInfo(config, "Plugin image config in vault has been updated.")
		}

	}

	logger.Println("PluginDeployFlow complete.")

	return nil
}

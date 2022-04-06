package deploy

import (
	"crypto/sha256"
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

		pluginToolConfig := util.GetPluginToolConfig(config, goMod, pluginConfig)
		// This should come from vault now....
		pluginToolConfig["ecrrepository"] = strings.Replace(pluginToolConfig["ecrrepository"].(string), "__imagename__", pluginName, -1) //"https://" +

		if imageFile, err := os.Open("/etc/opt/vault/plugins/" + pluginToolConfig["trcplugin"].(string)); err == nil {
			if err != nil {
				eUtils.LogErrorMessage(config, "Could not find plugin image", false)
				return err
			}
			sha256 := sha256.New()

			defer imageFile.Close()
			if _, err := io.Copy(sha256, imageFile); err != nil {
				eUtils.LogErrorMessage(config, "Could not sha256 image from file system.", false)
				return err
			}

			pluginToolConfig["filesystemsha256"] = fmt.Sprintf("%x", sha256.Sum(nil))
			if pluginToolConfig["filesystemsha256"] == pluginToolConfig["trcsha256"] { //Sha256 from file system matches in vault
				continue
			}
		} else {
			logger.Println("Attempting to download new image.")
		}
		// 1.c.i. Download new image from ECR.
		// 1.c.ii. Sha256 of new executable.
		// 1.c.ii.- if Sha256 of new executable === sha256 from vault.
		downloadErr := repository.GetImageAndShaFromDownload(pluginToolConfig)
		if downloadErr != nil {
			eUtils.LogErrorMessage(config, "Could not get download image: "+downloadErr.Error(), false)
			return downloadErr
		}
		if pluginToolConfig["imagesha256"] == pluginToolConfig["trcsha256"] { //Sha256 from download matches in vault
			err = ioutil.WriteFile("/etc/opt/vault/plugins/"+pluginToolConfig["trcplugin"].(string), pluginToolConfig["rawImageFile"].([]byte), 0644)
			if err != nil {
				eUtils.LogErrorMessage(config, "Could not write out download image.", false)
				return err
			}

			writeMap := make(map[string]interface{})
			writeMap["trcplugin"] = pluginToolConfig["trcplugin"].(string)
			writeMap["trcsha256"] = pluginToolConfig["trcsha256"].(string)
			writeMap["copied"] = true
			writeMap["deployed"] = false
			_, err = goMod.Write("super-secrets/Index/TrcVault/trcplugin/"+writeMap["trcplugin"].(string)+"/Certify", writeMap)
			if err != nil {
				logger.Println("Failed to write plugin state: " + err.Error())
				return err
			}
			utils.LogInfo(config, "Image has been copied and vault has been updated.")
		}
	}

	// This should come from config
	// 0. List all the plugins under Index/TrcVault/trcplugin

	//pluginEnvConfig["pluginNameList"]
	// Example:
	// config.SubSectionValue = "trc-plugin-vault"
	// Note: Code from trcplgtool
	// config := &eUtils.DriverConfig{Insecure: *insecurePtr, Log: logger, ExitOnFailure: true, StartDir: []string{*startDirPtr}, SubSectionValue: *pluginNamePtr}

	// 1. For each plugin do the following:
	// Assert: we already have a plugin name
	// 1a. retrieve TrcVault/trcplugin/<theplugin>/Certify/trcsha256
	// 1b. Read and sha256 of /etc/opt/vault/plugins/<theplugin>
	// 1c. if vault sha256 != filesystem sha256.
	// 1.c.i. Download new image from ECR.
	// 1.c.ii. Sha256 of new executable.
	// 1.c.ii.- if Sha256 of new executable === sha256 from vault.
	//  Save new image over existing image in /etc/opt/vault/plugins/<theplugin>
	// 2a. Update vault setting copied=true...
	// 3. Update apiChannel so api returns true
	logger.Println("PluginDeployFlow complete.")

	return nil
}

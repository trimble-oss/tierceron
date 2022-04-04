package deploy

import (
	"log"
	"strings"
	"tierceron/trcvault/util"

	sys "tierceron/vaulthelper/system"

	eUtils "tierceron/utils"
	helperkv "tierceron/vaulthelper/kv"

	"github.com/davecgh/go-spew/spew"
)

func PluginDeployFlow(pluginConfig map[string]interface{}, logger *log.Logger) error {
	var config *eUtils.DriverConfig
	var vault *sys.Vault
	var goMod *helperkv.Modifier
	var err error

	//Grabbing configs
	config, goMod, vault, err = eUtils.InitVaultModForPlugin(pluginConfig, logger)
	if err != nil {
		eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start.", false)
		return err
	}

	pluginToolConfig := util.GetPluginToolConfig(config, goMod, pluginConfig)
	// This should come from vault now....
	pluginToolConfig["ecrrepository"] = strings.Replace(pluginToolConfig["ecrrepository"].(string), "__imagename__", "trc-vault-plugin", -1) //"https://" +
	pluginToolConfig["trcsha256"] = "*sha256Ptr"
	pluginToolConfig["pluginNamePtr"] = "pluginNamePtr"

	// This should come from vault
	// 0. List all the plugins under Index/TrcVault/trcplugin
	// Example:
	// config.SubSectionValue = "trc-plugin-vault"
	// Note: Code from trcplgtool
	// config := &eUtils.DriverConfig{Insecure: *insecurePtr, Log: logger, ExitOnFailure: true, StartDir: []string{*startDirPtr}, SubSectionValue: *pluginNamePtr}

	spew.Dump(pluginToolConfig)
	spew.Dump(vault)

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

	return nil
}

package deploy

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"regexp"
	"strconv"
	"strings"

	"github.com/trimble-oss/tierceron/atrium/vestibulum/pluginutil"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/factory"
	trcplgtool "github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/trcplgtoolbase"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/pkg/core/cache"
	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"
	"github.com/trimble-oss/tierceron/pkg/core/util/repository"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	sys "github.com/trimble-oss/tierceron/pkg/vaulthelper/system"
	"kernel.org/pub/linux/libs/security/libcap/cap"

	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	//"kernel.org/pub/linux/libs/security/libcap/cap"
)

func init() {
	factory.StartPluginSettingEater()
}

func PluginDeployEnvFlow(pluginConfig map[string]interface{}, logger *log.Logger) error {
	logger.Println("PluginDeployInitFlow begun.")
	var err error
	var driverConfig *config.DriverConfig
	var goMod *helperkv.Modifier
	var vault *sys.Vault

	//Grabbing configs
	tempAddr := pluginConfig["vaddress"]
	tempTokenPtr := pluginConfig["tokenptr"]
	pluginConfig["vaddress"] = pluginConfig["caddress"]
	pluginConfig["tokenptr"] = pluginConfig["ctokenptr"]
	driverConfig, goMod, vault, err = eUtils.InitVaultModForPlugin(pluginConfig,
		cache.NewTokenCache("config_token_pluginany", eUtils.RefMap(pluginConfig, "tokenptr")),
		"config_token_pluginany", logger)
	if vault != nil {
		defer vault.Close()
	}

	pluginutil.PluginInitNewRelic(driverConfig, goMod, pluginConfig)
	logger = driverConfig.CoreConfig.Log

	if goMod != nil {
		defer goMod.Release()
	}
	pluginConfig["vaddress"] = tempAddr
	pluginConfig["tokenptr"] = tempTokenPtr

	if err != nil {
		eUtils.LogErrorMessage(driverConfig.CoreConfig, "Could not access vault.  Failure to start.", false)
		return err
	}

	pluginutil.TapFeatherInit(driverConfig, goMod, pluginConfig, false, logger)

	logger.Println("PluginDeployInitFlow complete.")

	return err
}

func PluginDeployFlow(pluginConfig map[string]interface{}, logger *log.Logger) error {
	logger.Println("PluginDeployFlow begun.")
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
	if eUtils.RefEquals(eUtils.RefMap(pluginConfig, "ctokenptr"), "") { //if no certification address found, it will try to certify against itself.
		return errors.New("could not find certification token")
	}

	addr := pluginConfig["vaddress"].(string)
	addrPtr := &addr
	tokenPtr := eUtils.RefMap(pluginConfig, "tokenptr")
	pluginConfig["vaddress"] = pluginConfig["caddress"]
	pluginConfig["tokenptr"] = pluginConfig["ctokenptr"]
	carrierDriverConfig, cGoMod, _, err := eUtils.InitVaultModForPlugin(pluginConfig,
		cache.NewTokenCache("config_token_pluginany", eUtils.RefMap(pluginConfig, "tokenptr")),
		"config_token_pluginany", logger)
	carrierDriverConfig.SubSectionValue = pluginName
	if err != nil {
		eUtils.LogErrorMessage(carrierDriverConfig.CoreConfig, "Could not access vault.  Failure to start.", false)
		return err
	}

	vaultPluginSignature, ptcErr := trcvutils.GetPluginToolConfig(carrierDriverConfig, cGoMod, pluginConfig, false)

	defer func(vaddrPtr *string, tPtr *string) {
		pluginConfig["vaddress"] = *vaddrPtr
		pluginConfig["tokenptr"] = *tPtr
	}(addrPtr, tokenPtr)

	if ptcErr != nil {
		eUtils.LogErrorMessage(carrierDriverConfig.CoreConfig, fmt.Sprintf("PluginDeployFlow failure: env: %s plugin load failure: %s", carrierDriverConfig.CoreConfig.Env, ptcErr.Error()), false)
		return nil
	}

	//grabbing configs
	if _, ok := vaultPluginSignature["trcplugin"]; !ok {
		// TODO: maybe delete plugin if it exists since there was no entry in vault...
		eUtils.LogErrorMessage(carrierDriverConfig.CoreConfig, fmt.Sprintf("PluginDeployFlow failure: env: %s plugin status load failure.", carrierDriverConfig.CoreConfig.Env), false)
		return nil
	}

	if _, ok := vaultPluginSignature["acrrepository"].(string); !ok {
		// TODO: maybe delete plugin if it exists since there was no entry in vault...
		eUtils.LogErrorMessage(carrierDriverConfig.CoreConfig, fmt.Sprintf("PluginDeployFlow failure: env: %s plugin status load failure - no certification entry found.", carrierDriverConfig.CoreConfig.Env), false)
		return nil
	}

	if _, ok := vaultPluginSignature["trcsha256"].(string); !ok {
		// TODO: maybe delete plugin if it exists since there was no entry in vault...
		eUtils.LogErrorMessage(carrierDriverConfig.CoreConfig, fmt.Sprintf("PluginDeployFlow failure: env: %s plugin status load failure - incomplete certification entry found.  Missing trcsha256.", carrierDriverConfig.CoreConfig.Env), false)
		return nil
	}

	//Checks if this instance of carrier is allowed to deploy that certain plugin.
	if instanceList, ok := vaultPluginSignature["instances"].(string); !ok {
		eUtils.LogErrorMessage(carrierDriverConfig.CoreConfig, fmt.Sprintf("PluginDeployFlow failure: env: %s Plugin has no valid instances: %s", carrierDriverConfig.CoreConfig.Env, vaultPluginSignature["trcplugin"].(string)), false)
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
				eUtils.LogErrorMessage(carrierDriverConfig.CoreConfig, fmt.Sprintf("Plugin %s not found for env: %s and this instance: %s\n", vaultPluginSignature["trcplugin"].(string), carrierDriverConfig.CoreConfig.Env, instanceIndex), false)
				vaultPluginSignature["trcsha256"] = "notfound"
				factory.PushPluginSha(carrierDriverConfig, pluginConfig, vaultPluginSignature)
				return nil
			}
		} else {
			eUtils.LogErrorMessage(carrierDriverConfig.CoreConfig, fmt.Sprintf("Unable to determine for env: %s this instance: %s index for deployment.  Error: %s\n", carrierDriverConfig.CoreConfig.Env, vaultPluginSignature["trcplugin"].(string), hostNameErr.Error()), false)
			return nil
		}
	}

	if deployedVal, ok := vaultPluginSignature["deployed"].(bool); ok && deployedVal {
		eUtils.LogErrorMessage(carrierDriverConfig.CoreConfig, fmt.Sprintf("Plugin has already been deployed env: %s and copied: %s\n", carrierDriverConfig.CoreConfig.Env, vaultPluginSignature["trcplugin"].(string)), false)
		return nil
	}

	pluginDownloadNeeded := false
	pluginCopied := false
	var agentPath string

	// trcsh is always type agent... even if it somehow ends up incorrect in vault...
	if vaultPluginSignature["trcplugin"].(string) == "trcsh" {
		vaultPluginSignature["trctype"] = "agent"
	}

	switch vaultPluginSignature["trctype"] {
	case "agent":
		agentPath = "/home/azuredeploy/bin/" + vaultPluginSignature["trcplugin"].(string)
	default:
		agentPath = coreopts.BuildOptions.GetVaultInstallRoot() + "/plugins/" + vaultPluginSignature["trcplugin"].(string)
	}

	if _, err := os.Stat(agentPath); errors.Is(err, os.ErrNotExist) {
		pluginDownloadNeeded = true
		logger.Printf("Attempting to download new image for env: %s and plugin %s\n", carrierDriverConfig.CoreConfig.Env, vaultPluginSignature["trcplugin"].(string))
	} else {
		if imageFile, err := os.Open(agentPath); err == nil {
			logger.Printf("Found image for env: %s and plugin %s\n", carrierDriverConfig.CoreConfig.Env, vaultPluginSignature["trcplugin"].(string))

			sha256 := sha256.New()

			defer imageFile.Close()
			if _, err := io.Copy(sha256, imageFile); err != nil {
				eUtils.LogErrorMessage(carrierDriverConfig.CoreConfig, fmt.Sprintf("PluginDeployFlow failure: Could not sha256 image from file system for env: %s and plugin %s\n", carrierDriverConfig.CoreConfig.Env, vaultPluginSignature["trcplugin"].(string)), false)
				return err
			}

			filesystemsha256 := fmt.Sprintf("%x", sha256.Sum(nil))
			if filesystemsha256 != vaultPluginSignature["trcsha256"] { //Sha256 from file system matches in vault
				pluginDownloadNeeded = true
				logger.Printf("Attempting to download new image for env: %s and plugin %s\n", carrierDriverConfig.CoreConfig.Env, vaultPluginSignature["trcplugin"].(string))
			} else {
				eUtils.LogErrorMessage(carrierDriverConfig.CoreConfig, fmt.Sprintf("Certified plugin already exists on file system - continuing with vault plugin status update for env: %s and plugin %s\n", carrierDriverConfig.CoreConfig.Env, vaultPluginSignature["trcplugin"].(string)), false)
			}
		} else {
			logger.Printf("Cannup update new image for env: %s and plugin %s Error: %s\n", carrierDriverConfig.CoreConfig.Env, vaultPluginSignature["trcplugin"].(string), err.Error())
		}
	}

	if pluginDownloadNeeded {
		logger.Printf("PluginDeployFlow new plugin image found for env: %s and plugin %s\n", carrierDriverConfig.CoreConfig.Env, pluginName)

		// 1.c.i. Download new image from ECR.
		// 1.c.ii. Sha256 of new executable.
		// 1.c.ii.- if Sha256 of new executable === sha256 from vault.
		downloadErr := repository.GetImageAndShaFromDownload(carrierDriverConfig, vaultPluginSignature)
		if downloadErr != nil {
			eUtils.LogErrorMessage(carrierDriverConfig.CoreConfig, fmt.Sprintf("Could not get download image for env: %s and plugin %s error: %s\n", carrierDriverConfig.CoreConfig.Env, pluginName, downloadErr.Error()), false)
			vaultPluginSignature["imagesha256"] = "invalidurl"
			return downloadErr
		}
		if vaultPluginSignature["imagesha256"] == vaultPluginSignature["trcsha256"] { //Sha256 from download matches in vault
			logger.Printf("PluginDeployFlow updating new image for env: %s and plugin %s\n", carrierDriverConfig.CoreConfig.Env, pluginName)
			err = os.WriteFile(agentPath, vaultPluginSignature["rawImageFile"].([]byte), 0644)
			vaultPluginSignature["rawImageFile"] = nil

			if err != nil {
				eUtils.LogErrorMessage(carrierDriverConfig.CoreConfig, fmt.Sprintf("PluginDeployFlow failure: Could not write out download image for env: %s and plugin %s error: %s\n", carrierDriverConfig.CoreConfig.Env, pluginName, downloadErr.Error()), false)
				return err
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
				defer imageFile.Close()
				chdModErr := imageFile.Chmod(0750)
				if chdModErr != nil {
					eUtils.LogErrorMessage(carrierDriverConfig.CoreConfig, fmt.Sprintf("PluginDeployFlow failure: Could not give permission to image in file system.  Bailing.. for env: %s and plugin %s\n", carrierDriverConfig.CoreConfig.Env, pluginName), false)
					return nil
				}
			} else {
				eUtils.LogErrorMessage(carrierDriverConfig.CoreConfig, fmt.Sprintf("PluginDeployFlow failure: Could not open image in file system to give permissions for env: %s and plugin %s\n", carrierDriverConfig.CoreConfig.Env, pluginName), false)
				return nil
			}

			ipcLockCapSet, err := cap.FromText("cap_ipc_lock=+ep")
			if err != nil {
				eUtils.LogErrorMessage(carrierDriverConfig.CoreConfig, fmt.Sprintf("PluginDeployFlow failure: Could not set needed capabilities for env: %s and plugin %s error: %s\n", carrierDriverConfig.CoreConfig.Env, pluginName, err.Error()), false)
			}
			ipcLockErr := ipcLockCapSet.SetFile(agentPath)
			if ipcLockErr != nil {
				eUtils.LogErrorMessage(carrierDriverConfig.CoreConfig, fmt.Sprintf("PluginDeployFlow failure: Could not apply needed capabilities for env: %s and plugin %s error: %s\n", carrierDriverConfig.CoreConfig.Env, pluginName, ipcLockErr.Error()), false)
			}

			pluginCopied = true
			eUtils.LogInfo(carrierDriverConfig.CoreConfig, fmt.Sprintf("Image has been copied for env: %s and plugin %s\n", carrierDriverConfig.CoreConfig.Env, pluginName))
		} else {
			logger.Printf("mismatched")
			imgsha := "notlatest or notfound"
			if _, okImg := vaultPluginSignature["imagesha256"]; okImg {
				imgsha = vaultPluginSignature["imagesha256"].(string)
			}
			eUtils.LogErrorMessage(carrierDriverConfig.CoreConfig, fmt.Sprintf("env: %s plugin: %s: PluginDeployFlow failure: Refusing to copy since vault certification does not match plugin sha256 signature.  Downloaded: %s, Expected: %s", carrierDriverConfig.CoreConfig.Env, pluginName, imgsha, vaultPluginSignature["trcsha256"]), false)
		}
	}

	if (!pluginDownloadNeeded && !pluginCopied) || (pluginDownloadNeeded && pluginCopied) { // No download needed because it's already there, but vault may be wrong.
		if vaultPluginSignature["copied"].(bool) && !vaultPluginSignature["deployed"].(bool) { //If status hasn't changed, don't update
			eUtils.LogInfo(carrierDriverConfig.CoreConfig, fmt.Sprintf("Not updating plugin image to vault as status is the same for env: %s and plugin: %s\n", carrierDriverConfig.CoreConfig.Env, pluginName))
		}

		eUtils.LogInfo(carrierDriverConfig.CoreConfig, pluginName+": Updating plugin image to vault.")
		if pluginSHA, pluginSHAOk := vaultPluginSignature["trcsha256"]; !pluginSHAOk || pluginSHA.(string) == "" {
			eUtils.LogInfo(carrierDriverConfig.CoreConfig, fmt.Sprintf("Plugin is not registered with carrier for env: %s and plugin: %s\n", carrierDriverConfig.CoreConfig.Env, pluginName))
			return nil
		}
		eUtils.LogInfo(carrierDriverConfig.CoreConfig, pluginName+": Checkpush sha256")
		factory.PushPluginSha(carrierDriverConfig, pluginConfig, vaultPluginSignature)
		eUtils.LogInfo(carrierDriverConfig.CoreConfig, pluginName+": End checkpush sha256")

		writeMap, err := cGoMod.ReadData(fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", pluginName))

		if err != nil {
			eUtils.LogInfo(carrierDriverConfig.CoreConfig, pluginName+": Initializing certification")
			writeMap = make(map[string]interface{})
		} else {
			eUtils.LogInfo(carrierDriverConfig.CoreConfig, pluginName+": Updating certification status")
		}

		if trcType, trcTypeOk := vaultPluginSignature["trctype"]; trcTypeOk {
			writeMap["trctype"] = trcType.(string)
		} else {
			writeMap["trctype"] = "vault"
		}
		writeMap = trcplgtool.WriteMapUpdate(writeMap, vaultPluginSignature, false, writeMap["trctype"].(string), "")
		if writeMap["trctype"].(string) == "agent" {
			writeMap["deployed"] = true
		}
		cGoMod.SectionPath = ""
		_, err = cGoMod.Write(fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", writeMap["trcplugin"].(string)), writeMap, carrierDriverConfig.CoreConfig.Log)
		if err != nil {
			logger.Printf("PluginDeployFlow failure: Failed to write plugin state for env: %s and plugin: %s error: %s\n", carrierDriverConfig.CoreConfig.Env, pluginName, err.Error())
		}
		if hostName != "" && writeMap["trctype"].(string) == "agent" {
			overridePath := "overrides/" + hostName + "/" + writeMap["trcplugin"].(string) + "/Certify"
			_, err = cGoMod.Write("super-secrets/Index/TrcVault/trcplugin/"+overridePath, writeMap, carrierDriverConfig.CoreConfig.Log)
			if err != nil {
				logger.Println(pluginName + ": PluginDeployFlow failure: Failed to write plugin state: " + err.Error())
			}
		}

		eUtils.LogInfo(carrierDriverConfig.CoreConfig, fmt.Sprintf("Plugin image config in vault has been updated for env: %s and plugin: %s\n", carrierDriverConfig.CoreConfig.Env, pluginName))
	} else {
		if !pluginDownloadNeeded && pluginCopied {
			eUtils.LogInfo(carrierDriverConfig.CoreConfig, fmt.Sprintf("Not updating plugin image to vault as status is the same for  for env: %s and plugin: %s\n", carrierDriverConfig.CoreConfig.Env, pluginName))
			// Already copied... Just echo back the sha256...
		}
	}
	// ALways set this so it completes if there is a sha256 available...
	// This will also release any clients attempting to communicate with carrier.
	factory.PushPluginSha(carrierDriverConfig, pluginConfig, vaultPluginSignature)

	logger.Println("PluginDeployFlow complete.")

	return nil
}

// Updated deployed to true for any plugin
func PluginDeployedUpdate(driverConfig *config.DriverConfig, mod *helperkv.Modifier, vault *sys.Vault, pluginNameList []string, cPath []string, logger *log.Logger) error {
	logger.Println("PluginDeployedUpdate start.")

	hostName, hostNameErr := os.Hostname()
	if hostNameErr != nil {
		return hostNameErr
	} else if hostName == "" {
		return errors.New("could not find hostname")
	}

	hostRegion := coreopts.BuildOptions.GetRegion(hostName)
	if hostRegion == "" {
		logger.Println("PluginDeployedUpdate self certification not provided on base region deployers")
		return nil
	}

	mod.Regions = append(mod.Regions, hostRegion)
	projects, services, _ := eUtils.GetProjectServices(nil, cPath)
	for _, pluginName := range pluginNameList {
		for i := 0; i < len(projects); i++ {
			if services[i] == "Certify" {
				mod.SectionName = "trcplugin"
				mod.SectionKey = "/Index/"
				mod.SubSectionValue = pluginName

				properties, err := trcvutils.NewProperties(driverConfig.CoreConfig, vault, mod, driverConfig.CoreConfig.Env, projects[i], services[i])
				if err != nil {
					return err
				}

				pluginData, replacedFields := properties.GetPluginData(hostRegion, services[i], "config", driverConfig.CoreConfig.Log)
				if pluginData == nil {
					pluginData = make(map[string]interface{})
					pluginData["trcplugin"] = pluginName

					var agentPath string

					if pluginData["trctype"] == "agent" {
						agentPath = "/home/azuredeploy/bin/" + pluginName
					} else {
						agentPath = coreopts.BuildOptions.GetVaultInstallRoot() + "/plugins/" + pluginName
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

				if hostRegion != "" {
					pluginData["deployed"] = true //Update deploy status if region exist otherwise this will block regionless deploys if set for regionless status
				}
				statusUpdateErr := properties.WritePluginData(pluginData, replacedFields, mod, driverConfig.CoreConfig.Log, hostRegion, pluginName)
				if statusUpdateErr != nil {
					return statusUpdateErr
				}

			}
		}
	}
	logger.Println("PluginDeployedUpdate complete.")
	return nil
}

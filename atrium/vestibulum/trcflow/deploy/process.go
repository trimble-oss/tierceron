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

	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/carrierfactory/servercapauth"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/factory"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/opts/prod"
	trcplgtool "github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/trcplgtoolbase"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"
	"github.com/trimble-oss/tierceron/pkg/core/util/repository"
	sys "github.com/trimble-oss/tierceron/pkg/vaulthelper/system"

	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	//"kernel.org/pub/linux/libs/security/libcap/cap"
)

func init() {
	factory.StartPluginSettingEater()
}

var onceAuth sync.Once
var gCapInitted bool = false

func IsCapInitted() bool { return gCapInitted }

func PluginDeployEnvFlow(pluginConfig map[string]interface{}, logger *log.Logger) error {
	logger.Println("PluginDeployInitFlow begun.")
	var err error
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
		return err
	}

	if ok, err := servercapauth.ValidatePathSha(goMod, pluginConfig, logger); ok {
		// Only start up if trcsh is up to date....
		onceAuth.Do(func() {
			if pluginConfig["env"].(string) == "dev" || pluginConfig["env"].(string) == "staging" {
				// Ensure only dev is the cap auth...
				logger.Printf("Cap auth init for env: %s\n", pluginConfig["env"].(string))
				var featherAuth *servercapauth.FeatherAuth = nil
				if pluginConfig["env"].(string) == "dev" {
					featherAuth, err = servercapauth.Init(goMod, pluginConfig, logger)
					if err != nil {
						eUtils.LogErrorMessage(config, "Skipping cap auth init.", false)
						return
					}
					pluginConfig["trcHatSecretsPort"] = featherAuth.SecretsPort
				}

				servercapauth.Memorize(pluginConfig, logger)

				// Not really clear how cap auth would do this...
				if featherAuth != nil {
					go servercapauth.Start(featherAuth, pluginConfig["env"].(string), logger)
				}
				logger.Printf("Cap auth init complete for env: %s\n", pluginConfig["env"].(string))
				gCapInitted = true
			}
		})
	} else {
		eUtils.LogErrorMessage(config, fmt.Sprintf("Mismatched sha256 cap auth for env: %s.  Skipping.", pluginConfig["env"].(string)), false)
	}

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
	if pluginConfig["ctoken"].(string) == "" { //if no certification address found, it will try to certify against itself.
		return errors.New("could not find certification token")
	}

	addrPtr := pluginConfig["vaddress"].(string)
	tokPtr := pluginConfig["token"].(string)
	pluginConfig["vaddress"] = pluginConfig["caddress"]
	pluginConfig["token"] = pluginConfig["ctoken"]
	cConfig, cGoMod, _, err := eUtils.InitVaultModForPlugin(pluginConfig, logger)
	cConfig.SubSectionValue = pluginName
	if err != nil {
		eUtils.LogErrorMessage(cConfig, "Could not access vault.  Failure to start.", false)
		return err
	}

	vaultPluginSignature, ptcErr := trcvutils.GetPluginToolConfig(cConfig, cGoMod, pluginConfig, false)

	defer func(vaddrPtr *string, tPtr *string) {
		pluginConfig["vaddress"] = *vaddrPtr
		pluginConfig["token"] = *tPtr
	}(&addrPtr, &tokPtr)

	if ptcErr != nil {
		eUtils.LogErrorMessage(cConfig, fmt.Sprintf("PluginDeployFlow failure: env: %s plugin load failure: %s", cConfig.Env, ptcErr.Error()), false)
		return nil
	}

	//grabbing configs
	if _, ok := vaultPluginSignature["trcplugin"]; !ok {
		// TODO: maybe delete plugin if it exists since there was no entry in vault...
		eUtils.LogErrorMessage(cConfig, fmt.Sprintf("PluginDeployFlow failure: env: %s plugin status load failure.", cConfig.Env), false)
		return nil
	}

	if _, ok := vaultPluginSignature["acrrepository"].(string); !ok {
		// TODO: maybe delete plugin if it exists since there was no entry in vault...
		eUtils.LogErrorMessage(cConfig, fmt.Sprintf("PluginDeployFlow failure: env: %s plugin status load failure - no certification entry found.", cConfig.Env), false)
		return nil
	}

	//Checks if this instance of carrier is allowed to deploy that certain plugin.
	if instanceList, ok := vaultPluginSignature["instances"].(string); !ok {
		eUtils.LogErrorMessage(cConfig, fmt.Sprintf("PluginDeployFlow failure: env: %s Plugin has no valid instances: %s", cConfig.Env, vaultPluginSignature["trcplugin"].(string)), false)
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
				eUtils.LogErrorMessage(cConfig, fmt.Sprintf("Plugin %s not found for env: %s and this instance: %s\n", vaultPluginSignature["trcplugin"].(string), cConfig.Env, instanceIndex), false)
				vaultPluginSignature["trcsha256"] = "notfound"
				factory.PushPluginSha(cConfig, pluginConfig, vaultPluginSignature)
				return nil
			}
		} else {
			eUtils.LogErrorMessage(cConfig, fmt.Sprintf("Unable to determine for env: %s this instance: %s index for deployment.  Error: %s\n", cConfig.Env, vaultPluginSignature["trcplugin"].(string), hostNameErr.Error()), false)
			return nil
		}
	}

	if deployedVal, ok := vaultPluginSignature["deployed"].(bool); ok && deployedVal {
		eUtils.LogErrorMessage(cConfig, fmt.Sprintf("Plugin has already been deployed env: %s and copied: %s\n", cConfig.Env, vaultPluginSignature["trcplugin"].(string)), false)
		return nil
	}

	pluginDownloadNeeded := false
	pluginCopied := false
	var agentPath string

	pluginExtension := ""
	if prod.IsProd() {
		pluginExtension = "-prod"
	}

	// trcsh is always type agent... even if it somehow ends up incorrect in vault...
	if vaultPluginSignature["trcplugin"].(string) == "trcsh" {
		vaultPluginSignature["trctype"] = "agent"
	}

	switch vaultPluginSignature["trctype"] {
	case "agent":
		agentPath = "/home/azuredeploy/bin/" + vaultPluginSignature["trcplugin"].(string)
	default:
		agentPath = "/etc/opt/vault/plugins/" + vaultPluginSignature["trcplugin"].(string) + pluginExtension
	}

	if _, err := os.Stat(agentPath); errors.Is(err, os.ErrNotExist) {
		pluginDownloadNeeded = true
		logger.Printf(fmt.Sprintf("Attempting to download new image for env: %s and plugin %s\n", cConfig.Env, vaultPluginSignature["trcplugin"].(string)))
	} else {
		if imageFile, err := os.Open(agentPath); err == nil {
			logger.Printf(fmt.Sprintf("Found image for env: %s and plugin %s\n", cConfig.Env, vaultPluginSignature["trcplugin"].(string)))

			sha256 := sha256.New()

			defer imageFile.Close()
			if _, err := io.Copy(sha256, imageFile); err != nil {
				eUtils.LogErrorMessage(cConfig, fmt.Sprintf("PluginDeployFlow failure: Could not sha256 image from file system for env: %s and plugin %s\n", cConfig.Env, vaultPluginSignature["trcplugin"].(string)), false)
			}

			filesystemsha256 := fmt.Sprintf("%x", sha256.Sum(nil))
			if filesystemsha256 != vaultPluginSignature["trcsha256"] { //Sha256 from file system matches in vault
				pluginDownloadNeeded = true
			} else {
				eUtils.LogErrorMessage(cConfig, fmt.Sprintf("Certified plugin already exists in file system - continuing with vault plugin status update for env: %s and plugin %s\n", cConfig.Env, vaultPluginSignature["trcplugin"].(string)), false)
			}
		} else {
			pluginDownloadNeeded = true
			logger.Printf(fmt.Sprintf("Attempting to download new image for env: %s and plugin %s\n", cConfig.Env, vaultPluginSignature["trcplugin"].(string)))
		}
	}

	if pluginDownloadNeeded {
		logger.Printf(fmt.Sprintf("PluginDeployFlow new plugin image found for env: %s and plugin %s\n", cConfig.Env, pluginName))

		// 1.c.i. Download new image from ECR.
		// 1.c.ii. Sha256 of new executable.
		// 1.c.ii.- if Sha256 of new executable === sha256 from vault.
		downloadErr := repository.GetImageAndShaFromDownload(cConfig, vaultPluginSignature)
		if downloadErr != nil {
			eUtils.LogErrorMessage(cConfig, fmt.Sprintf("Could not get download image for env: %s and plugin %s error: %s\n", cConfig.Env, pluginName, downloadErr.Error()), false)
			vaultPluginSignature["imagesha256"] = "invalidurl"
		}
		if vaultPluginSignature["imagesha256"] == vaultPluginSignature["trcsha256"] { //Sha256 from download matches in vault
			err = os.WriteFile(agentPath, vaultPluginSignature["rawImageFile"].([]byte), 0644)
			vaultPluginSignature["rawImageFile"] = nil

			if err != nil {
				eUtils.LogErrorMessage(cConfig, fmt.Sprintf("PluginDeployFlow failure: Could not write out download image for env: %s and plugin %s error: %s\n", cConfig.Env, pluginName, downloadErr.Error()), false)
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
					eUtils.LogErrorMessage(cConfig, fmt.Sprintf("PluginDeployFlow failure: Could not give permission to image in file system.  Bailing.. for env: %s and plugin %s\n", cConfig.Env, pluginName), false)
					return nil
				}
			} else {
				eUtils.LogErrorMessage(cConfig, fmt.Sprintf("PluginDeployFlow failure: Could not open image in file system to give permissions for env: %s and plugin %s\n", cConfig.Env, pluginName), false)
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
				eUtils.LogErrorMessage(cConfig, fmt.Sprintf("PluginDeployFlow failure: Could not set needed capabilities for env: %s and plugin %s error: %s: %s\n", cConfig.Env, pluginName, err.Error(), string(output)), false)
			}

			pluginCopied = true
			eUtils.LogInfo(cConfig, fmt.Sprintf("Image has been copied for env: %s and plugin %s\n", cConfig.Env, pluginName))
		} else {
			imgsha := "notlatest or notfound"
			if _, okImg := vaultPluginSignature["imagesha256"]; okImg {
				imgsha = vaultPluginSignature["imagesha256"].(string)
			}
			eUtils.LogErrorMessage(cConfig, fmt.Sprintf("env: %s plugin: %s: PluginDeployFlow failure: Refusing to copy since vault certification does not match plugin sha256 signature.  Downloaded: %s, Expected: %s", cConfig.Env, pluginName, imgsha, vaultPluginSignature["trcsha256"]), false)
		}
	}

	if (!pluginDownloadNeeded && !pluginCopied) || (pluginDownloadNeeded && pluginCopied) { // No download needed because it's already there, but vault may be wrong.
		if vaultPluginSignature["copied"].(bool) && !vaultPluginSignature["deployed"].(bool) { //If status hasn't changed, don't update
			eUtils.LogInfo(cConfig, fmt.Sprintf("Not updating plugin image to vault as status is the same for env: %s and plugin: %s\n", cConfig.Env, pluginName))
		}

		eUtils.LogInfo(cConfig, pluginName+": Updating plugin image to vault.")
		if pluginSHA, pluginSHAOk := vaultPluginSignature["trcsha256"]; !pluginSHAOk || pluginSHA.(string) == "" {
			eUtils.LogInfo(cConfig, fmt.Sprintf("Plugin is not registered with carrier for env: %s and plugin: %s\n", cConfig.Env, pluginName))
			return nil
		}
		eUtils.LogInfo(cConfig, pluginName+": Checkpush sha256")
		factory.PushPluginSha(cConfig, pluginConfig, vaultPluginSignature)
		eUtils.LogInfo(cConfig, pluginName+": End checkpush sha256")

		writeMap, err := cGoMod.ReadData(fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", pluginName))

		if err != nil {
			eUtils.LogInfo(cConfig, pluginName+": Initializing certification")
			writeMap = make(map[string]interface{})
		} else {
			eUtils.LogInfo(cConfig, pluginName+": Updating certification status")
		}

		if trcType, trcTypeOk := vaultPluginSignature["trctype"]; trcTypeOk {
			writeMap["trctype"] = trcType.(string)
		} else {
			writeMap["trctype"] = "vault"
		}
		writeMap = trcplgtool.WriteMapUpdate(writeMap, vaultPluginSignature, false, writeMap["trctype"].(string))
		if writeMap["trctype"].(string) == "agent" {
			writeMap["deployed"] = true
		}
		cGoMod.SectionPath = ""
		_, err = cGoMod.Write(fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", writeMap["trcplugin"].(string)), writeMap, cConfig.Log)
		if err != nil {
			logger.Printf(fmt.Sprintf("PluginDeployFlow failure: Failed to write plugin state for env: %s and plugin: %s error: %s\n", cConfig.Env, pluginName, err.Error()))
		}
		eUtils.LogInfo(cConfig, fmt.Sprintf("Plugin image config in vault has been updated for env: %s and plugin: %s\n", cConfig.Env, pluginName))
	} else {
		if !pluginDownloadNeeded && pluginCopied {
			eUtils.LogInfo(cConfig, fmt.Sprintf("Not updating plugin image to vault as status is the same for  for env: %s and plugin: %s\n", cConfig.Env, pluginName))
			// Already copied... Just echo back the sha256...
		}
	}
	// ALways set this so it completes if there is a sha256 available...
	// This will also release any clients attempting to communicate with carrier.
	factory.PushPluginSha(cConfig, pluginConfig, vaultPluginSignature)

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
		return errors.New("could not find hostname")
	}

	hostRegion := coreopts.BuildOptions.GetRegion(hostName)
	mod.Regions = append(mod.Regions, hostRegion)
	projects, services, _ := eUtils.GetProjectServices(cPath)
	for _, pluginName := range pluginNameList {
		for i := 0; i < len(projects); i++ {
			if services[i] == "Certify" {
				mod.SectionName = "trcplugin"
				mod.SectionKey = "/Index/"
				mod.SubSectionValue = pluginName

				properties, err := trcvutils.NewProperties(config, vault, mod, config.Env, projects[i], services[i])
				if err != nil {
					return err
				}

				pluginData, replacedFields := properties.GetPluginData(hostRegion, services[i], "config", config.Log)
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

				if hostRegion != "" {
					pluginData["deployed"] = true //Update deploy status if region exist otherwise this will block regionless deploys if set for regionless status
				}
				statusUpdateErr := properties.WritePluginData(pluginData, replacedFields, mod, config.Log, hostRegion, pluginName)
				if err != nil {
					return statusUpdateErr
				}

			}
		}
	}
	logger.Println("PluginDeployedUpdate complete.")
	return nil
}

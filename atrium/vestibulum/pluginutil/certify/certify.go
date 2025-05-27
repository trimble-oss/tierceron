package certify

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/pkg/utils/config"

	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	sys "github.com/trimble-oss/tierceron/pkg/vaulthelper/system"
)

func WriteMapUpdate(writeMap map[string]interface{}, pluginToolConfig map[string]interface{}, defineServicePtr bool, pluginTypePtr string, pathParamPtr string) map[string]interface{} {
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
		writeMap["trcprojectservice"] = pluginToolConfig["trcprojectservice"].(string)
		writeMap["trcdeployroot"] = pluginToolConfig["trcdeployroot"].(string)
	}
	if _, imgShaOk := pluginToolConfig["imagesha256"].(string); imgShaOk {
		writeMap["trcsha256"] = pluginToolConfig["imagesha256"].(string) // Pull image sha from registry...
	} else {
		writeMap["trcsha256"] = pluginToolConfig["trcsha256"].(string) // Pull image sha from registry...
	}
	if pathParamPtr != "" { //optional if not found.
		writeMap["trcpathparam"] = pathParamPtr
	} else if pathParam, pathOK := writeMap["trcpathparam"].(string); pathOK {
		writeMap["trcpathparam"] = pathParam
	}

	if newRelicAppName, nameOK := pluginToolConfig["newrelicAppName"].(string); newRelicAppName != "" && nameOK && pluginTypePtr == "vault" { //optional if not found.
		writeMap["newrelic_app_name"] = newRelicAppName
	}
	if newRelicLicenseKey, keyOK := pluginToolConfig["newrelicLicenseKey"].(string); newRelicLicenseKey != "" && keyOK && pluginTypePtr == "vault" { //optional if not found.
		writeMap["newrelic_license_key"] = newRelicLicenseKey
	}

	if trcbootstrap, ok := pluginToolConfig["trcbootstrap"].(string); ok {
		writeMap["trcbootstrap"] = trcbootstrap
	}

	writeMap["copied"] = false
	writeMap["deployed"] = false
	return writeMap
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

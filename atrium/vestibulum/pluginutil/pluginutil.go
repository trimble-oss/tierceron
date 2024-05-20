package pluginutil

import (
	"errors"
	"log"
	"os"

	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/logWriter"
	"github.com/newrelic/go-agent/v3/newrelic"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

func GetPluginCertifyMap(mod *kv.Modifier, pluginConfig map[string]interface{}) (map[string]interface{}, error) {
	if pluginName, ok := pluginConfig["pluginName"].(string); ok && pluginName != "" {
		certifyMap, err := mod.ReadData("super-secrets/Index/TrcVault/trcplugin/" + pluginName + "/Certify")
		if err != nil {
			return nil, err
		}
		return certifyMap, nil
	}
	return nil, errors.New("missing plugin name for configuration")
}

func PluginInitNewRelic(driverConfig *eUtils.DriverConfig, mod *kv.Modifier, pluginConfig map[string]interface{}) {
	certifyConfig, certifyErr := GetPluginCertifyMap(mod, pluginConfig)
	if certifyErr == nil {
		driverConfig.CoreConfig.Log.Printf("Found certification for plugin: %s Env: %s", pluginConfig["pluginName"], pluginConfig["env"])
		if newrelic_app_name, ok := certifyConfig["newrelic_app_name"].(string); ok && len(newrelic_app_name) > 0 {
			if newrelicLicenseKey, ok := certifyConfig["newrelic_license_key"].(string); ok {
				driverConfig.CoreConfig.Log.Println("Setting up newrelic...")
				app, err := newrelic.NewApplication(
					newrelic.ConfigAppName(newrelic_app_name),
					newrelic.ConfigLicense(newrelicLicenseKey),
					newrelic.ConfigDistributedTracerEnabled(true),
					newrelic.ConfigAppLogForwardingEnabled(true),
					newrelic.ConfigDebugLogger(os.Stdout),
					newrelic.ConfigInfoLogger(os.Stdout),
				)

				if err != nil {
					driverConfig.CoreConfig.Log.Println("Error setting up newrelic:", err)
					os.Exit(-1)
				}

				driverConfig.CoreConfig.Log = log.New(logWriter.New(driverConfig.CoreConfig.Log.Writer(), app), "["+pluginConfig["pluginName"].(string)+"]", log.LstdFlags)
				driverConfig.CoreConfig.Log.Println("Newrelic configured...")
			} else {
				driverConfig.CoreConfig.Log.Println("Missing license key for newrelic.  Continue without newrelic.")
			}
		} else {
			driverConfig.CoreConfig.Log.Println("Missing app name for newrelic.  Continue without newrelic.")
		}
	} else {
		driverConfig.CoreConfig.Log.Println("No pluginName provided for newrelic configuration.  Continue without newrelic.")
	}
}

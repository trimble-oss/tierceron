package pluginutil

import (
	"errors"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/logWriter"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/trimble-oss/tierceron-hat/cap/tap"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trccarrier/carrierfactory/servercapauth"
	"github.com/trimble-oss/tierceron/buildopts/cursoropts"
	"github.com/trimble-oss/tierceron/pkg/capauth"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	sys "github.com/trimble-oss/tierceron/pkg/vaulthelper/system"

	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

func GetPluginCertifyMap(mod *helperkv.Modifier, pluginConfig map[string]interface{}) (map[string]interface{}, error) {
	if pluginName, ok := pluginConfig["pluginName"].(string); ok && pluginName != "" {
		certifyMap, err := mod.ReadData("super-secrets/Index/TrcVault/trcplugin/" + pluginName + "/Certify")
		if err != nil {
			return nil, err
		}
		return certifyMap, nil
	}
	return nil, errors.New("missing plugin name for configuration")
}

func PluginInitNewRelic(driverConfig *eUtils.DriverConfig, mod *helperkv.Modifier, pluginConfig map[string]interface{}) {
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

var onceAuth sync.Once
var gCapInitted bool = false

func IsCapInitted() bool { return gCapInitted }

func PluginTapFeatherInit(trcshDriverConfig *capauth.TrcshDriverConfig, pluginConfig map[string]interface{}) error {
	var goMod *helperkv.Modifier
	var vault *sys.Vault
	var err error

	//Grabbing configs
	tempAddr := pluginConfig["vaddress"]
	tempToken := pluginConfig["token"]
	pluginConfig["vaddress"] = pluginConfig["caddress"]
	pluginConfig["token"] = pluginConfig["ctoken"]

	trcshDriverConfig.DriverConfig, goMod, vault, err = eUtils.InitVaultModForPlugin(pluginConfig, trcshDriverConfig.DriverConfig.CoreConfig.Log)
	if vault != nil {
		defer vault.Close()
	}

	if goMod != nil {
		defer goMod.Release()
	}
	pluginConfig["vaddress"] = tempAddr
	pluginConfig["token"] = tempToken

	if err != nil {
		eUtils.LogErrorMessage(&trcshDriverConfig.DriverConfig.CoreConfig, "Could not access vault.  Failure to start.", true)
		return err
	}
	return TapFeatherInit(trcshDriverConfig.DriverConfig, goMod, pluginConfig, true, trcshDriverConfig.DriverConfig.CoreConfig.Log)
}

func TapFeatherInit(driverConfig *eUtils.DriverConfig, mod *helperkv.Modifier, pluginConfig map[string]interface{}, wantsFeathering bool, logger *log.Logger) error {
	var err error
	var ok bool

	if ok, err = servercapauth.ValidateTrcshPathSha(mod, pluginConfig, logger); ok {
		// Only start up if trcsh is up to date....
		onceAuth.Do(func() {
			if pluginConfig["env"].(string) == "dev" || pluginConfig["env"].(string) == "staging" {
				// Ensure only dev is the cap auth...
				logger.Printf("Cap auth init for env: %s\n", pluginConfig["env"].(string))
				tap.TapInit(cursoropts.BuildOptions.GetCapPath())
				servercapauth.Memorize(pluginConfig, logger)

				var featherAuth *servercapauth.FeatherAuth = nil
				if pluginConfig["env"].(string) == "dev" || pluginConfig["env"].(string) == "staging" {
					featherAuth, err = servercapauth.Init(mod, pluginConfig, wantsFeathering, logger)
					if err != nil {
						eUtils.LogErrorMessage(&driverConfig.CoreConfig, "Skipping cap auth init.", false)
						return
					}
					if featherAuth != nil {
						pluginConfig["trcHatSecretsPort"] = featherAuth.SecretsPort
					}
				}

				if wantsFeathering {
					// Not really clear how cap auth would do this...
					go servercapauth.Start(featherAuth, pluginConfig["env"].(string), logger)
					logger.Printf("Cap auth feather init complete for env: %s\n", pluginConfig["env"].(string))
				}

				logger.Printf("Cap auth init complete for env: %s\n", pluginConfig["env"].(string))
				return
			}
		})
	} else {
		err := fmt.Errorf("mismatched sha256 cap auth for env: %s  skipping", pluginConfig["env"].(string))
		eUtils.LogErrorMessage(&driverConfig.CoreConfig, err.Error(), false)
		return err
	}
	return err
}

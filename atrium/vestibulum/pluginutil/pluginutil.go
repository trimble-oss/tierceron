package pluginutil

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/logWriter"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig/cache"
	prod "github.com/trimble-oss/tierceron-core/v2/prod"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trccarrier/carrierfactory/servercapauth"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/pkg/capauth"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	sys "github.com/trimble-oss/tierceron/pkg/vaulthelper/system"

	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

func GetPluginCertifyMap(mod *helperkv.Modifier, pluginConfig map[string]any) (map[string]any, error) {
	if pluginName, ok := pluginConfig["pluginName"].(string); ok && pluginName != "" {
		certifyMap, err := mod.ReadData("super-secrets/Index/TrcVault/trcplugin/" + pluginName + "/Certify")
		if err != nil {
			return nil, err
		}
		return certifyMap, nil
	}
	return nil, errors.New("missing plugin name for configuration")
}

func PluginInitNewRelic(driverConfig *config.DriverConfig, mod *helperkv.Modifier, pluginConfig map[string]any) {
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

var (
	onceAuth    sync.Once
	gCapInitted bool = false
)

func IsCapInitted() bool { return gCapInitted }

func PluginTapFeatherInit(trcshDriverConfig *capauth.TrcshDriverConfig, pluginConfig map[string]any) error {
	if trcshDriverConfig == nil {
		return errors.New("missing trcsh driver config for feather init")
	}
	if trcshDriverConfig.DriverConfig == nil {
		return errors.New("missing driver config for feather init")
	}
	if trcshDriverConfig.DriverConfig.CoreConfig == nil {
		return errors.New("missing core config for feather init")
	}
	if trcshDriverConfig.DriverConfig.CoreConfig.Log == nil {
		return errors.New("missing core config log for feather init")
	}
	if pluginConfig == nil {
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("PluginTapFeatherInit missing plugin config.")
		return errors.New("missing plugin config for feather init")
	}
	env, envOk := pluginConfig["env"].(string)
	if !envOk || len(env) == 0 {
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("PluginTapFeatherInit missing env in plugin config.")
		return errors.New("missing env for feather init")
	}
	trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("PluginTapFeatherInit start env=%s envBasis=%s\n", env, trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis)
	tempAddr := pluginConfig["vaddress"]
	tempTokenPtr := pluginConfig["tokenptr"]
	if cAddr, cAddressOk := pluginConfig["caddress"].(string); cAddressOk && len(cAddr) > 0 {
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("PluginTapFeatherInit using caddress override for env=%s\n", env)
		pluginConfig["vaddress"] = cAddr
	} else {
		eUtils.LogWarningMessage(trcshDriverConfig.DriverConfig.CoreConfig, "Unexpectedly caddress not available", false)
	}
	if cTokenPtr, cTokOk := pluginConfig["ctokenptr"].(*string); cTokOk && eUtils.RefLength(cTokenPtr) > 0 {
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("PluginTapFeatherInit using curator token override for env=%s tokenLength=%d\n", env, eUtils.RefLength(cTokenPtr))
		pluginConfig["tokenptr"] = cTokenPtr
	}

	if tokenPtr, tokPtrOk := pluginConfig["tokenptr"].(*string); tokPtrOk && eUtils.RefLength(tokenPtr) < 5 {
		eUtils.LogWarningMessage(trcshDriverConfig.DriverConfig.CoreConfig, "WARNING: Unexpectedly token not available", false)
	} else if tokPtrOk {
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("PluginTapFeatherInit token available for env=%s tokenLength=%d\n", env, eUtils.RefLength(tokenPtr))
	} else {
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("PluginTapFeatherInit tokenptr missing or not a string pointer for env=%s\n", env)
	}
	var featherMod *helperkv.Modifier
	var vault *sys.Vault
	var modErr error
	// Create modifier from kernel's TokenCache for cert loading (use kernel's env-based token with full permissions)
	trcshDriverConfig.DriverConfig, featherMod, vault, modErr = eUtils.InitVaultModForPlugin(pluginConfig,
		cache.NewTokenCache("config_token_pluginany",
			eUtils.RefMap(pluginConfig, "tokenptr"),
			eUtils.RefMap(pluginConfig, "vaddress")),
		"config_token_pluginany", trcshDriverConfig.DriverConfig.CoreConfig.Log)
	if vault != nil {
		defer vault.Close()
	}
	if modErr != nil {
		eUtils.LogErrorMessage(trcshDriverConfig.DriverConfig.CoreConfig, "Could not create feather modifier for feather init.", true)
		return modErr
	}
	if featherMod != nil {
		defer featherMod.Release()
	}

	pluginConfig["vaddress"] = tempAddr
	pluginConfig["tokenptr"] = tempTokenPtr

	// Use feather's modifier from TokenCache for cert loading (has full-permission token)
	trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("PluginTapFeatherInit invoking TapFeatherInit env=%s\n", env)
	tapErr := TapFeatherInit(trcshDriverConfig.DriverConfig, featherMod, pluginConfig, true, trcshDriverConfig.DriverConfig.CoreConfig.Log)
	if tapErr != nil {
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("PluginTapFeatherInit failed env=%s error=%s\n", env, tapErr.Error())
		return tapErr
	}
	trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("PluginTapFeatherInit complete env=%s\n", env)
	return nil
}

func TapFeatherInit(driverConfig *config.DriverConfig, mod *helperkv.Modifier, pluginConfig map[string]any, wantsFeathering bool, logger *log.Logger) error {
	var err error
	var ok bool
	logger.Printf("TapFeatherInit\n")

	if ok, err = servercapauth.ValidateTrcshPathSha(driverConfig, mod, pluginConfig, logger); ok {
		// Only start up if trcsh is up to date....
		onceAuth.Do(func() {
			logger.Printf("Initiating tap.\n")

			if pluginConfig["env"].(string) == "dev" || pluginConfig["env"].(string) == "staging" {
				// Ensure only dev is the cap auth...
				logger.Printf("Cap auth init for env: %s\n", pluginConfig["env"].(string))

				var featherAuth *servercapauth.FeatherAuth = nil
				if pluginConfig["env"].(string) == "dev" || pluginConfig["env"].(string) == "staging" {
					featherAuth, err = servercapauth.Init(driverConfig, mod, pluginConfig, wantsFeathering, logger)
					if err != nil {
						eUtils.LogErrorMessage(driverConfig.CoreConfig, "Skipping cap auth init.", false)
						return
					}
					if featherAuth != nil {
						pluginConfig["trcHatSecretsPort"] = featherAuth.SecretsPort
					}
					if wantsFeathering && len(featherAuth.EncryptPass) > 0 {
						pluginConfig["trcHatWantsFeathering"] = "true"
					}
				}
				servercapauth.Memorize(driverConfig, pluginConfig, logger)

				// Not really clear how cap auth would do this...
				var startReady sync.WaitGroup
				if featherAuth != nil && len(featherAuth.SecretsPort) > 0 {
					startReady.Add(1)
				}
				go servercapauth.Start(driverConfig, featherAuth, pluginConfig["env"].(string), logger, &startReady)
				startReady.Wait() // Wait for server to bind port
				logger.Printf("Cap auth feather init complete for env: %s\n", pluginConfig["env"].(string))

				gCapInitted = true

				logger.Printf("Cap auth init complete for env: %s\n", pluginConfig["env"].(string))
				logger.Printf("Tap init complete.\n")
				return
			} else {
				logger.Printf("Tap init complete.\n")
			}
		})
	} else {
		err := fmt.Errorf("mismatched sha256 cap auth for env: %s  skipping", pluginConfig["env"].(string))
		eUtils.LogErrorMessage(driverConfig.CoreConfig, err.Error(), false)
		return err
	}
	logger.Printf("TapFeatherInit complete\n")

	return err
}

func ValidateVaddr(vaddr string, logger *log.Logger) error {
	logger.Println("ValidateVaddr")
	for _, endpoint := range coreopts.BuildOptions.GetSupportedEndpoints(prod.IsProd()) {
		if strings.HasPrefix(vaddr, fmt.Sprintf("https://%s", endpoint[0])) {
			return nil
		}
	}
	logger.Println("Bad address: " + vaddr)
	return errors.New("Bad address: " + vaddr)
}

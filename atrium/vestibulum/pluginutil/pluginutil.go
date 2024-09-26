package pluginutil

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/logWriter"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/trimble-oss/tierceron-hat/cap/tap"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trccarrier/carrierfactory/servercapauth"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/opts/prod"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/cursoropts"
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
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
	logger.Printf("TapFeatherInit\n")

	if ok, err = servercapauth.ValidateTrcshPathSha(mod, pluginConfig, logger); ok {
		// Only start up if trcsh is up to date....
		onceAuth.Do(func() {
			logger.Printf("Initiating tap.\n")

			if pluginConfig["env"].(string) == "dev" || pluginConfig["env"].(string) == "staging" {
				// Ensure only dev is the cap auth...
				logger.Printf("Cap auth init for env: %s\n", pluginConfig["env"].(string))
				tap.TapInit(cursoropts.BuildOptions.GetCapPath())

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
				servercapauth.Memorize(pluginConfig, logger)

				// Not really clear how cap auth would do this...
				go servercapauth.Start(featherAuth, pluginConfig["env"].(string), logger)
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
		eUtils.LogErrorMessage(&driverConfig.CoreConfig, err.Error(), false)
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

func ParseCuratorEnvRecord(e *logical.StorageEntry, reqData *framework.FieldData, tokenEnvMap map[string]interface{}, logger *log.Logger) (map[string]interface{}, error) {
	logger.Println("parseCuratorEnvRecord")
	tokenMap := map[string]interface{}{}

	if tokenEnvMap != nil {
		tokenMap["env"] = tokenEnvMap["env"]
	}

	if e != nil {
		type tokenWrapper struct {
			Token      string `json:"token,omitempty"`
			VAddress   string `json:"vaddress,omitempty"`
			CAddress   string `json:"caddress,omitempty"`
			CToken     string `json:"ctoken,omitempty"`
			Pubrole    string `json:"pubrole,omitempty"`
			Configrole string `json:"configrole,omitempty"`
			Kubeconfig string `json:"kubeconfig,omitempty"`
			Plugin     string `json:"plugin,omitempty"`
		}
		tokenConfig := tokenWrapper{}
		decodeErr := e.DecodeJSON(&tokenConfig)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if memonly.IsMemonly() {
			memprotectopts.MemProtect(nil, &tokenConfig.VAddress)
			memprotectopts.MemProtect(nil, &tokenConfig.CAddress)
			memprotectopts.MemProtect(nil, &tokenConfig.CToken)
			memprotectopts.MemProtect(nil, &tokenConfig.Token)
			memprotectopts.MemProtect(nil, &tokenConfig.Pubrole)
			memprotectopts.MemProtect(nil, &tokenConfig.Configrole)
			memprotectopts.MemProtect(nil, &tokenConfig.Kubeconfig)
		}
		tokenMap["vaddress"] = tokenConfig.VAddress
		tokenMap["caddress"] = tokenConfig.CAddress
		tokenMap["ctoken"] = tokenConfig.CToken
		tokenMap["token"] = tokenConfig.Token
		tokenMap["pubrole"] = tokenConfig.Pubrole
		tokenMap["configrole"] = tokenConfig.Configrole
		tokenMap["kubeconfig"] = tokenConfig.Kubeconfig
		tokenMap["plugin"] = tokenConfig.Plugin
	}

	// Update and lock each field that is provided...
	if reqData != nil {
		tokenNameSlice := []string{"vaddress", "caddress", "ctoken", "token", "pubrole", "configrole", "kubeconfig"}
		for _, tokenName := range tokenNameSlice {
			if token, tokenOk := reqData.GetOk(tokenName); tokenOk && token.(string) != "" {
				tokenStr := token.(string)
				if memonly.IsMemonly() {
					memprotectopts.MemProtect(nil, &tokenStr)
				}
				tokenMap[tokenName] = tokenStr
			}
		}
	}
	logger.Println("parseCuratorEnvRecord complete")
	vaddrCheck := ""
	if v, vOk := tokenMap["vaddress"].(string); vOk {
		vaddrCheck = v
	}

	return tokenMap, ValidateVaddr(vaddrCheck, logger)
}

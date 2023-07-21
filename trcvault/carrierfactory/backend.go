package carrierfactory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
	"github.com/trimble-oss/tierceron/trcvault/opts/prod"
	trcvutils "github.com/trimble-oss/tierceron/trcvault/util"
	eUtils "github.com/trimble-oss/tierceron/utils"
	"github.com/trimble-oss/tierceron/utils/mlock"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"

	kv "github.com/hashicorp/vault-plugin-secrets-kv"

	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const CONFIG_PATH = "config"

var _ logical.Factory = TrcFactory

var logger *log.Logger

func InitLogger(l *log.Logger) {
	logger = l
}

func Init(processFlowConfig trcvutils.ProcessFlowConfig, processFlowInit trcvutils.ProcessFlowInitConfig, processFlow trcvutils.ProcessFlowFunc, headless bool, l *log.Logger) {
	eUtils.InitHeadless(headless)
	logger = l
	if os.Getenv(api.PluginMetadataModeEnv) == "true" {
		logger.Println("Metadata init.")
		return
	} else {
		logger.Println("Plugin Init begun.")
	}

	var configCompleteChan chan bool = nil
	if !headless {
		configCompleteChan = make(chan bool)
	}

	go func() {
		<-vaultInitialized
		go func() {
			for {
				// Sync drain on initialized in case any other updates come in...
				<-vaultInitialized
			}
		}()
		var supportedPluginNames []string

		for {
			logger.Println("Waiting for plugin env input....")
			pluginEnvConfig := <-tokenEnvChan
			logger.Println("Received new config for env: " + pluginEnvConfig["env"].(string))

			if _, pluginNameOk := pluginEnvConfig["trcplugin"]; !pluginNameOk {
				environmentConfigs[pluginEnvConfig["env"].(string)] = pluginEnvConfig
			}

			if _, ok := pluginEnvConfig["vaddress"]; !ok {
				logger.Println("Vault host not provided for env: " + pluginEnvConfig["env"].(string))
				continue
			}

			if configInitOnce, ciOk := pluginEnvConfig["syncOnce"]; ciOk {
				configInitOnce.(*sync.Once).Do(func() {

					if processFlowInit != nil {
						processFlowInit(pluginEnvConfig, logger)
					}

					logger.Println("Config engine init begun: " + pluginEnvConfig["env"].(string))

					// Get complete list of plugins...
					pluginEnvConfig = processFlowConfig(pluginEnvConfig)

					if len(supportedPluginNames) == 0 {
						if _, pluginNamesOk := pluginEnvConfig["pluginNameList"]; pluginNamesOk {
							supportedPluginNames = pluginEnvConfig["pluginNameList"].([]string)
						}
					}
					// Range over all plugins and init them... but only once!
					for _, pluginName := range pluginEnvConfig["pluginNameList"].([]string) {
						pluginEnvConfigClone := make(map[string]interface{})
						for k, v := range pluginEnvConfig {
							pluginEnvConfigClone[k] = v
						}
						pluginEnvConfigClone["trcplugin"] = pluginName
						logger.Println("*****Env: " + pluginEnvConfig["env"].(string) + " plugin: " + pluginEnvConfigClone["trcplugin"].(string))
						pecError := ProcessPluginEnvConfig(processFlowConfig, processFlow, pluginEnvConfigClone, configCompleteChan)
						if pecError != nil {
							logger.Println("Bad configuration data for env: " + pluginEnvConfig["env"].(string) + " and plugin: " + pluginName + " error: " + pecError.Error())
						}
					}

					if configCompleteChan != nil {
						configCompleteChan <- true
					}
					logger.Println("Config engine setup complete for env: " + pluginEnvConfig["env"].(string))
					pluginEnvConfig["syncOnce"] = nil
				})
			} else {

				if _, ok := pluginEnvConfig["trcplugin"]; !ok {
					continue
				}

				supported := false

				for _, pluginName := range supportedPluginNames {
					if pluginName == pluginEnvConfig["trcplugin"].(string) {
						supported = true
						break
					}
				}

				if !supported {
					logger.Println("Unsupported plugin for env: " + pluginEnvConfig["env"].(string) + " and plugin: " + pluginEnvConfig["trcplugin"].(string))
				} else {
					logger.Println("New plugin install env: " + pluginEnvConfig["env"].(string) + " plugin: " + pluginEnvConfig["trcplugin"].(string))
				}
				// Non init -- carrier new plugin deployment path...
				pecError := ProcessPluginEnvConfig(processFlowConfig, processFlow, pluginEnvConfig, configCompleteChan)

				if pecError != nil {
					logger.Println("Bad configuration data for env: " + pluginEnvConfig["env"].(string) + " error: " + pecError.Error())
				}
			}
		}

	}()
	if configCompleteChan != nil {
		<-configCompleteChan
	}
	logger.Println("Init ended.")
}

var KvInitialize func(context.Context, *logical.InitializationRequest) error
var KvCreate framework.OperationFunc
var KvUpdate framework.OperationFunc
var KvRead framework.OperationFunc

var vaultBootState int = 0

var vaultInitialized chan bool = make(chan bool)
var vaultHostInitialized chan bool = make(chan bool)
var environments []string = []string{"dev", "QA"}
var environmentsProd []string = []string{"staging", "prod"}
var environmentConfigs map[string]interface{} = map[string]interface{}{}

var tokenEnvChan chan map[string]interface{} = make(chan map[string]interface{}, 5)

func PushEnv(envMap map[string]interface{}) {
	tokenEnvChan <- envMap
}
func ValidateVaddr(vaddr string) error {
	logger.Println("ValidateVaddr")
	for _, endpoint := range coreopts.GetSupportedEndpoints() {
		if strings.HasPrefix(vaddr, endpoint) {
			return nil
		}
	}
	logger.Println("Bad address: " + vaddr)
	return errors.New("Bad address: " + vaddr)
}

// Cross checks against storage that this is a valid entry
func confirmInput(ctx context.Context, req *logical.Request, reqData *framework.FieldData, tokenEnvMap map[string]interface{}) (map[string]interface{}, error) {
	if tokenEnvMap == nil {
		tokenEnvMap = map[string]interface{}{}
	}
	if _, eOk := tokenEnvMap["env"].(string); !eOk {
		if req != nil {
			tokenEnvMap["env"] = req.Path
		} else {
			return nil, errors.New("Unable to determine env")
		}
	}
	logger.Println("Input validation for env: " + tokenEnvMap["env"].(string))
	var tokenConfirmationErr error
	if req != nil && req.Storage != nil {
		var tokenMap map[string]interface{}
		logger.Println("checkingVault")

		if tokenData, existingErr := req.Storage.Get(ctx, tokenEnvMap["env"].(string)); existingErr == nil {
			if tokenMap, tokenConfirmationErr = parseCarrierEnvRecord(tokenData, reqData, tokenEnvMap); tokenConfirmationErr == nil {
				return tokenMap, nil
			} else {
				return nil, tokenConfirmationErr
			}
		} else {
			// Completely new entry...
			logger.Println("This shouldn't happen env.")
			if tokenMap, tokenConfirmationErr = parseCarrierEnvRecord(tokenData, reqData, tokenEnvMap); tokenConfirmationErr != nil {
				return nil, tokenConfirmationErr
			} else {
				return tokenMap, nil
			}
		}
	} else {
		return nil, errors.New("Unconfirmed")
	}
	return nil, tokenConfirmationErr
}

func parseCarrierEnvRecord(e *logical.StorageEntry, reqData *framework.FieldData, tokenEnvMap map[string]interface{}) (map[string]interface{}, error) {
	logger.Println("parseCarrierEnvRecord")
	tokenMap := map[string]interface{}{}

	if tokenEnvMap != nil {
		tokenMap["env"] = tokenEnvMap["env"]
	}

	if e != nil {
		type tokenWrapper struct {
			Token      string `json:"token,omitempty"`
			VAddress   string `json:"vaddress,omitempty"`
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
			mlock.Mlock2(nil, &tokenConfig.VAddress)
			mlock.Mlock2(nil, &tokenConfig.Token)
			mlock.Mlock2(nil, &tokenConfig.Pubrole)
			mlock.Mlock2(nil, &tokenConfig.Configrole)
			mlock.Mlock2(nil, &tokenConfig.Kubeconfig)
		}
		tokenMap["vaddress"] = tokenConfig.VAddress
		tokenMap["token"] = tokenConfig.Token
		tokenMap["pubrole"] = tokenConfig.Pubrole
		tokenMap["configrole"] = tokenConfig.Configrole
		tokenMap["kubeconfig"] = tokenConfig.Kubeconfig
		tokenMap["plugin"] = tokenConfig.Plugin
	}

	// Update and lock each field that is provided...
	if reqData != nil {
		tokenNameSlice := []string{"vaddress", "token", "pubrole", "configrole", "kubeconfig"}
		for _, tokenName := range tokenNameSlice {
			if token, tokenOk := reqData.GetOk(tokenName); tokenOk && token.(string) != "" {
				tokenStr := token.(string)
				if memonly.IsMemonly() {
					mlock.Mlock2(nil, &tokenStr)
				}
				tokenMap[tokenName] = tokenStr
			}
		}
	}
	logger.Println("parseCarrierEnvRecord complete")
	vaddrCheck := ""
	if v, vOk := tokenMap["vaddress"].(string); vOk {
		vaddrCheck = v
	}

	return tokenMap, ValidateVaddr(vaddrCheck)
}

func ProcessPluginEnvConfig(processFlowConfig trcvutils.ProcessFlowConfig,
	processPluginFlow trcvutils.ProcessFlowFunc,
	pluginEnvConfig map[string]interface{},
	configCompleteChan chan bool) error {
	logger.Println("ProcessPluginEnvConfig begun: " + pluginEnvConfig["env"].(string) + " plugin: " + pluginEnvConfig["trcplugin"].(string))

	env, eOk := pluginEnvConfig["env"]
	if !eOk || env.(string) == "" {
		logger.Println("Bad configuration data.  Missing env.")
		return errors.New("missing token")
	}

	token, tOk := pluginEnvConfig["token"]
	if !tOk || token.(string) == "" {
		logger.Println("Bad configuration data for env: " + env.(string) + ".  Missing token.")
		return errors.New("missing token")
	}

	address, aOk := pluginEnvConfig["vaddress"]
	if !aOk || address.(string) == "" {
		logger.Println("Bad configuration data for env: " + env.(string) + ".  Missing address.")
		return errors.New("missing address")
	}

	pubrole, pOk := pluginEnvConfig["pubrole"]
	if !pOk || pubrole.(string) == "" {
		logger.Println("Bad configuration data for env: " + env.(string) + ".  Missing pub role.")
		return errors.New("missing pub role")
	}

	configrole, rOk := pluginEnvConfig["configrole"]
	if !rOk || configrole.(string) == "" {
		logger.Println("Bad configuration data for env: " + env.(string) + ".  Missing config role.")
		return errors.New("missing config role")
	}

	kubeconfig, rOk := pluginEnvConfig["kubeconfig"]
	if !rOk || kubeconfig.(string) == "" {
		logger.Println("Bad configuration data for env: " + env.(string) + ".  Missing kube config.")
		return errors.New("missing kube config")
	}

	pluginEnvConfig = processFlowConfig(pluginEnvConfig)
	if memonly.IsMemonly() {
		for _, value := range pluginEnvConfig {
			if valueSlice, isValueSlice := value.([]string); isValueSlice {
				for _, valueEntry := range valueSlice {
					mlock.Mlock2(nil, &valueEntry)
				}
			} else if valueString, isValueString := value.(string); isValueString {
				mlock.Mlock2(nil, &valueString)
			} else if _, isBool := value.(bool); isBool {
				// mlock.Mlock2(nil, &valueString)
				// TODO: no need to lock bools
			}
		}
	}

	go func(pec map[string]interface{}, l *log.Logger) {
		logger.Println("Begin processFlows for env: " + pec["env"].(string) + " plugin: " + pec["trcplugin"].(string))

		flowErr := processPluginFlow(pec, l)
		if configCompleteChan != nil {
			configCompleteChan <- true
		}
		if flowErr != nil {
			l.Println("Flow had an error: " + flowErr.Error())
		}
	}(pluginEnvConfig, logger)

	logger.Println("End processFlows for env: " + env.(string))

	return nil
}

// TrcInitialize -- main entry point for plugin.  When carrier is started,
// this function is always called.
func TrcInitialize(ctx context.Context, req *logical.InitializationRequest) error {
	logger.Println("TrcCarrierInitialize begun.")
	if memonly.IsMemonly() {
		logger.Println("Unlocking everything.")
		mlock.MunlockAll(nil)
	}
	queuedEnvironments := environments
	if prod.IsProd() {
		queuedEnvironments = environmentsProd
	}

	// Read in existing vault data from all existing environments on startup...
	for _, env := range queuedEnvironments {
		logger.Println("Processing env: " + env)
		tokenData, sgErr := req.Storage.Get(ctx, env)

		if sgErr != nil || tokenData == nil {
			if sgErr != nil {
				logger.Println("Missing configuration data for env: " + env + " error: " + sgErr.Error())
			} else {
				logger.Println("Missing configuration data for env: " + env)
			}
			continue
		}
		tokenMap, ptError := parseCarrierEnvRecord(tokenData, nil, map[string]interface{}{"env": env})

		if ptError != nil {
			logger.Println("Bad configuration data for env: " + env + " error: " + ptError.Error())
		} else {
			if _, ok := tokenMap["token"]; ok {
				tokenMap["env"] = env
				tokenMap["syncOnce"] = &sync.Once{}

				logger.Println("Initialize Pushing env: " + env)
				go PushEnv(tokenMap) // Startup is async queued.
				logger.Println("Env pushed: " + env)
			}
		}
	}
	vaultInitialized <- true

	if KvInitialize != nil {
		logger.Println("Entering KvInitialize...")
		return KvInitialize(ctx, req)
	}

	logger.Println("TrcCarrierInitialize complete.")
	return nil
}

func handleWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	key := data.Get("path").(string)
	if key == "" {
		return logical.ErrorResponse("missing path"), nil
	}

	// Check that some fields are given
	if len(req.Data) == 0 {
		return logical.ErrorResponse("missing data fields"), nil
	}

	// JSON encode the data
	buf, err := json.Marshal(req.Data)
	if err != nil {
		return nil, fmt.Errorf("json encoding failed: %v", err)
	}

	// Write out a new key
	entry := &logical.StorageEntry{
		Key:   key,
		Value: buf,
	}
	if err := req.Storage.Put(ctx, entry); err != nil {
		return nil, fmt.Errorf("failed to write: %v", err)
	}
	//ctx.Done()

	return nil, nil
}

func TrcRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	logger.Println("TrcCarrierRead complete.")

	return &logical.Response{
		Data: map[string]interface{}{
			"message": "Nice try.",
		},
	}, nil
}

func TrcCreate(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	logger.Println("TrcCarrierCreateUpdate")
	tokenEnvMap := map[string]interface{}{}
	key := req.Path //data.Get("path").(string)
	if key == "" {
		return logical.ErrorResponse("missing path"), nil
	}

	if token, tokenOk := data.GetOk("token"); tokenOk {
		tokenEnvMap["token"] = token
	} else {
		return nil, errors.New("Token required.")
	}

	if vaddr, addressOk := data.GetOk("vaddress"); addressOk {
		tokenEnvMap["vaddress"] = vaddr.(string)
	} else {
		return nil, errors.New("Vault Url required.")
	}

	tokenEnvMap["env"] = req.Path

	// Check that some fields are given
	if len(req.Data) == 0 {
		//ctx.Done()
		return logical.ErrorResponse("missing data fields"), nil
	}

	// JSON encode the data
	buf, err := json.Marshal(req.Data)
	if err != nil {
		//ctx.Done()
		return nil, fmt.Errorf("json encoding failed: %v", err)
	}

	// Write out a new key
	entry := &logical.StorageEntry{
		Key:   key,
		Value: buf,
	}
	if err := req.Storage.Put(ctx, entry); err != nil {
		//ctx.Done()
		return nil, fmt.Errorf("failed to write: %v", err)
	}

	logger.Println("Create Pushing env: " + tokenEnvMap["env"].(string))
	PushEnv(tokenEnvMap)
	logger.Println("TrcCarrierCreateUpdate complete.")

	return &logical.Response{
		Data: map[string]interface{}{
			"message": "Token created.",
		},
	}, nil
}

// TrcUpdate -- called during write operations...
// req  -- contains actual request.
// data -- contains schema validated request fields... best to pull from data...
func TrcUpdate(ctx context.Context, req *logical.Request, reqData *framework.FieldData) (*logical.Response, error) {
	logger.Println("TrcCarrierUpdate")
	tokenEnvMap := map[string]interface{}{}
	key := req.Path
	var plugin interface{}
	var tokenParseDataErr error

	pluginOk := false
	if plugin, pluginOk = reqData.GetOk("plugin"); pluginOk {
		// Then this is the deploy calling.
		logger.Println("TrcCarrierUpdate checking plugin: " + plugin.(string))

		if entry, err := req.Storage.Get(ctx, req.Path); err != nil || entry == nil {
			//ctx.Done()
			return &logical.Response{
				Data: map[string]interface{}{
					"message": "Nice try.",
				},
			}, nil
		} else {
			vaultData := map[string]interface{}{}
			if err := json.Unmarshal(entry.Value, &vaultData); err != nil {
				//ctx.Done()
				return nil, err
			}

			if tokenEnvMap, tokenParseDataErr = confirmInput(ctx, req, reqData, tokenEnvMap); tokenParseDataErr != nil {
				return logical.ErrorResponse("Plugin delivery failure: invalid input validation"), nil
			} else {
				tokenEnvMap["trcplugin"] = plugin.(string)
			}
			logger.Println("Creating modifier for env: " + req.Path)

			// Plugins
			mod, err := helperkv.NewModifier(true, tokenEnvMap["token"].(string), tokenEnvMap["vaddress"].(string), req.Path, nil, true, logger)
			if mod != nil {
				defer mod.Release()
			}
			if err != nil {
				logger.Println("Failed to init mod for deploy update")
				logger.Println("Error: " + err.Error())
				return logical.ErrorResponse("Failed to init mod for deploy update"), nil
			}
			mod.Env = req.Path
			logger.Println("TrcCarrierUpdate getting plugin settings for env: " + req.Path)
			// The following confirms that this version of carrier has been certified to run...
			// It will bail if it hasn't.

			writeMap, err := mod.ReadData("super-secrets/Index/TrcVault/trcplugin/" + tokenEnvMap["trcplugin"].(string) + "/Certify")
			if err != nil {
				logger.Println("Failed to read previous plugin status from vault")
				logger.Println("Error: " + err.Error())
				return logical.ErrorResponse("Failed to read previous plugin status from vault"), nil
			}
			logger.Println("TrcCarrierUpdate Checking sha")

			if _, ok := writeMap["trcsha256"]; !ok {
				logger.Println("Failed to read previous plugin sha from vault")
				return logical.ErrorResponse("Failed to read previous plugin sha from vault"), nil
			}

			logger.Println("Update Pushing plugin for env: " + tokenEnvMap["env"].(string) + " and plugin: " + tokenEnvMap["trcplugin"].(string))
			tokenEnvMap["trcsha256chan"] = make(chan bool)
			PushEnv(tokenEnvMap)
			logger.Println("Queued plugin: " + tokenEnvMap["trcplugin"].(string))

			// Listen on sha256 channel....
			<-tokenEnvMap["trcsha256chan"].(chan bool)
			var sha256 string
			if sha256Interface, shaOk := tokenEnvMap["trcsha256"]; shaOk {
				sha256 = sha256Interface.(string)
			} else {
				sha256 = "Failure to copy plugin."
			}

			return &logical.Response{
				Data: map[string]interface{}{
					"message": sha256,
				},
			}, nil

		}
	} else {
		// Then this is the carrier calling
		// Path includes Env and token will only work if it has right permissions.
		if tokenEnvMap, tokenParseDataErr = confirmInput(ctx, req, reqData, tokenEnvMap); tokenParseDataErr != nil {
			// Bad or corrupt data in vault.
			return nil, errors.New("Input data validation error.")
		}

		logger.Println("TrcCarrierUpdate merging tokens.")
		if key == "" {
			return logical.ErrorResponse("missing path"), nil
		}

		// Check that some fields are given
		if len(req.Data) == 0 {
			return logical.ErrorResponse("missing data fields"), nil
		}

		logger.Println("Update carrier secrets for env: " + tokenEnvMap["env"].(string))
		// JSON encode the data
		buf, err := json.Marshal(req.Data)
		if err != nil {
			return nil, fmt.Errorf("json encoding failed: %v", err)
		}

		// Write out a new key
		entry := &logical.StorageEntry{
			Key:   key,
			Value: buf,
		}
		if err := req.Storage.Put(ctx, entry); err != nil {
			//ctx.Done()
			return nil, fmt.Errorf("failed to write: %v", err)
		}
	}

	logger.Println("TrcCarrierUpdate complete.")

	return &logical.Response{
		Data: map[string]interface{}{
			"message": "Token updated.",
		},
	}, nil
}

// TrcFactory configures and returns Mock backends
func TrcFactory(ctx context.Context, conf *logical.BackendConfig) (logical.Backend, error) {
	logger.Println("TrcCarrierFactory")
	env, err := conf.System.PluginEnv(ctx)
	if env != nil {
		logger.Println("=============== Initializing Vault Tierceron Plugin ===============")
		logger.Println("Factory initialization begun.")
	}

	if err != nil {
		logger.Println("Factory init failure: " + err.Error())
		os.Exit(-1)
	}

	bkv, err := kv.Factory(ctx, conf)
	if err != nil {
		logger.Println("Something wrong: " + err.Error())
	} else {
		if env != nil {
			logger.Println("Kv initialization begun.")
		}
		KvInitialize = bkv.(*kv.PassthroughBackend).InitializeFunc
		bkv.(*kv.PassthroughBackend).InitializeFunc = TrcInitialize
		if env != nil {
			logger.Println("Kv initialization complete.")
		}

		KvCreate = bkv.(*kv.PassthroughBackend).Paths[0].Callbacks[logical.CreateOperation]
		KvUpdate = bkv.(*kv.PassthroughBackend).Paths[0].Callbacks[logical.UpdateOperation]
		KvRead = bkv.(*kv.PassthroughBackend).Paths[0].Callbacks[logical.ReadOperation]
		bkv.(*kv.PassthroughBackend).Paths[0].Callbacks[logical.CreateOperation] = TrcCreate
		bkv.(*kv.PassthroughBackend).Paths[0].Callbacks[logical.UpdateOperation] = TrcUpdate
		bkv.(*kv.PassthroughBackend).Paths[0].Callbacks[logical.ReadOperation] = TrcRead
	}

	bkv.(*kv.PassthroughBackend).Paths = []*framework.Path{
		{
			Pattern:         "(dev|QA|staging|prod)",
			HelpSynopsis:    "Configure an access token.",
			HelpDescription: "Use this endpoint to configure the auth tokens required by trcvault.",

			Fields: map[string]*framework.FieldSchema{
				"pubrole": {
					Type:        framework.TypeString,
					Description: "Pub role for specified environment.",
				},
				"configrole": {
					Type:        framework.TypeString,
					Description: "Read only role for specified environment.",
				},
				"kubeconfig": {
					Type:        framework.TypeString,
					Description: "kube config for specified environment.",
				},
				"token": {
					Type:        framework.TypeString,
					Description: "Token used for specified environment.",
				},
				"vaddress": {
					Type:        framework.TypeString,
					Description: "Vaurl Url for plugin reference purposes.",
				},
				"plugin": {
					Type:        framework.TypeString,
					Description: "Optional plugin name.",
					Required:    false,
				},
			},

			Callbacks: map[logical.Operation]framework.OperationFunc{
				logical.ReadOperation:   bkv.(*kv.PassthroughBackend).Paths[0].Callbacks[logical.ReadOperation],
				logical.CreateOperation: TrcCreate,
				logical.UpdateOperation: TrcUpdate,
			},
		},
	}

	if env != nil {
		logger.Println("Factory initialization complete.")
		logger.Println("=============== Vault Tierceron Plugin Initialization complete ===============")
	}

	if err != nil {
		logger.Println("TrcCarrierFactory had an error: " + err.Error())
	}

	return bkv, err
}

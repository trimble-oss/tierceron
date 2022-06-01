package factory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	vcutils "tierceron/trcconfig/utils"
	trcvutils "tierceron/trcvault/util"
	eUtils "tierceron/utils"
	helperkv "tierceron/vaulthelper/kv"
	"time"

	kv "github.com/hashicorp/vault-plugin-secrets-kv"
	"gopkg.in/yaml.v2"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const CONFIG_PATH = "config"

var _ logical.Factory = TrcFactory

var logger *log.Logger

func Init(processFlowConfig trcvutils.ProcessFlowConfig, processFlows trcvutils.ProcessFlowFunc, headless bool, l *log.Logger) {
	eUtils.InitHeadless(headless)
	logger = l
	logger.Println("Init begun.")

	// Set up a table process runner.
	go initVaultHostBootstrap()
	<-vaultHostInitialized

	var testCompleteChan chan bool = nil
	if !headless {
		testCompleteChan = make(chan bool)
	}

	go func() {
		<-vaultInitialized
		for {
			pluginEnvConfig := <-tokenEnvChan

			if _, ok := pluginEnvConfig["address"]; !ok {
				// Testflow won't have this set yet.
				pluginEnvConfig["address"] = GetVaultHost()
			}

			if !strings.HasSuffix(pluginEnvConfig["address"].(string), GetVaultPort()) {
				// Missing port.
				vhost := pluginEnvConfig["address"].(string)
				vhost = vhost + ":" + GetVaultPort()
				pluginEnvConfig["address"] = vhost
			}
			pluginEnvConfig["insecure"] = true

			logger.Println("Config engine init begun: " + pluginEnvConfig["env"].(string))
			pecError := ProcessPluginEnvConfig(processFlowConfig, processFlows, pluginEnvConfig, testCompleteChan)

			if pecError != nil {
				logger.Println("Bad configuration data for env: " + pluginEnvConfig["env"].(string) + " error: " + pecError.Error())
				testCompleteChan <- true
			}
			logger.Println("Config engine setup complete for env: " + pluginEnvConfig["env"].(string))
		}

	}()
	if testCompleteChan != nil {
		<-testCompleteChan
	}
	logger.Println("Init ended.")
}

var KvInitialize func(context.Context, *logical.InitializationRequest) error
var KvCreate framework.OperationFunc
var KvUpdate framework.OperationFunc
var KvRead framework.OperationFunc

var vaultBootState int = 0
var vaultHost string // Plugin will only communicate locally with a vault instance.
var vaultPort string
var vaultInitialized chan bool = make(chan bool)
var vaultHostInitialized chan bool = make(chan bool)
var environments []string = []string{"dev", "QA"}
var environmentConfigs map[string]*EnvConfig = map[string]*EnvConfig{}

var tokenEnvChan chan map[string]interface{} = make(chan map[string]interface{}, 5)

var pluginSettingsChan map[string]chan bool = map[string]chan bool{}
var pluginShaMap map[string]string = map[string]string{}

func PushEnv(envMap map[string]interface{}) {
	tokenEnvChan <- envMap
}

func PushPluginSha(config *eUtils.DriverConfig, plugin string, sha string) {
	pluginShaMap[plugin] = sha
	if _, pscOk := pluginSettingsChan[plugin]; !pscOk {
		eUtils.LogInfo(config, "Creating new plugin chan.")
		pluginSettingsChan[plugin] = make(chan bool, 1)
	}

	pluginSettingsChan[plugin] <- true
}

func GetVaultHost() string {
	return vaultHost
}

func GetVaultPort() string {
	return vaultPort
}

func initVaultHostBootstrap() error {
	const (
		DEFAULT  = 0               //
		WARMUP   = 1 << iota       // 1
		HOST     = 1 << iota       // 2
		COMPLETE = (WARMUP | HOST) // 3
	)

	if vaultBootState == DEFAULT {
		vaultBootState = WARMUP
		logger.Println("Begin finding vault.")

		vaultHostChan := make(chan string, 1)
		vaultLookupErrChan := make(chan error, 1)
		trcvutils.GetLocalVaultHost(true, vaultHostChan, vaultLookupErrChan, logger)

		for (vaultBootState & COMPLETE) != COMPLETE {
			select {
			case v := <-vaultHostChan:
				vaultHost = v
				vaultBootState |= HOST
				vaultHostInitialized <- true
			case lvherr := <-vaultLookupErrChan:
				logger.Println("Couldn't find local vault: " + lvherr.Error())
				vaultBootState = COMPLETE
			}
		}
		vaultInitialized <- true
		logger.Println("End finding vault.")
	}
	return nil
}

func parseToken(e *logical.StorageEntry) (map[string]interface{}, error) {
	tokenMap := map[string]interface{}{}
	type tokenWrapper struct {
		Token    string `json:"token,omitempty"`
		VAddress string `json:"vaddress,omitempty"`
	}
	tokenConfig := tokenWrapper{}
	e.DecodeJSON(&tokenConfig)
	tokenMap["token"] = tokenConfig.Token

	vaultUrl, err := url.Parse(tokenConfig.VAddress)
	if err == nil {
		vaultPort = vaultUrl.Port()
	}

	return tokenMap, nil
}

func populateTrcVaultDbConfigs(config *eUtils.DriverConfig) error {
	logger.Println("Begin populateTrcVaultDbConfigs for env: " + config.Env)
	var goMod *helperkv.Modifier
	goMod, errModInit := helperkv.NewModifier(true, config.Token, config.VaultAddress, config.Env, []string{}, logger)
	goMod.Env = config.Env

	if errModInit != nil {
		logger.Println("Vault connect failure")
		return errModInit
	}

	configuredTemplate, _, _, ctErr := vcutils.ConfigTemplate(config, goMod, "/trc_templates/TrcVault/Database/config.tmpl", true, "TrcVault", "Database", false, true)
	if ctErr != nil {
		logger.Println("Config template lookup failure: " + ctErr.Error())
		return ctErr
	}

	var vaultEnvConfig EnvConfig

	errYaml := yaml.Unmarshal([]byte(configuredTemplate), &vaultEnvConfig)
	if errYaml != nil {
		logger.Println("Vault config lookup failure: " + configuredTemplate)
		return errYaml
	}

	vaultEnvConfig.Env = config.Env
	environmentConfigs[config.Env] = &vaultEnvConfig
	logger.Println("Config created for env: " + config.Env)

	logger.Println("End populateTrcVaultDbConfigs")
	return nil
}

func ProcessPluginEnvConfig(processFlowConfig trcvutils.ProcessFlowConfig,
	processFlows trcvutils.ProcessFlowFunc,
	pluginEnvConfig map[string]interface{},
	testCompleteChan chan bool) error {
	logger.Println("ProcessPluginEnvConfig begun.")
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

	address, aOk := pluginEnvConfig["address"]
	if !aOk || address.(string) == "" {
		logger.Println("Bad configuration data for env: " + env.(string) + ".  Missing address.")
		return errors.New("missing address")
	}

	if _, enabledTrcDb := pluginEnvConfig["enableTrcDbInterface"]; enabledTrcDb {
		// This isn't even used yet....
		ptvError := populateTrcVaultDbConfigs(&eUtils.DriverConfig{Env: env.(string), Token: token.(string), VaultAddress: vaultHost, ExitOnFailure: false})
		if ptvError != nil {
			logger.Println("Bad configuration data for env: " + env.(string) + ".  error: " + ptvError.Error())
			return ptvError
		}
	}

	pluginEnvConfig = processFlowConfig(pluginEnvConfig)
	logger.Println("Begin processFlows for env: " + env.(string))

	go func(pec map[string]interface{}, l *log.Logger) {
		flowErr := processFlows(pec, l)
		if testCompleteChan != nil {
			testCompleteChan <- true
		}
		if flowErr != nil {
			l.Println("Flow had an error: " + flowErr.Error())
		}
	}(pluginEnvConfig, logger)

	logger.Println("End processFlows for env: " + env.(string))

	return nil
}

func TrcInitialize(ctx context.Context, req *logical.InitializationRequest) error {
	logger.Println("TrcInitialize begun.")

	for _, env := range environments {
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
		tokenMap, ptError := parseToken(tokenData)

		if ptError != nil {
			logger.Println("Bad configuration data for env: " + env + " error: " + ptError.Error())
		} else {
			if _, ok := tokenMap["token"]; ok {
				tokenMap["env"] = env
				tokenMap["insecure"] = true
				tokenMap["address"] = vaultHost
				PushEnv(tokenMap)
			}
		}
	}

	if KvInitialize != nil {
		logger.Println("Entering KvInitialize...")
		return KvInitialize(ctx, req)
	}

	//ctx.Done()
	logger.Println("TrcInitialize complete.")
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
	logger.Println("TrcRead")

	key := req.Path //data.Get("path").(string)
	if key == "" {
		//ctx.Done()
		return logical.ErrorResponse("missing path"), nil
	}

	// Write out a new key
	if entry, err := req.Storage.Get(ctx, key); err != nil || entry == nil {
		//ctx.Done()
		return &logical.Response{
			Data: map[string]interface{}{
				"message": "Entry missing.",
			},
		}, nil
	} else {
		vData := map[string]interface{}{}
		if err := json.Unmarshal(entry.Value, &vData); err != nil {
			//ctx.Done()
			return nil, err
		}
		tokenEnvMap := map[string]interface{}{}
		tokenEnvMap["env"] = req.Path
		tokenEnvMap["address"] = vaultHost
		tokenEnvMap["insecure"] = true
		if vData["token"] != nil {
			logger.Println("Env queued: " + req.Path)
		}
		tokenEnvMap["token"] = vData["token"]
		PushEnv(tokenEnvMap)
		//ctx.Done()
	}
	logger.Println("TrcRead complete.")

	return &logical.Response{
		Data: map[string]interface{}{
			"message": "Nice try.",
		},
	}, nil
}

func TrcCreate(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	logger.Println("TrcCreateUpdate")
	tokenEnvMap := map[string]interface{}{}
	key := req.Path //data.Get("path").(string)
	if key == "" {
		//ctx.Done()
		return logical.ErrorResponse("missing path"), nil
	}

	if token, tokenOk := data.GetOk("token"); tokenOk {
		tokenEnvMap["token"] = token
	} else {
		return nil, errors.New("Token required.")
	}

	if vaddr, addressOk := data.GetOk("vaddress"); addressOk {
		vaultUrl, err := url.Parse(vaddr.(string))
		tokenEnvMap["interfaceaddr"] = vaddr.(string)
		if err == nil {
			vaultPort = vaultUrl.Port()
		}
	} else {
		return nil, errors.New("Vault Url required.")
	}

	tokenEnvMap["env"] = req.Path
	tokenEnvMap["address"] = vaultHost
	tokenEnvMap["insecure"] = true

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

	tokenEnvChan <- tokenEnvMap
	//ctx.Done()
	logger.Println("TrcCreateUpdate complete.")

	return &logical.Response{
		Data: map[string]interface{}{
			"message": "Token created.",
		},
	}, nil
}

func TrcUpdate(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	logger.Println("TrcUpdate")
	tokenEnvMap := map[string]interface{}{}

	var plugin interface{}
	pluginOk := false
	if plugin, pluginOk = data.GetOk("plugin"); pluginOk {
		logger.Println("TrcUpdate checking plugin: " + plugin.(string))

		// Then this is the carrier calling.
		tokenEnvMap["trcplugin"] = plugin.(string)
		if _, pscOk := pluginSettingsChan[plugin.(string)]; !pscOk {
			pluginSettingsChan[plugin.(string)] = make(chan bool, 1)
		}
		logger.Println("TrcUpdate begin setup for plugin settings init")

		if token, tokenOk := data.GetOk("token"); tokenOk {
			logger.Println("TrcUpdate stage 1")

			if GetVaultPort() == "" {
				logger.Println("TrcUpdate stage 1.1")
				if vaddr, addressOk := data.GetOk("vaddress"); addressOk {
					logger.Println("TrcUpdate stage 1.1.1")
					vaultUrl, err := url.Parse(vaddr.(string))
					tokenEnvMap["interfaceaddr"] = vaddr.(string)
					if err == nil {
						logger.Println("TrcUpdate stage 1.1.1.1")
						vaultPort = vaultUrl.Port()
					} else {
						logger.Println("Bad address: " + vaddr.(string))
					}
				} else {
					return nil, errors.New("Vault Update Url required.")
				}
			}

			if !strings.HasSuffix(vaultHost, GetVaultPort()) {
				// Missing port.
				vaultHost = vaultHost + ":" + GetVaultPort()
			}

			// Plugins
			mod, err := helperkv.NewModifier(true, token.(string), vaultHost, req.Path, nil, logger)
			if err != nil {
				logger.Println("Failed to init mod for deploy update")
				//ctx.Done()
				logger.Println("Error: " + err.Error())
				return logical.ErrorResponse("Failed to init mod for deploy update"), nil
			}
			mod.Env = req.Path
			logger.Println("TrcUpdate getting plugin settings for env: " + req.Path)
			writeMap, err := mod.ReadData("super-secrets/Index/TrcVault/trcplugin/" + tokenEnvMap["trcplugin"].(string) + "/Certify")
			if err != nil {
				logger.Println("Failed to read previous plugin status from vault")
				logger.Println("Error: " + err.Error())
				return logical.ErrorResponse("Failed to read previous plugin status from vault"), nil
			}
			logger.Println("TrcUpdate Checking sha")

			if _, ok := writeMap["trcsha256"]; !ok {
				logger.Println("Failed to read previous plugin sha from vault")
				return logical.ErrorResponse("Failed to read previous plugin sha from vault"), nil
			}
		}
	}

	// TODO: Verify token and env...
	// Path includes Env and token will only work if it has right permissions.
	tokenEnvMap["env"] = req.Path

	if token, tokenOk := data.GetOk("token"); tokenOk {
		tokenEnvMap["token"] = token
	} else {
		//ctx.Done()
		return nil, errors.New("Token required.")
	}

	if vaddr, addressOk := data.GetOk("vaddress"); addressOk {
		vaultUrl, err := url.Parse(vaddr.(string))
		tokenEnvMap["interfaceaddr"] = vaddr.(string)
		if err == nil {
			vaultPort = vaultUrl.Port()
		}
	} else {
		return nil, errors.New("Vault Create Url required.")
	}

	tokenEnvMap["address"] = vaultHost
	tokenEnvMap["insecure"] = true

	key := req.Path
	if key == "" {
		//ctx.Done()
		return logical.ErrorResponse("missing path"), nil
	}

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

	// This will kick off the main flow for the plugin..
	tokenEnvChan <- tokenEnvMap

	if pluginOk {
		// Listen on sha256 channel....
		var sha256 string
		sha256, shaOk := pluginShaMap[tokenEnvMap["trcplugin"].(string)]

		select {
		case <-pluginSettingsChan[tokenEnvMap["trcplugin"].(string)]:
			sha256 = pluginShaMap[tokenEnvMap["trcplugin"].(string)]
		case <-time.After(time.Second * 7):
			if !shaOk {
				sha256 = "Failure to copy plugin."
			}
		}
		//ctx.Done()

		return &logical.Response{
			Data: map[string]interface{}{
				"message": sha256,
			},
		}, nil
	}

	//ctx.Done()

	logger.Println("TrcUpdate complete.")

	return &logical.Response{
		Data: map[string]interface{}{
			"message": "Token updated.",
		},
	}, nil
}

// TrcFactory configures and returns Mock backends
func TrcFactory(ctx context.Context, conf *logical.BackendConfig) (logical.Backend, error) {
	logger.Println("TrcFactory")
	env, err := conf.System.PluginEnv(ctx)
	if env != nil {
		logger.Println("=============== Initializing Vault Tierceron Plugin ===============")
		logger.Println("Factory initialization begun.")
	}
	environmentConfigs = map[string]*EnvConfig{}

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
		logger.Println("TrcFactory had an error: " + err.Error())
	}

	return bkv, err
}

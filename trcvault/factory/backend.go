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
	"time"

	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
	"github.com/trimble-oss/tierceron/trcvault/opts/prod"
	trcvutils "github.com/trimble-oss/tierceron/trcvault/util"
	eUtils "github.com/trimble-oss/tierceron/utils"

	kv "github.com/hashicorp/vault-plugin-secrets-kv"

	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const CONFIG_PATH = "config"

var _ logical.Factory = TrcFactory

var logger *log.Logger

func Init(processFlowConfig trcvutils.ProcessFlowConfig, processFlows trcvutils.ProcessFlowFunc, headless bool, l *log.Logger) {
	eUtils.InitHeadless(headless)
	logger = l
	if os.Getenv(api.PluginMetadataModeEnv) == "true" {
		logger.Println("Metadata init.")
		return
	} else {
		logger.Println("Plugin Init begun.")
	}

	// Set up a table process runner.
	go initVaultHostBootstrap()
	<-vaultHostInitialized

	var configCompleteChan chan bool = nil
	if !headless {
		configCompleteChan = make(chan bool)
	}

	go func() {
		<-vaultInitialized
		for {
			pluginEnvConfig := <-tokenEnvChan
			environmentConfigs[pluginEnvConfig["env"].(string)] = pluginEnvConfig

			if _, ok := pluginEnvConfig["vaddress"]; !ok {
				// Testflow won't have this set yet.
				pluginEnvConfig["vaddress"] = GetVaultHost()
			}

			if !strings.HasSuffix(pluginEnvConfig["vaddress"].(string), GetVaultPort()) {
				// Missing port.
				vhost := pluginEnvConfig["vaddress"].(string)
				vhost = vhost + ":" + GetVaultPort()
				pluginEnvConfig["vaddress"] = vhost
			}

			logger.Println("Config engine init begun: " + pluginEnvConfig["env"].(string))
			pecError := ProcessPluginEnvConfig(processFlowConfig, processFlows, pluginEnvConfig, configCompleteChan)

			if pecError != nil {
				logger.Println("Bad configuration data for env: " + pluginEnvConfig["env"].(string) + " error: " + pecError.Error())
				configCompleteChan <- true
			}
			logger.Println("Config engine setup complete for env: " + pluginEnvConfig["env"].(string))
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
var vaultHost string // Plugin will only communicate locally with a vault instance.
var vaultPort string
var vaultInitialized chan bool = make(chan bool)
var vaultHostInitialized chan bool = make(chan bool)
var environments []string = []string{"dev", "QA"}
var environmentsProd []string = []string{"staging", "prod"}
var environmentConfigs map[string]interface{} = map[string]interface{}{}

var tokenEnvChan chan map[string]interface{} = make(chan map[string]interface{}, 5)

var pluginSettingsChan map[string]chan time.Time = map[string]chan time.Time{}
var pluginShaMap map[string]string = map[string]string{}

func PushEnv(envMap map[string]interface{}) {
	tokenEnvChan <- envMap
}

// This is to flush pluginSettingsChan on an interval to prevent deadlocks.
func StartPluginSettingEater() {
	go func() {
		for { //Infinite loop
			if pluginSettingsChan != nil {
				for plugin, pluginSetChan := range pluginSettingsChan {
					select {
					case set := <-pluginSetChan:
						if time.Now().After(set.Add(time.Second * 30)) { //If signal was sent more than 30 seconds ago
							if logger != nil {
								logger.Println("Emptying stale update alert for " + plugin)
							}
							time.Sleep(time.Millisecond * 50)
						} else {
							if pluginSetChan != nil {
								pluginSetChan <- set
							}
						}
					default:
						continue
					}
				}
			}
			time.Sleep(time.Minute * 5) //Check every 5 minutes
		}
	}()
}

// Only push to sha256 chan if one is present.  Non blocking otherwise.
func PushPluginSha(config *eUtils.DriverConfig, pluginConfig map[string]interface{}, vaultPluginSignature map[string]interface{}) {
	if _, trcShaChanOk := pluginConfig["trcsha256chan"]; trcShaChanOk {
		if vaultPluginSignature != nil {
			pluginConfig["trcsha256"] = vaultPluginSignature["trcsha256"]
		}
		pluginConfig["trcsha256chan"].(chan bool) <- true
	}
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
		CAddress string `json:"caddress,omitempty"`
		CToken   string `json:"ctoken,omitempty"`
	}
	tokenConfig := tokenWrapper{}
	e.DecodeJSON(&tokenConfig)
	tokenMap["token"] = tokenConfig.Token
	tokenMap["caddress"] = tokenConfig.CAddress
	tokenMap["ctoken"] = tokenConfig.CToken

	vaultUrl, err := url.Parse(tokenConfig.VAddress)
	if err == nil {
		vaultPort = vaultUrl.Port()
	}

	return tokenMap, nil
}

func ProcessPluginEnvConfig(processFlowConfig trcvutils.ProcessFlowConfig,
	processFlows trcvutils.ProcessFlowFunc,
	pluginEnvConfig map[string]interface{},
	configCompleteChan chan bool) error {
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

	address, aOk := pluginEnvConfig["vaddress"]
	if !aOk || address.(string) == "" {
		logger.Println("Bad configuration data for env: " + env.(string) + ".  Missing address.")
		return errors.New("missing address")
	}

	ctoken, cTOk := pluginEnvConfig["ctoken"]
	if !cTOk || ctoken.(string) == "" {
		logger.Println("Bad configuration data for env: " + env.(string) + ".  Missing ctoken.")
		return errors.New("missing ctoken")
	}

	caddress, caOk := pluginEnvConfig["caddress"]
	if !caOk || caddress.(string) == "" {
		logger.Println("Bad configuration data for env: " + env.(string) + ".  Missing caddress.")
		return errors.New("missing address")
	}

	pluginEnvConfig = processFlowConfig(pluginEnvConfig)
	logger.Println("Begin processFlows for env: " + env.(string))
	if memonly.IsMemonly() {
		logger.Println("Unlocking everything.")
		memprotectopts.MemUnprotectAll(nil)
		for _, environmentConfig := range environmentConfigs {
			for _, value := range environmentConfig.(map[string]interface{}) {
				if valueSlice, isValueSlice := value.([]string); isValueSlice {
					for _, valueEntry := range valueSlice {
						memprotectopts.MemProtect(nil, &valueEntry)
					}
				} else if valueString, isValueString := value.(string); isValueString {
					memprotectopts.MemProtect(nil, &valueString)
				} else if _, isBool := value.(bool); isBool {
					// memprotectopts.MemProtect(nil, &valueString)
					// TODO: no need to lock bools
				}
			}
		}
		logger.Println("Finished selective locks.")
	}

	go func(pec map[string]interface{}, l *log.Logger) {
		flowErr := processFlows(pec, l)
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

func TrcInitialize(ctx context.Context, req *logical.InitializationRequest) error {
	logger.Println("TrcInitialize begun.")

	queuedEnvironments := environments
	if prod.IsProd() {
		queuedEnvironments = environmentsProd
	}

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
		tokenMap, ptError := parseToken(tokenData)

		if ptError != nil {
			logger.Println("Bad configuration data for env: " + env + " error: " + ptError.Error())
		} else {
			if _, ok := tokenMap["token"]; ok {
				tokenMap["env"] = env
				tokenMap["vaddress"] = vaultHost
				logger.Println("Initialize Pushing env: " + env)
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
		tokenEnvMap["vaddress"] = vData["vaddress"]
		if vData["token"] != nil {
			logger.Println("Env queued: " + req.Path)
		} else {
			mTokenErr := errors.New("Skipping environment due to missing token: " + req.Path)
			logger.Println(mTokenErr.Error())
			return nil, mTokenErr
		}
		tokenEnvMap["token"] = vData["token"]
		tokenEnvMap["caddress"] = vData["caddress"]
		tokenEnvMap["ctoken"] = vData["ctoken"]

		logger.Println("Read Pushing env: " + tokenEnvMap["env"].(string))
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

	if cAddr, tokenOK := data.GetOk("caddress"); tokenOK {
		tokenEnvMap["caddress"] = cAddr
	} else {
		return nil, errors.New("caddress required.")
	}
	if cToken, tokenOK := data.GetOk("ctoken"); tokenOK {
		tokenEnvMap["ctoken"] = cToken
	} else {
		return nil, errors.New("ctoken required.")
	}

	if vaddr, addressOk := data.GetOk("vaddress"); addressOk {
		vaultUrl, err := url.Parse(vaddr.(string))
		tokenEnvMap["vaddress"] = vaddr.(string)
		if err == nil {
			vaultPort = vaultUrl.Port()
		}
	} else {
		return nil, errors.New("Vault Url required.")
	}

	tokenEnvMap["env"] = req.Path
	tokenEnvMap["vaddress"] = vaultHost

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
			pluginSettingsChan[plugin.(string)] = make(chan time.Time, 1)
		}
		logger.Println("TrcUpdate begin setup for plugin settings init")

		if _, tokenOk := data.GetOk("token"); tokenOk {
			logger.Println("TrcUpdate stage 1")

			if GetVaultPort() == "" {
				logger.Println("TrcUpdate stage 1.1")
				if vaddr, addressOk := data.GetOk("vaddress"); addressOk {
					logger.Println("TrcUpdate stage 1.1.1")
					vaultUrl, err := url.Parse(vaddr.(string))
					tokenEnvMap["vaddress"] = vaddr.(string)
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

			if caddr, addressOk := data.GetOk("caddress"); addressOk {
				vaultUrl, err := url.Parse(caddr.(string))
				if err == nil {
					cVaultPort := vaultUrl.Port()
					tokenEnvMap["caddress"] = vaultUrl.Host + cVaultPort
				}
			} else {
				return nil, errors.New("Certification Vault Url required.")
			}

			if cToken, tokenOK := data.GetOk("ctoken"); tokenOK {
				tokenEnvMap["ctoken"] = cToken
			} else {
				return nil, errors.New("Certification Vault token required.")
			}

			if !strings.HasSuffix(vaultHost, GetVaultPort()) {
				// Missing port.
				vaultHost = vaultHost + ":" + GetVaultPort()
			}

			pluginConfig := map[string]interface{}{}
			pluginConfig = buildopts.ProcessPluginEnvConfig(pluginConfig) //contains logNamespace for InitVaultMod
			if pluginConfig == nil {
				logger.Println("Error: " + errors.New("Could not find plugin config").Error())
				return logical.ErrorResponse("Failed to find config for TrcUpdate."), nil
			}

			hostName, hostNameErr := os.Hostname()
			if hostNameErr != nil {
				logger.Println("Error: " + hostNameErr.Error() + " - 1")
				return logical.ErrorResponse("Failed to find hostname"), nil
			} else if hostName == "" {
				logger.Println("Error: " + errors.New("Found empty hostname").Error())
				return logical.ErrorResponse("Found empty hostname"), nil
			}
			hostRegion := coreopts.GetRegion(hostName)

			pluginConfig["env"] = req.Path
			pluginConfig["vaddress"] = tokenEnvMap["caddress"].(string)
			pluginConfig["token"] = tokenEnvMap["ctoken"].(string)
			pluginConfig["regions"] = []string{hostRegion}
			cConfig, cMod, cVault, err := eUtils.InitVaultModForPlugin(pluginConfig, logger)
			if err != nil {
				logger.Println("Error: " + err.Error() + " - 1")
				logger.Println("Failed to init mod for deploy update")
				return logical.ErrorResponse("Failed to create config for TrcUpdate."), nil
			}
			if cMod != nil {
				defer cMod.Release()
			}
			cMod.Env = req.Path
			logger.Println("TrcUpdate getting plugin settings for env: " + req.Path)

			cMod.SectionName = "trcplugin"
			cMod.SectionKey = "/Index/"
			cMod.SubSectionValue = plugin.(string)

			properties, err := trcvutils.NewProperties(cConfig, cVault, cMod, cMod.Env, "TrcVault", "Certify")
			if err != nil {
				logger.Println("Error: " + err.Error())
				return logical.ErrorResponse("Failed to read previous plugin status from vault - 1"), nil
			}

			writeMap, _ := properties.GetPluginData(hostRegion, "Certify", "config", logger)
			if writeMap == nil {
				logger.Println("Failed to read previous plugin status from vault")
				logger.Println("Error: " + err.Error())
				return logical.ErrorResponse("Failed to read previous plugin status from vault - 2"), nil
			}
			logger.Println("TrcUpdate Checking sha")

			if _, ok := writeMap["trcsha256"]; !ok {
				logger.Println("Failed to read previous plugin sha from vault")
				return logical.ErrorResponse("Failed to read previous plugin sha from vault"), nil
			}
			cMod.Close()
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
			tokenEnvMap["vaddress"] = vaddr.(string)
			if err == nil {
				vaultPort = vaultUrl.Port()
			}
		} else {
			return nil, errors.New("Vault Create Url required.")
		}

		tokenEnvMap["vaddress"] = vaultHost

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
		logger.Println("Update Pushing env: " + tokenEnvMap["env"].(string))
		PushEnv(tokenEnvMap)

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
	}

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
				"caddress": {
					Type:        framework.TypeString,
					Description: "Vault Url for plugin certification purposes.",
				},
				"ctoken": {
					Type:        framework.TypeString,
					Description: "Vault token for plugin certification purposes.",
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

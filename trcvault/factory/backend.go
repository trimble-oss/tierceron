package factory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"tierceron/trcconfig/utils"
	vscutils "tierceron/trcvault/util"
	eUtils "tierceron/utils"
	helperkv "tierceron/vaulthelper/kv"

	kv "github.com/hashicorp/vault-plugin-secrets-kv"
	"gopkg.in/yaml.v2"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const CONFIG_PATH = "config"

var _ logical.Factory = TrcFactory

var logger *log.Logger

func Init(l *log.Logger) {
	eUtils.InitHeadless(true)
	logger = l

	tokenEnvChan = make(chan map[string]interface{}, 5)

	// Set up a table process runner.
	initVaultHost()

	go func() {
		for {
			select {
			case tokenEnvMap := <-tokenEnvChan:

				logger.Println("Config engine init begun: " + tokenEnvMap["env"].(string))
				pecError := ProcessEnvConfig(tokenEnvMap)

				if pecError != nil {
					logger.Println("Bad configuration data for env: " + tokenEnvMap["env"].(string) + " error: " + pecError.Error())
				}
				logger.Println("Config engine setup complete for env: " + tokenEnvMap["env"].(string))
			}
		}

	}()
}

var KvInitialize func(context.Context, *logical.InitializationRequest) error
var KvCreate framework.OperationFunc
var KvUpdate framework.OperationFunc

var vaultHost string // Plugin will only communicate locally with a vault instance.
var environments []string = []string{"dev", "QA"}
var environmentConfigs map[string]*EnvConfig

var tokenEnvChan chan map[string]interface{}

func initVaultHost() error {
	if vaultHost == "" {
		logger.Println("Begin finding vault.")

		v, lvherr := vscutils.GetLocalVaultHost(true, logger)
		if lvherr != nil {
			logger.Println("Couldn't find local vault: " + lvherr.Error())
			return lvherr
		} else {
			logger.Println("Found vault at: " + v)
		}
		vaultHost = v
		logger.Println("End finding vault.")
	}
	return nil
}

func parseToken(e *logical.StorageEntry) (map[string]interface{}, error) {
	tokenMap := map[string]interface{}{}
	type tokenWrapper struct {
		Token string `json:"token,omitempty"`
	}
	tokenConfig := tokenWrapper{}
	e.DecodeJSON(&tokenConfig)
	tokenMap["token"] = tokenConfig.Token

	return tokenMap, nil
}

func populateTrcVaultDbConfigs(addr string, token string, env string) error {
	logger.Println("Begin populateTrcVaultDbConfigs for env: " + env)
	var goMod *helperkv.Modifier
	goMod, errModInit := helperkv.NewModifier(true, token, addr, env, []string{}, logger)
	goMod.Env = env

	if errModInit != nil {
		logger.Println("Vault connect failure")
		return errModInit
	}

	configuredTemplate, _, _, ctErr := utils.ConfigTemplate(goMod, "/trc_templates/TrcVault/Database/config.tmpl", true, "TrcVault", "Database", false, true, logger)
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

	vaultEnvConfig.Env = env
	environmentConfigs[env] = &vaultEnvConfig
	logger.Println("Config created for env: " + env)

	logger.Println("End populateTrcVaultDbConfigs")
	return nil
}

func ProcessEnvConfig(config map[string]interface{}) error {
	env, eOk := config["env"]
	if !eOk || env.(string) == "" {
		logger.Println("Bad configuration data.  Missing env.")
		return errors.New("missing token")
	}

	token, rOk := config["token"]
	if !rOk || token.(string) == "" {
		logger.Println("Bad configuration data for env: " + env.(string) + ".  Missing token.")
		return errors.New("missing token")
	}

	ptvError := populateTrcVaultDbConfigs(vaultHost, token.(string), env.(string))
	if ptvError != nil {
		logger.Println("Bad configuration data for env: " + env.(string) + ".  error: " + ptvError.Error())
		return ptvError
	}

	// Adding additional configurations the plugin needs to know which tables to process
	// and where to get additional configurations.
	config["templatePath"] = []string{
		"trc_templates/TenantDatabase/TenantConfiguration/TenantConfiguration.tmpl",           // implemented
		"trc_templates/TenantDatabase/SpectrumEnterpriseConfig/SpectrumEnterpriseConfig.tmpl", // not yet implemented.
		//		"trc_templates/TenantDatabase/KafkaTableConfiguration/KafkaTableConfiguration.tmpl",   // not yet implemented.
		//		"trc_templates/TenantDatabase/Mysqlfile/Mysqlfile.tmpl",                               // not yet implemented.
	}
	config["connectionPath"] = "trc_templates/TrcVault/Database/config.tmpl"

	vscutils.ProcessTables(config, logger)

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
				tokenEnvChan <- tokenMap
			}
		}
	}

	if KvInitialize != nil {
		logger.Println("Entering KvInitialize...")
		return KvInitialize(ctx, req)
	}

	logger.Println("TrcInitialize complete.")
	ctx.Done()
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
	ctx.Done()

	return nil, nil
}

func TrcCreate(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	logger.Println("TrcCreateUpdate")
	tokenEnvMap := map[string]interface{}{}
	tokenEnvMap["env"] = req.Path

	if token, tokenOk := data.GetOk("token"); tokenOk {
		tokenEnvMap["token"] = token
	} else {
		return nil, errors.New("Token required.")
	}

	tokenEnvMap["address"] = vaultHost

	key := req.Path //data.Get("path").(string)
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

	tokenEnvChan <- tokenEnvMap
	ctx.Done()

	return &logical.Response{
		Data: map[string]interface{}{
			"message": "Token created.",
		},
	}, nil
}

func TrcUpdate(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	logger.Println("TrcUpdate")
	tokenEnvMap := map[string]interface{}{}
	tokenEnvMap["env"] = req.Path

	if token, tokenOk := data.GetOk("token"); tokenOk {
		tokenEnvMap["token"] = token
	} else {
		return nil, errors.New("Token required.")
	}
	tokenEnvMap["address"] = vaultHost

	key := req.Path
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

	tokenEnvChan <- tokenEnvMap
	ctx.Done()

	return &logical.Response{
		Data: map[string]interface{}{
			"message": "Token updated.",
		},
	}, nil
}

// TrcFactory configures and returns Mock backends
func TrcFactory(ctx context.Context, conf *logical.BackendConfig) (logical.Backend, error) {
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
		bkv.(*kv.PassthroughBackend).Paths[0].Callbacks[logical.CreateOperation] = TrcCreate
		bkv.(*kv.PassthroughBackend).Paths[0].Callbacks[logical.UpdateOperation] = TrcUpdate
	}

	bkv.(*kv.PassthroughBackend).Paths = []*framework.Path{
		&framework.Path{
			Pattern:         "(dev|QA|staging|prod)",
			HelpSynopsis:    "Configure an access token.",
			HelpDescription: "Use this endpoint to configure the auth tokens required by trcvault.",

			Fields: map[string]*framework.FieldSchema{
				"token": &framework.FieldSchema{
					Type:        framework.TypeString,
					Description: "Token used for specified environment.",
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

	return bkv, err
}

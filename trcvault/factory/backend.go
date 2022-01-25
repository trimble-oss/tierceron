package factory

import (
	"context"
	"errors"
	"log"
	"tierceron/trcconfig/utils"
	vscutils "tierceron/trcvault/util"
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
	logger = l
}

var KvInitialize func(context.Context, *logical.InitializationRequest) error
var KvCreateUpdate framework.OperationFunc

var vaultHost string // Plugin will only communicate locally with a vault instance.
var environments []string = []string{"dev", "QA"}
var environmentConfigs map[string]*EnvConfig

func initVaultHost() error {
	if vaultHost == "" {
		logger.Println("Begin finding vault.")

		v, lvherr := vscutils.GetLocalVaultHost()
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
	goMod, errModInit := helperkv.NewModifier(true, token, addr, env, []string{})
	goMod.Env = env

	if errModInit != nil {
		logger.Println("Vault connect failure")
		return errModInit
	}

	configuredTemplate, _, _, ctErr := utils.ConfigTemplate(goMod, "/trc_templates/TrcVault/Database/config.tmpl", true, "TrcVault", "Database", false, true)
	if ctErr != nil {
		logger.Println("Config template lookup failure: " + ctErr.Error())
		return ctErr
	}

	var vaultEnvConfig EnvConfig

	errYaml := yaml.Unmarshal([]byte(configuredTemplate), &vaultEnvConfig)
	if errYaml != nil {
		logger.Println("Vault config lookup failure")
		return errYaml
	}

	vaultEnvConfig.Env = env
	environmentConfigs[env] = &vaultEnvConfig
	logger.Println("Config created for env: " + env)

	logger.Println("End populateTrcVaultDbConfigs")
	return nil
}

func ProcessEnvConfig(env string, config map[string]interface{}) error {

	token, rOk := config["token"]
	if !rOk || token.(string) == "" {
		logger.Println("Bad configuration data for env: " + env + ".  Missing token.")
		return errors.New("missing token")
	}

	ptvError := populateTrcVaultDbConfigs(vaultHost, token.(string), env)
	if ptvError != nil {
		logger.Println("Bad configuration data for env: " + env + ".  error: " + ptvError.Error())
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

	vscutils.ProcessTables(env, config)

	return nil
}

func TrcInitialize(ctx context.Context, req *logical.InitializationRequest) error {
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
			continue
		}
		initVaultHost()

		pecError := ProcessEnvConfig(env, tokenMap)

		if pecError != nil {
			logger.Println("Bad configuration data for env: " + env + " error: " + pecError.Error())
			continue
		}
	}

	if KvInitialize != nil {
		return KvInitialize(ctx, req)
	}

	logger.Println("TrcInitialize complete.")
	return nil
}

func TrcCreateUpdate(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	logger.Println("TrcCreateUpdate")
	initVaultHost()

	path := data.Get("path")

	switch path.(string) {
	case "dev":
		pecError := ProcessEnvConfig("dev", data.Raw)
		if pecError != nil {
			return nil, pecError
		}
	case "QA":
		pecError := ProcessEnvConfig("QA", data.Raw)
		if pecError != nil {
			return nil, pecError
		}
	default:
		break
	}
	return KvCreateUpdate(ctx, req, data)
}

// TrcFactory configures and returns Mock backends
func TrcFactory(ctx context.Context, conf *logical.BackendConfig) (logical.Backend, error) {
	logger.Println("Factory initialization begun.")
	environmentConfigs = map[string]*EnvConfig{}

	bkv, err := kv.Factory(ctx, conf)

	KvInitialize = bkv.(*kv.PassthroughBackend).InitializeFunc
	bkv.(*kv.PassthroughBackend).InitializeFunc = TrcInitialize

	KvCreateUpdate = bkv.(*kv.PassthroughBackend).Paths[0].Callbacks[logical.CreateOperation]
	bkv.(*kv.PassthroughBackend).Paths[0].Callbacks[logical.CreateOperation] = TrcCreateUpdate
	bkv.(*kv.PassthroughBackend).Paths[0].Callbacks[logical.UpdateOperation] = TrcCreateUpdate

	logger.Println("Factory initialization complete.")

	return bkv, err
}

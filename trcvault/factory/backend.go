package factory

import (
	"context"
	"encoding/json"
	"log"
	"tierceron/trcconfig/utils"
	trcvutil "tierceron/trcvault/util"
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
var KvReadFunction framework.OperationFunc

var vaultHost string // Plugin will only communicate locally with a vault instance.
var environments []string = []string{"dev", "QA"}
var environmentConfigs map[string]*EnvConfig

func parseToken(envConfigData []byte) (map[string]interface{}, error) {
	tokenMap := map[string]interface{}{}
	type tokenWrapper struct {
		token string `json:"token,omitempty"`
	}
	tokenConfig := tokenWrapper{}
	jsonErr := json.Unmarshal(envConfigData, &tokenConfig)
	if jsonErr != nil {
		return nil, jsonErr
	}
	tokenMap["token"] = tokenConfig.token

	return tokenMap, nil
}

func populateTrcVaultDbConfigs(addr string, token string, env string) error {
	var goMod *helperkv.Modifier
	goMod, errModInit := helperkv.NewModifier(true, token, addr, "", []string{})
	if errModInit != nil {
		return errModInit
	}

	configuredTemplate, _, _ := utils.ConfigTemplate(goMod, "/trc_templates/TrcVault/Database/config.tmpl", true, "TrcVault", "Database", false, true)

	var vaultEnvConfig EnvConfig

	errYaml := yaml.Unmarshal([]byte(configuredTemplate), &vaultEnvConfig)
	if errYaml != nil {
		return errYaml
	}
	vaultEnvConfig.env = env
	environmentConfigs[env] = &vaultEnvConfig
	return nil
}

func processEnvConfig(env string, config map[string]interface{}) error {
	token, rOk := config["token"]
	if !rOk {
		logger.Println("Bad configuration data for env: " + env + ".  Missing token.")
	}

	// TODO: Pull this addr...  do we go with localhost???  But that won't work with certs
	// and we'll have to go insecure...
	// if vaultHost == "" {
	// 	v, lvherr := trcvutil.GetLocalVaultHost()
	// 	if lvherr != nil {
	// 		logger.Println("Couldn't find local vault: " + lvherr.Error())
	// 		return lvherr
	// 	}
	// 	vaultHost = v
	// }

	ptvError := populateTrcVaultDbConfigs(vaultHost, token.(string), env)
	if ptvError != nil {
		logger.Println("Bad configuration data for env: " + env + ".  error: " + ptvError.Error())
		return ptvError
	}

	//
	// TODO: kick off singleton of enterprise registration...
	// 1. ETL from mysql -> vault?  Either in memory or mysql->file->Vault
	// 2. Pull enterprises from vault --> local queryable manageable mysql db.
	// 3. Query each enterprise and look for eid.
	// 4. If no eid, then -- register enterprise...
	//     a. Query Team (one or multiple)  See auto registration.
	//        -- we will probably need to update config.tmpl with more configs for
	//           interacting with Team.
	//     b. Update TrcDb enterprise row with eid returned by sequence of Team calls.
	//     c. Update TrcDb enterprise row back to vault.
	//  Expose mysql port and begin testing for load and stability.
	//     d. Optionally update TrcDb enterprise back to mysql.
	//
	return nil
}

func TrcInitialize(ctx context.Context, req *logical.InitializationRequest) error {

	if vaultHost == "" {
		v, lvherr := trcvutil.GetLocalVaultHost()
		if lvherr != nil {
			logger.Println("Couldn't find local vault: " + lvherr.Error())
			return lvherr
		}
		vaultHost = v
	}

	for _, env := range environments {
		tokenData, sgErr := req.Storage.Get(ctx, "config/"+env)

		if sgErr != nil || tokenData == nil {
			logger.Println("Missing configuration data for env: " + env + " error: " + sgErr.Error())
			continue
		}
		tokenMap, ptError := parseToken(tokenData.Value)
		if ptError != nil {
			logger.Println("Bad configuration data for env: " + env + " error: " + ptError.Error())
			continue
		}

		pecError := processEnvConfig(env, tokenMap)

		if pecError != nil {
			logger.Println("Bad configuration data for env: " + env + " error: " + pecError.Error())
			continue
		}

		logger.Println("Config created for env: " + env)
	}

	if KvInitialize != nil {
		return KvInitialize(ctx, req)
	}

	logger.Println("TrcInitialize complete.")
	return nil
}

func TrcCreateUpdate(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	logger.Println("TrcCreateUpdate")

	path := data.Get("path")

	switch path {
	case "config/dev":
		pecError := processEnvConfig("dev", data.Raw)
		if pecError != nil {
			return nil, pecError
		}
	case "config/QA":
		pecError := processEnvConfig("QA", data.Raw)
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

	bkv, err := kv.Factory(ctx, conf)

	KvInitialize = bkv.(*kv.PassthroughBackend).InitializeFunc
	bkv.(*kv.PassthroughBackend).InitializeFunc = TrcInitialize

	KvCreateUpdate = bkv.(*kv.PassthroughBackend).Paths[0].Callbacks[logical.CreateOperation]
	KvReadFunction = bkv.(*kv.PassthroughBackend).Paths[0].Callbacks[logical.ReadOperation]

	bkv.(*kv.PassthroughBackend).Paths[0].Callbacks[logical.CreateOperation] = TrcCreateUpdate

	logger.Println("Factory initialization complete.")

	return bkv, err
}

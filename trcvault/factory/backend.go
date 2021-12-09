package factory

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"tierceron/trcconfig/utils"
	trcvutil "tierceron/trcvault/util"
	helperkv "tierceron/vaulthelper/kv"

	"github.com/davecgh/go-spew/spew"
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

		v, lvherr := trcvutil.GetLocalVaultHost()
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
	logger.Println("Begin populateTrcVaultDbConfigs for env: " + env)
	var goMod *helperkv.Modifier
	goMod, errModInit := helperkv.NewModifier(true, token, addr, env, []string{})
	goMod.Env = env

	if errModInit != nil {
		logger.Println("Vault connect failure")
		return errModInit
	}

	logger.Println("Begin config lookup: " + goMod.Env)
	configuredTemplate, _, _, ctErr := utils.ConfigTemplate(goMod, "/trc_templates/TrcVault/Database/config.tmpl", true, "TrcVault", "Database", false, true)
	if ctErr != nil {
		logger.Println("Config template lookup failure: " + spew.Sdump(ctErr))
		return ctErr
	}

	logger.Println("End config lookup")
	logger.Println(spew.Sdump(configuredTemplate))
	configuredTemplate = strings.ReplaceAll(configuredTemplate, "\\n", "\n")
	logger.Println(spew.Sdump(configuredTemplate))

	var vaultEnvConfig EnvConfig

	errYaml := yaml.Unmarshal([]byte(configuredTemplate), &vaultEnvConfig)
	if errYaml != nil {
		logger.Println("Vault config lookup failure")
		return errYaml
	}
	logger.Println(spew.Sdump(vaultEnvConfig))

	vaultEnvConfig.env = env
	environmentConfigs[env] = &vaultEnvConfig
	logger.Println(spew.Sdump(vaultEnvConfig))
	logger.Println("End populateTrcVaultDbConfigs")
	return nil
}

func processEnvConfig(env string, config map[string]interface{}) error {
	token, rOk := config["token"]
	if !rOk || token.(string) == "" {
		logger.Println("Bad configuration data for env: " + env + ".  Missing token.")
		return errors.New("missing token")
	}
	logger.Println("Token for env: " + env + ".  token: " + spew.Sdump(token))

	ptvError := populateTrcVaultDbConfigs(vaultHost, token.(string), env)
	if ptvError != nil {
		logger.Println("Bad configuration data for env: " + env + ".  error: " + ptvError.Error())
		return ptvError
	}

	//
	// TODO: kick off singleton of enterprise registration...
	// 1. ETL from mysql -> vault?  Either in memory or mysql->file->Vault
	//
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
	for _, env := range environments {
		logger.Println("Processing env: " + env)
		tokenData, sgErr := req.Storage.Get(ctx, "vaultdb/"+env)

		if sgErr != nil || tokenData == nil {
			if sgErr != nil {
				logger.Println("Missing configuration data for env: " + env + " error: " + sgErr.Error())
			} else {
				logger.Println("Missing configuration data for env: " + env)
			}
			continue
		}
		logger.Println("Parsing token for env: " + env)
		tokenMap, ptError := parseToken(tokenData.Value)
		if ptError != nil {
			logger.Println("Bad configuration data for env: " + env + " error: " + ptError.Error())
			continue
		}
		initVaultHost()

		logger.Println("Getting configs for env: " + env)
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
	initVaultHost()
	environmentConfigs = map[string]*EnvConfig{}

	path := data.Get("path")

	switch path.(string) {
	case "dev":
		pecError := processEnvConfig("dev", data.Raw)
		if pecError != nil {
			return nil, pecError
		}
	case "QA":
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
	bkv.(*kv.PassthroughBackend).Paths[0].Callbacks[logical.CreateOperation] = TrcCreateUpdate
	bkv.(*kv.PassthroughBackend).Paths[0].Callbacks[logical.UpdateOperation] = TrcCreateUpdate

	logger.Println("Factory initialization complete.")

	return bkv, err
}

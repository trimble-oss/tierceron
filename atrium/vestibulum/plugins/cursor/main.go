package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/trimble-oss/tierceron/atrium/buildopts/flowcoreopts"
	"github.com/trimble-oss/tierceron/atrium/buildopts/flowopts"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcshbase"
	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/cursoropts"
	"github.com/trimble-oss/tierceron/buildopts/deployopts"
	"github.com/trimble-oss/tierceron/buildopts/harbingeropts"
	memonly "github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/saltyopts"
	"github.com/trimble-oss/tierceron/buildopts/tcopts"
	"github.com/trimble-oss/tierceron/buildopts/xencryptopts"
	"github.com/trimble-oss/tierceron/pkg/capauth"
	"github.com/trimble-oss/tierceron/pkg/core"
	tiercerontls "github.com/trimble-oss/tierceron/pkg/tls"

	eUtils "github.com/trimble-oss/tierceron/pkg/utils"

	"github.com/hashicorp/go-hclog"
	kv "github.com/hashicorp/vault-plugin-secrets-kv"
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/hashicorp/vault/sdk/plugin"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/pluginutil"
	"golang.org/x/sys/unix"
)

type trcshBackend struct{}

func (b *trcshBackend) Read(path string) (*api.Secret, error) {
	// Implement logic to read secrets based on the path
	fmt.Println("Reading secret from path:", path)
	// ...
	return nil, nil
}

func (b *trcshBackend) Cleanup(ctx context.Context) {
	// Perform any necessary cleanup tasks here
	fmt.Println("Cleaning up backend")
}

func GenerateSchema(fields map[string]string) map[string]*framework.FieldSchema {
	schema := map[string]*framework.FieldSchema{}
	for key, value := range fields {
		schema[key] = &framework.FieldSchema{
			Type:        framework.TypeString,
			Description: value,
		}
	}
	return schema
}

var cursorFields map[string]string
var logger *log.Logger
var pluginConfig map[string]interface{}

var createUpdateFunc func(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) = func(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	pluginName := cursoropts.BuildOptions.GetPluginName()
	logger.Printf("%s CreateUpdate\n", pluginName)

	key := req.Path //data.Get("path").(string)
	if key == "" {
		return logical.ErrorResponse("missing path"), nil
	}

	// Check that some fields are given
	if len(req.Data) == 0 {
		//ctx.Done()
		return logical.ErrorResponse("missing data fields"), nil
	}

	tapMap := map[string]*string{}
	for _, cursor := range cursorFields {
		tapMap[cursor] = pluginConfig[cursor].(*string)
	}

	// JSON encode the data
	buf, err := json.Marshal(tapMap)
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

	logger.Printf("%s CreateUpdate complete\n", pluginName)

	return &logical.Response{
		Data: map[string]interface{}{
			"message": "Cursor updated",
		},
	}, nil
}

func main() {
	if memonly.IsMemonly() {
		mLockErr := unix.Mlockall(unix.MCL_CURRENT | unix.MCL_FUTURE)
		if mLockErr != nil {
			fmt.Println(mLockErr)
			os.Exit(-1)
		}
	}
	buildopts.NewOptionsBuilder(buildopts.LoadOptions())
	coreopts.NewOptionsBuilder(coreopts.LoadOptions())
	deployopts.NewOptionsBuilder(deployopts.LoadOptions())
	flowcoreopts.NewOptionsBuilder(flowcoreopts.LoadOptions())
	flowopts.NewOptionsBuilder(flowopts.LoadOptions())
	harbingeropts.NewOptionsBuilder(harbingeropts.LoadOptions())
	tcopts.NewOptionsBuilder(tcopts.LoadOptions())
	xencryptopts.NewOptionsBuilder(xencryptopts.LoadOptions())
	saltyopts.NewOptionsBuilder(saltyopts.LoadOptions())
	cursoropts.NewOptionsBuilder(cursoropts.LoadOptions())
	tiercerontls.InitRoot()

	eUtils.InitHeadless(true)
	logFile := cursoropts.BuildOptions.GetLogPath()
	f, logErr := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if logErr != nil {
		logFile = "./trccursor.log"
		f, logErr = os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	}
	logger = log.New(f, fmt.Sprintf("[%s]", cursoropts.BuildOptions.GetPluginName()), log.LstdFlags)
	eUtils.CheckError(&core.CoreConfig{
		ExitOnFailure: true,
		Log:           logger,
	}, logErr, true)
	logger.Println("Beginning plugin startup.")
	buildopts.BuildOptions.SetLogger(logger.Writer())

	if os.Getenv(api.PluginMetadataModeEnv) == "true" {
		logger.Println("Metadata init.")
	} else {
		logger.Println("Plugin Init begun.")
		cursorFields = cursoropts.BuildOptions.GetCursorFields()

		// Initialize configs for curator.
		trcshDriverConfig, err := trcshbase.TrcshInitConfig("dev", "", "", true, logger)
		eUtils.CheckError(&core.CoreConfig{
			ExitOnFailure: true,
			Log:           logger,
		}, err, true)

		// Get secrets from curator.
		for secretFieldKey, _ := range cursorFields {
			secretFieldValue, err := capauth.PenseQuery(trcshDriverConfig, secretFieldKey)
			if err != nil {
				logger.Println("Failed to retrieve wanted key: %s\n", secretFieldKey)
			}
			pluginConfig[secretFieldKey] = secretFieldValue
		}

		// Clean up tap
		e := os.Remove(fmt.Sprintf("%strcsnap.sock", cursoropts.BuildOptions.GetCapPath()))
		if e != nil {
			logger.Println("Unable to refresh socket.  Uneccessary.")
		}

		// Establish tap and feather.
		pluginutil.PluginTapFeatherInit(trcshDriverConfig, pluginConfig)
	}

	apiClientMeta := api.PluginAPIClientMeta{}
	flags := apiClientMeta.FlagSet()

	logger.Println("Running plugin with cert validation...")

	args := os.Args
	args = append(args, fmt.Sprintf("--client-cert=%s", "../certs/serv_cert.pem"))
	args = append(args, fmt.Sprintf("--client-key=%s", "../certs/serv_key.pem"))

	argErr := flags.Parse(args[1:])
	if argErr != nil {
		logger.Fatal(argErr)
	}
	logger.Print("Warming up...")

	tlsConfig := apiClientMeta.GetTLSConfig()
	tlsProviderFunc := api.VaultPluginTLSProvider(tlsConfig)

	pluginName := cursoropts.BuildOptions.GetPluginName()

	logger.Print("Starting server...")
	err := plugin.Serve(&plugin.ServeOpts{
		BackendFactoryFunc: func(ctx context.Context, config *logical.BackendConfig) (logical.Backend, error) {
			// Access backend configuration if needed
			fmt.Println("Backend configuration:", config)

			bkv, err := kv.Factory(ctx, config)

			bkv.(*kv.PassthroughBackend).Paths = []*framework.Path{
				{
					Pattern:         "(dev|QA|staging|prod)",
					HelpSynopsis:    "Configure an access token.",
					HelpDescription: "Use this endpoint to configure the auth tokens required by trcvault.",

					Fields: GenerateSchema(cursorFields),
					Callbacks: map[logical.Operation]framework.OperationFunc{
						logical.ReadOperation:   bkv.(*kv.PassthroughBackend).Paths[0].Callbacks[logical.ReadOperation],
						logical.CreateOperation: createUpdateFunc,
						logical.UpdateOperation: createUpdateFunc,
					},
				},
			}

			if err != nil {
				logger.Print("%s had an error: %v", pluginName, err.Error())
			}

			return bkv, err
		},
		TLSProviderFunc: tlsProviderFunc,
		Logger: hclog.New(&hclog.LoggerOptions{
			Level:      hclog.Trace,
			Output:     logger.Writer(),
			JSONFormat: false,
		}),
	})
	if err != nil {
		logger.Fatal("Plugin shutting down")
	}
}

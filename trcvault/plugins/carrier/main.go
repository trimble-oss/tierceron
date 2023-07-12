package main

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strings"

	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/trcflow/deploy"
	"github.com/trimble-oss/tierceron/trcvault/carrierfactory"
	"github.com/trimble-oss/tierceron/trcvault/opts/insecure"
	memonly "github.com/trimble-oss/tierceron/trcvault/opts/memonly"
	eUtils "github.com/trimble-oss/tierceron/utils"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/sdk/plugin"
	"golang.org/x/sys/unix"
)

func main() {
	if memonly.IsMemonly() {
		mLockErr := unix.Mlockall(unix.MCL_CURRENT | unix.MCL_FUTURE)
		if mLockErr != nil {
			fmt.Println(mLockErr)
			os.Exit(-1)
		}
	}

	eUtils.InitHeadless(true)
	logFile := "/var/log/trcplugincarrier.log"
	if !memonly.IsMemonly() && insecure.IsInsecure() {
		logFile = "trcplugincarrier.log"
	}

	f, logErr := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if logErr != nil {
		logFile = "./trcplugincarrier.log"
		f, logErr = os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	}
	logger := log.New(f, "[trcplugincarrier]", log.LstdFlags)
	eUtils.CheckError(&eUtils.DriverConfig{Insecure: true, Log: logger, ExitOnFailure: true}, logErr, true)
	logger.Println("Beginning plugin startup.")
	buildopts.SetLogger(logger.Writer())
	defer func() {
		if e := recover(); e != nil {
			logger.Printf("%s: %s", e, debug.Stack())
		}
	}()
	carrierfactory.Init(coreopts.ProcessDeployPluginEnvConfig, deploy.PluginDeployEnvFlow, deploy.PluginDeployFlow, true, logger)

	apiClientMeta := api.PluginAPIClientMeta{}
	flags := apiClientMeta.FlagSet()

	args := os.Args
	vaultHost := carrierfactory.GetVaultHost()
	if strings.HasPrefix(vaultHost, buildopts.GetLocalVaultAddr()) {
		logger.Println("Running in developer mode with self signed certs.")
		args = append(args, "--tls-skip-verify=true")
	} else {
		logger.Println("Running plugin with cert validation...")
		args = append(args, fmt.Sprintf("--client-cert=%s", "/etc/opt/vault/certs/serv_cert.pem"))
		args = append(args, fmt.Sprintf("--client-key=%s", "/etc/opt/vault/certs/serv_key.pem"))
	}

	argErr := flags.Parse(args[1:])
	if argErr != nil {
		logger.Fatal(argErr)
	}
	logger.Print("Warming up...")

	tlsConfig := apiClientMeta.GetTLSConfig()
	tlsProviderFunc := api.VaultPluginTLSProvider(tlsConfig)

	logger.Print("Starting server...")
	err := plugin.Serve(&plugin.ServeOpts{
		BackendFactoryFunc: carrierfactory.TrcFactory,
		TLSProviderFunc:    tlsProviderFunc,
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

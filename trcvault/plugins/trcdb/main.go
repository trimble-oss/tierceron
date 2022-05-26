package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"tierceron/trcflow/flumen"
	"tierceron/trcvault/factory"
	"tierceron/trcvault/opts/insecure"
	memonly "tierceron/trcvault/opts/memonly"
	"tierceron/trcvault/opts/prod"
	eUtils "tierceron/utils"
	"tierceron/utils/mlock"

	tclib "VaultConfig.TenantConfig/lib"
	tcutil "VaultConfig.TenantConfig/util"
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
	logFile := "/var/log/trcpluginvault.log"
	if !prod.IsProd() && insecure.IsInsecure() {
		logFile = "trcpluginvault.log"
	}
	f, logErr := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	logger := log.New(f, "[trcpluginvault]", log.LstdFlags)
	eUtils.CheckError(&eUtils.DriverConfig{Insecure: true, Log: logger, ExitOnFailure: true}, logErr, true)

	tclib.SetLogger(func(query string, args ...interface{}) {
		logger.Println(query)
	})
	tclib.SetErrorLogger(logger.Writer())
	factory.Init(tcutil.ProcessPluginEnvConfig, flumen.ProcessFlows, true, logger)
	mlock.Mlock(logger)

	apiClientMeta := api.PluginAPIClientMeta{}
	flags := apiClientMeta.FlagSet()

	args := os.Args

	vaultHost := factory.GetVaultHost()

	if strings.HasPrefix(vaultHost, tcutil.GetLocalVaultAddr()) {
		logger.Println("Running in developer mode with self signed certs.")
		args = append(args, "--tls-skip-verify=true")
	} else {
		// TODO: this may not be needed...
		//	args = append(args, fmt.Sprintf("--ca-cert=", caPEM))
	}

	argErr := flags.Parse(args[1:])
	if argErr != nil {
		logger.Fatal(argErr)
	}
	logger.Print("Warming up...")

	tlsConfig := apiClientMeta.GetTLSConfig()
	tlsProviderFunc := api.VaultPluginTLSProvider(tlsConfig)

	err := plugin.Serve(&plugin.ServeOpts{
		BackendFactoryFunc: factory.TrcFactory,
		TLSProviderFunc:    tlsProviderFunc,
	})
	if err != nil {
		logger.Fatal("Plugin shutting down")
	}

}

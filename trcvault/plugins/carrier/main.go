package main

import (
	"fmt"
	"log"
	"os"
	"tierceron/trcflow/deploy"
	"tierceron/trcvault/factory"
	memonly "tierceron/trcvault/opts/memonly"
	vscutils "tierceron/trcvault/util"
	eUtils "tierceron/utils"
	"tierceron/utils/mlock"

	tclib "VaultConfig.TenantConfig/lib"
	tcutil "VaultConfig.TenantConfig/util"
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/sdk/plugin"
	"golang.org/x/sys/unix"
)

// TODO: Expose public Https api...

func main() {
	if memonly.IsMemonly() {
		mLockErr := unix.Mlockall(unix.MCL_CURRENT | unix.MCL_FUTURE)
		if mLockErr != nil {
			fmt.Println(mLockErr)
			os.Exit(-1)
		}
	}
	eUtils.InitHeadless(true)
	f, logErr := os.OpenFile("trcplugincarrier.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	logger := log.New(f, "[trcplugincarrier]", log.LstdFlags)
	eUtils.CheckError(&eUtils.DriverConfig{Insecure: true, Log: logger, ExitOnFailure: true}, logErr, true)
	logger.Println("Beginning plugin startup.")

	tclib.SetLogger(logger.Writer())
	factory.Init(tcutil.ProcessDeployPluginEnvConfig, deploy.PluginDeployFlow, true, logger)
	mlock.Mlock(logger)

	apiClientMeta := api.PluginAPIClientMeta{}
	flags := apiClientMeta.FlagSet()

	args := os.Args
	vaultHost, lvherr := vscutils.GetLocalVaultHost(false, logger)
	if lvherr != nil {
		logger.Println("Host lookup failure.")
		os.Exit(-1)
	}

	if vaultHost == tcutil.GetLocalVaultAddr() {
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

	logger.Print("Starting server...")
	err := plugin.Serve(&plugin.ServeOpts{
		BackendFactoryFunc: factory.TrcFactory,
		TLSProviderFunc:    tlsProviderFunc,
	})
	if err != nil {
		logger.Fatal("Plugin shutting down")
	}
}

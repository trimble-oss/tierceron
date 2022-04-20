package main

import (
	"fmt"
	"log"
	"os"
	"tierceron/trcflow/flumen"
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

func main() {
	if memonly.IsMemonly() {
		mLockErr := unix.Mlockall(unix.MCL_CURRENT | unix.MCL_FUTURE)
		if mLockErr != nil {
			fmt.Println(mLockErr)
			os.Exit(-1)
		}
	}
	f, logErr := os.OpenFile("trcvault.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	logger := log.New(f, "[trcvault]", log.LstdFlags)
	eUtils.CheckError(&eUtils.DriverConfig{Insecure: true, Log: logger, ExitOnFailure: true}, logErr, true)

	tclib.SetLogger(logger.Writer())
	factory.Init(tcutil.ProcessPluginEnvConfig, flumen.ProcessFlows, true, logger)
	mlock.Mlock(logger)

	apiClientMeta := api.PluginAPIClientMeta{}
	flags := apiClientMeta.FlagSet()

	args := os.Args

	// Set up a table process runner.
	vaultSequenceCompleteChan := make(chan bool, 1)
	var vaultHost string

	go func() {
		vaultHostChan := make(chan string, 1)
		vaultLookupErrChan := make(chan error, 1)
		vscutils.GetLocalVaultHost(false, vaultHostChan, vaultLookupErrChan, logger)
		select {
		case v := <-vaultHostChan:
			vaultHost = v
			factory.InitVaultHost(v)
			logger.Println("Found vault at: " + v)
		case lvherr := <-vaultLookupErrChan:
			logger.Println("Couldn't find local vault: " + lvherr.Error())
			os.Exit(-1)
		}
		vaultSequenceCompleteChan <- true
	}()

	go func() {
		<-vaultSequenceCompleteChan
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

		err := plugin.Serve(&plugin.ServeOpts{
			BackendFactoryFunc: factory.TrcFactory,
			TLSProviderFunc:    tlsProviderFunc,
		})
		if err != nil {
			logger.Fatal("Plugin shutting down")
		}
	}()

}

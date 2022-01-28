package main

import (
	"log"
	"os"
	"tierceron/trcvault/factory"
	vscutils "tierceron/trcvault/util"
	eUtils "tierceron/utils"
	"tierceron/utils/mlock"

	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/sdk/plugin"
)

func main() {
	f, logErr := os.OpenFile("trcvault.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	eUtils.CheckError(logErr, true)
	logger := log.New(f, "[trcvault]", log.LstdFlags)
	mlock.Mlock(logger)
	factory.Init(logger)

	apiClientMeta := api.PluginAPIClientMeta{}
	flags := apiClientMeta.FlagSet()

	args := os.Args
	vaultHost, lvherr := vscutils.GetLocalVaultHost(false, logger)
	if lvherr != nil {
		logger.Println("Host lookup failure.")
		os.Exit(-1)
	}

	if vaultHost == "https://vault.whoboot.org" {
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

	/*
		signalChannel := make(chan os.Signal, 1)
		signal.Notify(signalChannel,
			syscall.SIGHUP,
			syscall.SIGINT,
			syscall.SIGTERM,
			syscall.SIGQUIT)

		go func() {
			for {
				select {
				case s := <-signalChannel:
					logger.Println("Got signal:", s)
					logger.Println("Received shutdown presumably from vault.")
					os.Exit(0)
				}
			}
		}()
	*/

	err := plugin.Serve(&plugin.ServeOpts{
		BackendFactoryFunc: factory.TrcFactory,
		TLSProviderFunc:    tlsProviderFunc,
	})
	if err != nil {
		logger.Fatal("Plugin shutting down")
	}
}

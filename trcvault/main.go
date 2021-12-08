package main

import (
	"log"
	"os"
	"tierceron/trcvault/factory"
	eUtils "tierceron/utils"

	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/sdk/plugin"
)

func main() {
	// mLockErr := unix.Mlockall(unix.MCL_CURRENT | unix.MCL_FUTURE)
	// if mLockErr != nil {
	// 	fmt.Println(mLockErr)
	// 	os.Exit(-1)
	// }
	f, logErr := os.OpenFile("trcvault.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	eUtils.CheckError(logErr, true)
	logger := log.New(f, "[trcvault]", log.LstdFlags)
	factory.Init(logger)

	logger.Println("=============== Initializing Vault Tierceron Plugin ===============")

	apiClientMeta := api.PluginAPIClientMeta{}
	flags := apiClientMeta.FlagSet()
	argErr := flags.Parse(os.Args[1:])
	eUtils.LogErrorObject(argErr, logger, true)
	logger.Println("Vault Tierceron Plugin Args parsed")

	tlsConfig := apiClientMeta.GetTLSConfig()
	tlsProviderFunc := api.VaultPluginTLSProvider(tlsConfig)

	err := plugin.Serve(&plugin.ServeOpts{
		BackendFactoryFunc: factory.TrcFactory,
		TLSProviderFunc:    tlsProviderFunc,
	})
	eUtils.LogErrorObject(err, logger, true)
	logger.Println("=============== Vault Tierceron Plugin Initialization complete ===============")
}

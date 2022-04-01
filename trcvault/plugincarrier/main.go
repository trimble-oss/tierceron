package main

import (
	"fmt"
	"log"
	"os"
	"tierceron/trcvault/factory"
	memonly "tierceron/trcvault/opts/memonly"
	"tierceron/trcvault/util"
	vscutils "tierceron/trcvault/util"
	eUtils "tierceron/utils"
	"tierceron/utils/mlock"
	helperkv "tierceron/vaulthelper/kv"
	sys "tierceron/vaulthelper/system"

	tclib "VaultConfig.TenantConfig/lib"
	tcutil "VaultConfig.TenantConfig/util"
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/sdk/plugin"
	"golang.org/x/sys/unix"
)

func PluginDeployFlow(pluginConfig map[string]interface{}, logger *log.Logger) error {
	var config *eUtils.DriverConfig
	var vault *sys.Vault
	var goMod *helperkv.Modifier
	var err error

	//Grabbing configs
	config, goMod, vault, err = eUtils.InitVaultModForPlugin(pluginConfig, logger)
	if err != nil {
		eUtils.LogErrorMessage(config, "Could not access vault.  Failure to start.", false)
		return err
	}

	pluginToolConfig := util.GetPluginToolConfig(config, goMod)

	// 0. List all the plugins under Index/TrcVault/trcplugin

	// 1. For each plugin do the following:
	// Assert: we already have a plugin name
	// 1a. retrieve TrcVault/trcplugin/<theplugin>/Certify/trcsha256
	// 1b. Read and sha256 of /etc/opt/vault/plugins/<theplugin>
	// 1c. if vault sha256 != filesystem sha256.
	// 1.c.i. Download new image from ECR.
	// 1.c.ii. Sha256 of new executable.
	// 1.c.ii.- if Sha256 of new executable === sha256 from vault.
	//  Save new image over existing image in /etc/opt/vault/plugins/<theplugin>
	// 2a. Update vault setting copied=true...
	// 3. Update apiChannel so api returns true

	return nil
}

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

	tclib.SetLogger(logger.Writer())
	factory.Init(PluginDeployFlow, logger)
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

	err := plugin.Serve(&plugin.ServeOpts{
		BackendFactoryFunc: factory.TrcFactory,
		TLSProviderFunc:    tlsProviderFunc,
	})
	if err != nil {
		logger.Fatal("Plugin shutting down")
	}
}

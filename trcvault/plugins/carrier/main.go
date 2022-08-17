package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"tierceron/buildopts"
	"tierceron/trcflow/deploy"
	"tierceron/trcvault/factory"
	"tierceron/trcvault/opts/insecure"
	memonly "tierceron/trcvault/opts/memonly"
	eUtils "tierceron/utils"

	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/sdk/plugin"
	"golang.org/x/sys/unix"
)

func main() {
	os.Setenv("MADV_DONTNEED", "0")
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
	logger := log.New(f, "[trcplugincarrier]", log.LstdFlags)
	eUtils.CheckError(&eUtils.DriverConfig{Insecure: true, Log: logger, ExitOnFailure: true}, logErr, true)
	logger.Println("Beginning plugin startup.")

	buildopts.SetLogger(logger.Writer())
	factory.Init(buildopts.ProcessDeployPluginEnvConfig, deploy.PluginDeployFlow, true, logger)

	apiClientMeta := api.PluginAPIClientMeta{}
	flags := apiClientMeta.FlagSet()

	args := os.Args
	vaultHost := factory.GetVaultHost()
	if strings.HasPrefix(vaultHost, buildopts.GetLocalVaultAddr()) {
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

package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/trimble-oss/tierceron/atrium/vestibulum/trccarrier/carrierfactory"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcflow/deploy"
	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/pkg/core"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

var signalChannel chan os.Signal

// This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func main() {

	// Set up global signal capture.
	signalChannel = make(chan os.Signal, 1)
	signal.Notify(signalChannel,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	logFilePtr := flag.String("log", "./trcdeployplugin.log", "Output path for log file")
	tokenPtr := flag.String("token", "", "Vault access token")
	flag.Parse()

	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	logger := log.New(f, "[trcdbplugin]", log.LstdFlags)

	driverConfig := &eUtils.DriverConfig{
		CoreConfig: core.CoreConfig{
			ExitOnFailure: true,
			Log:           logger,
		},
	}
	eUtils.CheckError(&driverConfig.CoreConfig, err, true)

	//Grabbing configs
	envMap := buildopts.BuildOptions.GetTestDeployConfig(*tokenPtr)
	//envMap["vaddress"] = "vaultaddr"
	//envMap["token"] = "INSERT TOKEN HERE"
	carrierfactory.InitLogger(logger)
	//go carrierfactory.InitVaultHostRemoteBootstrap(envMap["vaddress"].(string))

	go carrierfactory.Init(coreopts.BuildOptions.ProcessDeployPluginEnvConfig, deploy.PluginDeployEnvFlow, deploy.PluginDeployFlow, true, logger)
	envMap["env"] = "QA"
	envMap["insecure"] = true
	envMap["syncOnce"] = &sync.Once{}
	carrierfactory.PushEnv(envMap)

	for {
		select {
		case <-signalChannel:
			eUtils.LogErrorMessage(&driverConfig.CoreConfig, "Receiving shutdown presumably from vault.", true)
			os.Exit(0)
		}
	}
}

package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"tierceron/trcflow/deploy"
	"tierceron/trcvault/factory"
	eUtils "tierceron/utils"

	tcutil "VaultConfig.TenantConfig/util"
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

	configDriver := &eUtils.DriverConfig{Log: logger, ExitOnFailure: true}
	eUtils.CheckError(configDriver, err, true)

	//Grabbing configs
	envMap := tcutil.GetTestConfig(*tokenPtr)

	go factory.Init(deploy.PluginDeployFlow, true, logger)
	for {
		select {
		case <-factory.ReadyForEnvs():
			factory.PushEnv(envMap)
		case <-signalChannel:
			eUtils.LogErrorMessage(configDriver, "Receiving shutdown presumably from vault.", true)
			os.Exit(0)
		}
	}
}

package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/trcflow/deploy"
	"github.com/trimble-oss/tierceron/trcvault/carrierfactory"
	eUtils "github.com/trimble-oss/tierceron/utils"
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
	envMap := buildopts.GetTestDeployConfig(*tokenPtr)

	go carrierfactory.Init(coreopts.ProcessDeployPluginEnvConfig, deploy.PluginDeployFlow, true, logger)
	envMap["env"] = "QA"
	envMap["insecure"] = true
	envMap["syncOnce"] = &sync.Once{}
	carrierfactory.PushEnv(envMap)

	for {
		select {
		case <-signalChannel:
			eUtils.LogErrorMessage(configDriver, "Receiving shutdown presumably from vault.", true)
			os.Exit(0)
		}
	}
}

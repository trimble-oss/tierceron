package main

import (
	"flag"
	"log"
	"os"

	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

// This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func main() {
	// Supported build flags:
	//    insecure harbinger tc testrunner ( mysql, testflow -- auto registration -- warning do not use!)
	logFilePtr := flag.String("log", "./trcgorillaz.log", "Output path for log file")
	//tokenPtr := flag.String("token", "", "Vault access Token")
	flag.Parse()

	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	logger := log.New(f, "[trcgorillaz]", log.LstdFlags)
	eUtils.CheckError(&coreconfig.CoreConfig{ExitOnFailure: true, Log: logger}, err, true)

	//pluginConfig := buildopts.BuildOptions.GetTestConfig(*tokenPtr, false)

	//trcflow.ProcessFlows(pluginConfig, logger)
}

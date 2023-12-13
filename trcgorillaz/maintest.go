package main

import (
	"flag"
	"log"
	"os"

	eUtils "github.com/trimble-oss/tierceron/utils"
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
	eUtils.CheckError(&eUtils.DriverConfig{Log: logger, ExitOnFailure: true}, err, true)

	//pluginConfig := buildopts.GetTestConfig(*tokenPtr, false)

	//trcflow.ProcessFlows(pluginConfig, logger)
}

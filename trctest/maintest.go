package main

import (
	"flag"
	"log"
	"os"
	trcflow "tierceron/trcflow/flumen"
	eUtils "tierceron/utils"

	tcutil "VaultConfig.TenantConfig/util"
)

// This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func main() {
	logFilePtr := flag.String("log", "./trcdbplugin.log", "Output path for log file")
	tokenPtr := flag.String("token", "", "Vault access token")
	flag.Parse()

	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	logger := log.New(f, "[trcdbplugin]", log.LstdFlags)
	eUtils.CheckError(&eUtils.DriverConfig{Log: logger, ExitOnFailure: true}, err, true)

	pluginConfig := tcutil.GetTestConfig(*tokenPtr, false)

	trcflow.ProcessFlows(pluginConfig, logger)
}

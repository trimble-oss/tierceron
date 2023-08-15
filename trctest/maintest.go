package main

import (
	"flag"
	"log"
	"os"

	"github.com/trimble-oss/tierceron/buildopts/testopts"
	trcflow "github.com/trimble-oss/tierceron/trcflow/flumen"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
	eUtils "github.com/trimble-oss/tierceron/utils"
	"github.com/trimble-oss/tierceron/utils/mlock"
)

// This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func main() {

	// Supported build flags:
	//    insecure harbinger tc testrunner ( mysql, testflow -- auto registration -- warning do not use!)
	logFilePtr := flag.String("log", "./trcdbplugin.log", "Output path for log file")
	tokenPtr := flag.String("token", "", "Vault access Token")
	flag.Parse()

	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	logger := log.New(f, "[trcdbplugin]", log.LstdFlags)
	eUtils.CheckError(&eUtils.DriverConfig{Log: logger, ExitOnFailure: true}, err, true)

	pluginConfig := testopts.GetTestConfig(*tokenPtr, false)
	pluginConfig["address"] = "https://vault.whoboot.org:8200"
	pluginConfig["vaddress"] = "https://vault.whoboot.org:8200"
	pluginConfig["token"] = "s.QuTHJxhDYjNWnVB083no275G" //s.WkYK920xf4EougSqq3E77MA1
	pluginConfig["env"] = "dev"
	pluginConfig["insecure"] = true

	if memonly.IsMemonly() {
		mlock.MunlockAll(nil)
		for _, value := range pluginConfig {
			if valueSlice, isValueSlice := value.([]string); isValueSlice {
				for _, valueEntry := range valueSlice {
					mlock.Mlock2(nil, &valueEntry)
				}
			} else if valueString, isValueString := value.(string); isValueString {
				mlock.Mlock2(nil, &valueString)
			}
		}
	}

	trcflow.ProcessFlows(pluginConfig, logger)
}

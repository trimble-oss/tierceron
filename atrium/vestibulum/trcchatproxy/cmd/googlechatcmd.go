package main

import (
	"flag"
	"log"
	"os"

	"github.com/trimble-oss/tierceron/pkg/core"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"

	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcchatproxy/pubsub"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcchatproxy/trcchat"
)

func main() {
	shutdown := make(chan bool)
	logFilePtr := flag.String("log", "./googlechatcmd.log", "Output path for log file")
	tokenPtr := flag.String("token", "", "Output path for log file")
	callerTokenPtr := flag.String("callerToken", "", "Output path for log file")
	flag.Parse()

	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	log.SetOutput(f)
	driverConfig := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			ExitOnFailure: true,
			Log:           log.Default(),
		},
	}
	eUtils.CheckError(driverConfig.CoreConfig, err, true)
	pubsub.CommonInit(true)
	trcchat.CommonInit(*tokenPtr, *callerTokenPtr)

	<-shutdown
}

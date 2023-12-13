package main

import (
	"flag"
	"log"
	"os"

	eUtils "github.com/trimble-oss/tierceron/utils"

	"github.com/trimble-oss/tierceron/trcchatproxy/pubsub"
	"github.com/trimble-oss/tierceron/trcchatproxy/trcchat"
)

func main() {
	shutdown := make(chan bool)
	logFilePtr := flag.String("log", "./googlechatcmd.log", "Output path for log file")
	tokenPtr := flag.String("token", "", "Output path for log file")
	callerTokenPtr := flag.String("callerToken", "", "Output path for log file")
	flag.Parse()

	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	log.SetOutput(f)
	configDriver := &eUtils.DriverConfig{Log: log.Default(), ExitOnFailure: true}
	eUtils.CheckError(configDriver, err, true)
	pubsub.CommonInit(true)
	trcchat.CommonInit(*tokenPtr, *callerTokenPtr)

	<-shutdown
}

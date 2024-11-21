package main

import (
	"embed"
	"flag"

	tbtcore "github.com/trimble-oss/tierceron/installation/trcshhive/trcshk/echo/tbtcore" // Update package path as needed
	// using old chat api because new cloud client library structs do not encode/decode correctly as json
)

func GetConfigPaths() []string {
	return tbtcore.GetConfigPaths()
}

func Init(properties *map[string]interface{}) {
	tbtcore.Init(properties)
}

//- go:embed tls/mashup.crt
//var mashupCert embed.FS

//- go:embed tls/mashup.key
//var mashupKey embed.FS

//go:embed local_config/application.yml
var configFile embed.FS

func main() {

	// authorizedSpace := flag.String("space", "", "Authorized space for posting asyncronous messages.")
	envPtr := flag.String("env", "dev", "Environment to configure") //envPtr :=
	logFilePtr := flag.String("log", "./echo.log", "Output path for log file")
	flag.Parse()

	// Running outside the hive, we must provide our own certs.
	tbtcore.EchoRunner( /*&mashupCert, &mashupKey,*/ nil, nil, &configFile, envPtr, logFilePtr)

	wait := make(chan bool)
	wait <- true
}

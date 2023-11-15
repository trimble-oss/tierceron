package main

import (
	"flag"
	"fmt"
	"os"

	plgtbase "github.com/trimble-oss/tierceron/trcvault/trcplgtoolbase"
)

// This executable automates the cerification of a plugin docker image.
func main() {
	fmt.Println("Version: " + "1.02")

	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.Usage = flag.Usage
	envPtr := flagset.String("env", "dev", "Environment to configure")
	addrPtr := flagset.String("addr", "", "API endpoint for the vault")
	tokenPtr := flagset.String("token", "", "Vault access token")
	regionPtr := flagset.String("region", "", "Region to be processed") //If this is blank -> use context otherwise override context.

	err := plgtbase.CommonMain(envPtr, addrPtr, tokenPtr, regionPtr, flagset, os.Args, nil)
	if err != nil {
		os.Exit(1)
	}

}

package main

import (
	"flag"
	"fmt"

	plgtbase "github.com/trimble-oss/tierceron/trcvault/trcplgtoolbase"
)

// This executable automates the cerification of a plugin docker image.
func main() {
	fmt.Println("Version: " + "1.01")

	envPtr := flag.String("env", "dev", "Environment to configure")
	addrPtr := flag.String("addr", "", "API endpoint for the vault")
	tokenPtr := flag.String("token", "", "Vault access token")
	regionPtr := flag.String("region", "", "Region to be processed") //If this is blank -> use context otherwise override context.

	plgtbase.CommonMain(envPtr, addrPtr, tokenPtr, regionPtr, nil)
}

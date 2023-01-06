package main

import (
	"flag"
	"fmt"

	"tierceron/trcconfigbase"
	"tierceron/trcvault/opts/memonly"
	"tierceron/utils/mlock"
)

// This assumes that the vault is completely new, and should only be run for the purpose
// of automating setup and initial seeding

func main() {
	if memonly.IsMemonly() {
		mlock.Mlock(nil)
	}
	fmt.Println("Version: " + "1.26")
	envPtr := flag.String("env", "dev", "Environment to configure")
	addrPtr := flag.String("addr", "", "API endpoint for the vault")

	trcconfigbase.CommonMain(envPtr, addrPtr, nil)
}

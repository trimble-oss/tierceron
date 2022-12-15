package main

import (
	"flag"
	"fmt"

	trcinitbase "tierceron/trcinitbase"
	"tierceron/trcvault/opts/memonly"
	"tierceron/utils/mlock"
)

// This assumes that the vault is completely new, and should only be run for the purpose
// of automating setup and initial seeding

func main() {
	if memonly.IsMemonly() {
		mlock.Mlock(nil)
	}
	fmt.Println("Version: " + "1.34")
	envPtr := flag.String("env", "dev", "Environment to be seeded")
	trcinitbase.CommonMain(envPtr, nil, nil)
}

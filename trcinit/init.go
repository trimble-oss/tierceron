package main

import (
	"flag"
	"fmt"

	trcinitbase "tierceron/trcinitbase"
)

// This assumes that the vault is completely new, and should only be run for the purpose
// of automating setup and initial seeding

func main() {
	fmt.Println("Version: " + "1.30")
	envPtr := flag.String("env", "dev", "Environment to be seeded")
	trcinitbase.CommonMain(envPtr, nil)
}

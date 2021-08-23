package main

import (
	"flag"
	"fmt"

	"Vault.Whoville/vaultinitbase"
)

// This assumes that the vault is completely new, and should only be run for the purpose
// of automating setup and initial seeding

func main() {
	fmt.Println("Version: " + "1.29")
	envPtr := flag.String("env", "dev", "Environment to be seeded")
	vaultinitbase.CommonMain(envPtr, nil)
}

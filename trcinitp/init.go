package main

import (
	"fmt"

	trcinitbase "tierceron/trcinitbase"

	configcore "VaultConfig.Bootstrap/configcore"
)

// This assumes that the vault is completely new, and should only be run for the purpose
// of automating setup and initial seeding

func main() {
	fmt.Println("Version: " + "1.5")
	env := "local"
	addr := configcore.VaultHostPort
	trcinitbase.CommonMain(&env, &addr)
}

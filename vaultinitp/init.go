package main

import (
	"fmt"

	"Vault.Whoville/vaultinitbase"
	configcore "VaultConfig.Bootstrap/configcore"
)

// This assumes that the vault is completely new, and should only be run for the purpose
// of automating setup and initial seeding

func main() {
	fmt.Println("Version: " + "1.5")
	env := "local"
	addr := configcore.VaultHostPort
	vaultinitbase.CommonMain(&env, &addr)
}

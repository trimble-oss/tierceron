package main

import (
	"fmt"

	"Vault.Whoville/vaultxbase"
	configcore "VaultConfig.Bootstrap/configcore"
)

// This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func main() {
	fmt.Println("Version: " + "1.4")
	env := "local"
	addr := configcore.VaultHostPort
	vaultxbase.CommonMain(&env, &addr)
}

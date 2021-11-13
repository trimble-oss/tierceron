package main

import (
	"fmt"

	"tierceron/trcx/xutil"
	"tierceron/trcxbase"

	configcore "VaultConfig.Bootstrap/configcore"
)

// This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func main() {
	fmt.Println("Version: " + "1.4")
	env := "local"
	addr := configcore.VaultHostPort
	trcxbase.CommonMain(nil, xutil.GenerateSeedsFromVault, &env, &addr, nil)
}

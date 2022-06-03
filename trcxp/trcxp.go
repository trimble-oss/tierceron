package main

import (
	"fmt"

	"tierceron/buildopts/coreopts"
	"tierceron/trcx/xutil"
	"tierceron/trcxbase"
)

// This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func main() {
	fmt.Println("Version: " + "1.5")
	env := "local"
	addr := coreopts.GetVaultHostPort()
	trcxbase.CommonMain(nil, xutil.GenerateSeedsFromVault, &env, &addr, nil)
}

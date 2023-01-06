package main

import (
	"fmt"

	"tierceron/buildopts/coreopts"
	"tierceron/trcvault/opts/memonly"
	"tierceron/trcx/xutil"
	"tierceron/trcxbase"
	"tierceron/utils/mlock"
)

// This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func main() {
	if memonly.IsMemonly() {
		mlock.Mlock(nil)
	}
	fmt.Println("Version: " + "1.5")
	env := "local"
	addr := coreopts.GetVaultHostPort()
	trcxbase.CommonMain(nil, xutil.GenerateSeedsFromVault, &env, &addr, nil, nil)
}

package main

import (
	"flag"
	"fmt"

	"tierceron/trcvault/opts/memonly"
	"tierceron/trcx/xutil"
	trcxbase "tierceron/trcxbase"
	"tierceron/utils/mlock"
)

// This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func main() {
	if memonly.IsMemonly() {
		mlock.Mlock(nil)
	}
	fmt.Println("Version: " + "1.25")
	envPtr := flag.String("env", "dev", "Environment to get seed data for.")

	trcxbase.CommonMain(nil, xutil.GenerateSeedsFromVault, envPtr, nil, nil, nil)
}

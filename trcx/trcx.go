package main

import (
	"flag"
	"fmt"

	"tierceron/trcx/xutil"
	trcxbase "tierceron/trcxbase"
)

// This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func main() {
	fmt.Println("Version: " + "1.21")
	envPtr := flag.String("env", "dev", "Environment to get seed data for.")

	trcxbase.CommonMain(nil, xutil.GenerateSeedsFromVault, envPtr, nil, nil)
}

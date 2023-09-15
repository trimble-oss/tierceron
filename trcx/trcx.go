package main

import (
	"flag"
	"fmt"

	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
	"github.com/trimble-oss/tierceron/trcx/xutil"
	trcxbase "github.com/trimble-oss/tierceron/trcxbase"
)

// This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func main() {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	fmt.Println("Version: " + "1.25")
	envPtr := flag.String("env", "dev", "Environment to get seed data for.")

	trcxbase.CommonMain(nil, xutil.GenerateSeedsFromVault, envPtr, nil, nil, nil)
}

package main

import (
	"fmt"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
	"github.com/trimble-oss/tierceron/trcx/xutil"
	"github.com/trimble-oss/tierceron/trcxbase"
)

// This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func main() {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	fmt.Println("Version: " + "1.5")
	env := "local"
	addr := coreopts.GetVaultHostPort()
	trcxbase.CommonMain(nil, xutil.GenerateSeedsFromVault, &env, &addr, nil, nil)
}

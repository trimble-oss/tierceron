package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/memonly"
	"github.com/trimble-oss/tierceron-core/v2/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig"
	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig/cache"
	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/deployopts"
	"github.com/trimble-oss/tierceron/buildopts/tcopts"
	"github.com/trimble-oss/tierceron/buildopts/xencryptopts"
	trcxbase "github.com/trimble-oss/tierceron/pkg/cli/trcxbase"
	"github.com/trimble-oss/tierceron/pkg/trcx/xutil"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

// This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func main() {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	buildopts.NewOptionsBuilder(buildopts.LoadOptions())
	coreopts.NewOptionsBuilder(coreopts.LoadOptions())
	deployopts.NewOptionsBuilder(deployopts.LoadOptions())
	tcopts.NewOptionsBuilder(tcopts.LoadOptions())
	xencryptopts.NewOptionsBuilder(xencryptopts.LoadOptions())
	fmt.Println("Version: " + "1.26")
	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Fprintf(flagset.Output(), "Usage of %s:\n", os.Args[0])
		flagset.PrintDefaults()
	}
	envPtr := flagset.String("env", "dev", "Environment to get seed data for.")

	driverConfig := config.DriverConfig{
		CoreConfig: &coreconfig.CoreConfig{
			ExitOnFailure: true,
			TokenCache:    cache.NewTokenCacheEmpty(),
		},
	}
	trcxbase.CommonMain(nil, xutil.GenerateSeedsFromVault, envPtr, nil, nil, nil, flagset, os.Args, &driverConfig)
}

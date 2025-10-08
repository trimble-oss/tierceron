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
	"github.com/trimble-oss/tierceron/buildopts/kernelopts"
	"github.com/trimble-oss/tierceron/buildopts/tcopts"
	"github.com/trimble-oss/tierceron/buildopts/xencryptopts"
	"github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

// This assumes that the vault is completely new, and should only be run for the purpose
// of automating setup and initial seeding

func main() {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	buildopts.NewOptionsBuilder(buildopts.LoadOptions())
	coreopts.NewOptionsBuilder(coreopts.LoadOptions())
	deployopts.NewOptionsBuilder(deployopts.LoadOptions())
	kernelopts.NewOptionsBuilder(kernelopts.LoadOptions())
	tcopts.NewOptionsBuilder(tcopts.LoadOptions())
	xencryptopts.NewOptionsBuilder(xencryptopts.LoadOptions())
	trcconfigbase.PrintVersion()
	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Fprintf(flagset.Output(), "Usage of %s:\n", os.Args[0])
		flagset.PrintDefaults()
	}
	envPtr := flagset.String("env", "dev", "Environment to configure")
	regionPtr := flagset.String("region", "", "Region to be processed") //If this is blank -> use context otherwise override context.
	tokenNamePtr := flagset.String("tokenName", "", "Token name used by this"+coreopts.BuildOptions.GetFolderPrefix(nil)+"config to access the vault")

	driverConfig := config.DriverConfig{
		CoreConfig: &coreconfig.CoreConfig{
			ExitOnFailure: true,
			TokenCache:    cache.NewTokenCacheEmpty(),
		},
	}

	err := trcconfigbase.CommonMain(envPtr, nil, tokenNamePtr, regionPtr, flagset, os.Args, &driverConfig)
	if err != nil {
		os.Exit(1)
	}
}

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig/cache"
	"github.com/trimble-oss/tierceron/pkg/utils/config"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/memonly"
	"github.com/trimble-oss/tierceron-core/v2/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig"
	"github.com/trimble-oss/tierceron/atrium/buildopts/flowcoreopts"
	plgtbase "github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/trcplgtoolbase"
	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/cursoropts"
	"github.com/trimble-oss/tierceron/buildopts/deployopts"
	"github.com/trimble-oss/tierceron/buildopts/harbingeropts"
	"github.com/trimble-oss/tierceron/buildopts/kernelopts"
	"github.com/trimble-oss/tierceron/buildopts/tcopts"
	"github.com/trimble-oss/tierceron/buildopts/xencryptopts"
	"github.com/trimble-oss/tierceron/pkg/capauth"
)

// This executable automates the cerification of a plugin docker image.
func main() {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	buildopts.NewOptionsBuilder(buildopts.LoadOptions())
	coreopts.NewOptionsBuilder(coreopts.LoadOptions())
	deployopts.NewOptionsBuilder(deployopts.LoadOptions())
	flowcoreopts.NewOptionsBuilder(flowcoreopts.LoadOptions())
	harbingeropts.NewOptionsBuilder(harbingeropts.LoadOptions())
	tcopts.NewOptionsBuilder(tcopts.LoadOptions())
	xencryptopts.NewOptionsBuilder(xencryptopts.LoadOptions())
	cursoropts.NewOptionsBuilder(cursoropts.LoadOptions())
	kernelopts.NewOptionsBuilder(kernelopts.LoadOptions())

	fmt.Println("Version: " + "1.05")

	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Fprintf(flagset.Output(), "Usage of %s:\n", os.Args[0])
		flagset.PrintDefaults()
	}
	envPtr := flagset.String("env", "dev", "Environment to configure")
	regionPtr := flagset.String("region", "", "Region to be processed") // If this is blank -> use context otherwise override context.
	tokenNamePtr := flagset.String("tokenName", "", "Token name used by this"+coreopts.BuildOptions.GetFolderPrefix(nil)+"config to access the vault")

	trcshDriveConfigPtr := &capauth.TrcshDriverConfig{
		DriverConfig: &config.DriverConfig{
			CoreConfig: &coreconfig.CoreConfig{
				ExitOnFailure:        true,
				Insecure:             false,
				CurrentRoleEntityPtr: new(string),
				TokenCache:           cache.NewTokenCacheEmpty(),
			},
			IsShellSubProcess: false,
			StartDir:          []string{""},
		},
	}

	err := plgtbase.CommonMain(envPtr, nil, tokenNamePtr, regionPtr, flagset, os.Args, trcshDriveConfigPtr)
	if err != nil {
		os.Exit(1)
	}
}

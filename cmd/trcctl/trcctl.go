package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/trimble-oss/tierceron/atrium/buildopts/flowcoreopts"
	flowcoreloadopts "github.com/trimble-oss/tierceron/atrium/buildopts/flowcoreopts"
	"github.com/trimble-oss/tierceron/atrium/buildopts/flowopts"
	flowloadopts "github.com/trimble-oss/tierceron/atrium/buildopts/flowopts"
	"github.com/trimble-oss/tierceron/atrium/buildopts/testopts"
	testloadopts "github.com/trimble-oss/tierceron/atrium/buildopts/testopts"
	buildloadopts "github.com/trimble-oss/tierceron/buildopts"
	coreloadopts "github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/cursoropts"
	cursorloadopts "github.com/trimble-oss/tierceron/buildopts/cursoropts"
	deployloadopts "github.com/trimble-oss/tierceron/buildopts/deployopts"
	"github.com/trimble-oss/tierceron/buildopts/harbingeropts"
	harbingerloadopts "github.com/trimble-oss/tierceron/buildopts/harbingeropts"
	"github.com/trimble-oss/tierceron/buildopts/kernelopts"
	kernelloadopts "github.com/trimble-oss/tierceron/buildopts/kernelopts"
	"github.com/trimble-oss/tierceron/buildopts/pluginopts"
	pluginloadopts "github.com/trimble-oss/tierceron/buildopts/pluginopts"
	tcloadopts "github.com/trimble-oss/tierceron/buildopts/tcopts"
	xencryptloadopts "github.com/trimble-oss/tierceron/buildopts/xencryptopts"

	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/deployopts"
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/buildopts/tcopts"
	"github.com/trimble-oss/tierceron/buildopts/xencryptopts"
	"github.com/trimble-oss/tierceron/pkg/cli/trcctlbase"
	"github.com/trimble-oss/tierceron/pkg/core"
	"github.com/trimble-oss/tierceron/pkg/core/cache"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

// This is a controller program that can act as any command line utility.
// The swiss army knife of tierceron if you will.
func main() {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	testopts.NewOptionsBuilder(testloadopts.LoadOptions())
	flowopts.NewOptionsBuilder(flowloadopts.LoadOptions())
	flowcoreopts.NewOptionsBuilder(flowcoreloadopts.LoadOptions())
	harbingeropts.NewOptionsBuilder(harbingerloadopts.LoadOptions())
	kernelopts.NewOptionsBuilder(kernelloadopts.LoadOptions())
	cursoropts.NewOptionsBuilder(cursorloadopts.LoadOptions())
	pluginopts.NewOptionsBuilder(pluginloadopts.LoadOptions())
	buildopts.NewOptionsBuilder(buildloadopts.LoadOptions())
	coreopts.NewOptionsBuilder(coreloadopts.LoadOptions())
	deployopts.NewOptionsBuilder(deployloadopts.LoadOptions())
	tcopts.NewOptionsBuilder(tcloadopts.LoadOptions())
	xencryptopts.NewOptionsBuilder(xencryptloadopts.LoadOptions())
	fmt.Println("Version: " + "1.36")
	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Fprintf(flagset.Output(), "Usage of %s:\n", os.Args[0])
		flagset.PrintDefaults()
	}
	envPtr := flagset.String("env", "", "Environment to be seeded") //If this is blank -> use context otherwise override context.
	pluginNamePtr := flagset.String("pluginName", "", "Specifies which templates to filter")
	tokenPtr := flagset.String("token", "", "Vault access token")
	uploadCertPtr := flagset.Bool("certs", false, "Upload certs if provided")
	prodPtr := flagset.Bool("prod", false, "Prod only seeds vault with staging environment")
	flagset.Bool("pluginInfo", false, "Lists all plugins")
	flagset.Bool("novault", false, "Don't pull configuration data from vault.")

	driverConfig := config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			ExitOnFailure: true,
			TokenCache:    cache.NewTokenCacheEmpty(),
		},
	}

	err := trcctlbase.CommonMain(envPtr,
		pluginNamePtr,
		tokenPtr,
		uploadCertPtr,
		prodPtr,
		flagset,
		os.Args,
		&driverConfig)
	if err != nil {
		os.Exit(1)
	}
}

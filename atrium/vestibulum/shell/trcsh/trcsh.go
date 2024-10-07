package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcshbase"
	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/cursoropts"
	"github.com/trimble-oss/tierceron/buildopts/deployopts"
	"github.com/trimble-oss/tierceron/buildopts/harbingeropts"
	"github.com/trimble-oss/tierceron/buildopts/kernelopts"
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/buildopts/saltyopts"
	"github.com/trimble-oss/tierceron/buildopts/tcopts"
	"github.com/trimble-oss/tierceron/buildopts/xencryptopts"
	"github.com/trimble-oss/tierceron/pkg/core"
	tiercerontls "github.com/trimble-oss/tierceron/pkg/tls"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

// This is a controller program that can act as any command line utility.
// The Tierceron Shell runs tierceron and kubectl commands in a secure shell.
func main() {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	buildopts.NewOptionsBuilder(buildopts.LoadOptions())
	coreopts.NewOptionsBuilder(coreopts.LoadOptions())
	deployopts.NewOptionsBuilder(deployopts.LoadOptions())
	harbingeropts.NewOptionsBuilder(harbingeropts.LoadOptions())
	tcopts.NewOptionsBuilder(tcopts.LoadOptions())
	xencryptopts.NewOptionsBuilder(xencryptopts.LoadOptions())
	saltyopts.NewOptionsBuilder(saltyopts.LoadOptions())
	kernelopts.NewOptionsBuilder(kernelopts.LoadOptions())
	cursoropts.NewOptionsBuilder(cursoropts.LoadOptions())
	eUtils.InitHeadless(true)

	tiercerontls.InitRoot()

	fmt.Println("trcsh Version: " + "1.25")
	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Fprintf(flagset.Output(), "Usage of %s:\n", os.Args[0])
		flagset.PrintDefaults()
	}
	envPtr := flagset.String("env", "", "Environment to be processed") //If this is blank -> use context otherwise override context.
	addrPtr := flagset.String("addr", "", "API endpoint for the vault")
	secretIDPtr := flagset.String("secretID", "", "Secret for app role ID")
	appRoleIDPtr := flagset.String("appRoleID", "", "Public app role ID")

	logger, logErr := trcshbase.CreateLogFile()
	if logErr != nil {
		os.Exit(1)
	}

	driverConfig := config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			Log: logger,
		},
	}

	err := trcshbase.CommonMain(envPtr, addrPtr, nil, secretIDPtr, appRoleIDPtr, flagset, os.Args, &driverConfig)
	if err != nil {
		os.Exit(1)
	}
}

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/deployopts"
	"github.com/trimble-oss/tierceron/buildopts/harbingeropts"
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/buildopts/tcopts"
	"github.com/trimble-oss/tierceron/buildopts/xencryptopts"
	trcinitbase "github.com/trimble-oss/tierceron/pkg/cli/trcinitbase"
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
	harbingeropts.NewOptionsBuilder(harbingeropts.LoadOptions())
	tcopts.NewOptionsBuilder(tcopts.LoadOptions())
	xencryptopts.NewOptionsBuilder(xencryptopts.LoadOptions())
	fmt.Println("Version: " + "1.35")
	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Fprintf(flagset.Output(), "Usage of %s:\n", os.Args[0])
		flagset.PrintDefaults()
	}

	envPtr := flagset.String("env", "dev", "Environment to be seeded")
	trcinitbase.CommonMain(envPtr, nil, nil, flagset, os.Args)
}

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	trcinitbase "github.com/trimble-oss/tierceron/trcinitbase"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
)

// This assumes that the vault is completely new, and should only be run for the purpose
// of automating setup and initial seeding

func main() {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	fmt.Println("Version: " + "1.34")
	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Fprintf(flagset.Output(), "Usage of %s:\n", os.Args[0])
		flagset.PrintDefaults()
	}

	envPtr := flagset.String("env", "dev", "Environment to be seeded")
	trcinitbase.CommonMain(envPtr, nil, nil, flagset, os.Args)
}

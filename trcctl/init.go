package main

import (
	"flag"
	"fmt"
	"os"

	"tierceron/trcconfigbase"
	trcinitbase "tierceron/trcinitbase"
	"tierceron/trcvault/opts/memonly"
	"tierceron/trcx/xutil"
	"tierceron/trcxbase"
	"tierceron/utils/mlock"
)

// This is a controller program that can act as any command line utility.
// The swiss army knife of tierceron if you will.
func main() {
	if memonly.IsMemonly() {
		mlock.Mlock(nil)
	}
	fmt.Println("Version: " + "1.34")
	envPtr := flag.String("env", "dev", "Environment to be seeded")
	envCtxPtr := flag.String("context", "dev", "Context to define.")

	if ctl := os.Args[1]; ctl != "" {
		switch ctl {
		case "pub":
			// TODO
		case "sub":
			// TODO
		case "init":
			trcinitbase.CommonMain(envPtr, nil, envCtxPtr)
		case "config":
			trcconfigbase.CommonMain(envPtr, nil, envCtxPtr)
		case "x":
			trcxbase.CommonMain(nil, xutil.GenerateSeedsFromVault, envPtr, nil, nil, nil)
		}
	}

}

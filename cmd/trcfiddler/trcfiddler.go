package main

import (
	"flag"
	"fmt"
	"os"

	"kernel.org/pub/linux/libs/security/libcap/cap"
)

// This assumes that the vault is completely new, and should only be run for the purpose
// of automating setup and initial seeding

func main() {
	fmt.Fprintln(os.Stderr, "Version: "+"1.30")
	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Fprintf(flagset.Output(), "Usage of %s:\n", os.Args[0])
		flagset.PrintDefaults()
	}
	//_ := flagset.String("env", "dev", "Environment to configure")

	ipcLockCapSet, err := cap.FromText("cap_ipc_lock=+ep")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
	}
	setErr := ipcLockCapSet.SetFile("/home/joel/workspace/github/mrjrieke/tierceron/bin/trcconfig")

	if setErr != nil {
		fmt.Fprintf(os.Stderr, "%v", setErr)
		os.Exit(1)
	}
}

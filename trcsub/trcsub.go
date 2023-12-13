package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/trcsubbase"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
)

// Reads in template files in specified directory
// Template directory should contain directories for each service
// Templates are uploaded to templates/<service>/<file name>/template-file
// The file is saved under the data key, and the extension under the ext key
// Vault automatically encodes the file into base64

func main() {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	fmt.Println("Version: " + "1.26")
	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Fprintf(flagset.Output(), "Usage of %s:\n", os.Args[0])
		flagset.PrintDefaults()
	}
	envPtr := flagset.String("env", "dev", "Environment to configure")
	addrPtr := flagset.String("addr", "", "API endpoint for the vault")
	secretIDPtr := flagset.String("secretID", "", "Public app role ID")
	appRoleIDPtr := flagset.String("appRoleID", "", "Secret app role ID")

	err := trcsubbase.CommonMain(envPtr, addrPtr, nil, secretIDPtr, appRoleIDPtr, flagset, os.Args, nil)
	if err != nil {
		os.Exit(1)
	}
}

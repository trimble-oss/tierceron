package main

import (
	"flag"
	"fmt"

	"github.com/trimble-oss/tierceron/trcsubbase"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
	"github.com/trimble-oss/tierceron/utils/mlock"
)

// Reads in template files in specified directory
// Template directory should contain directories for each service
// Templates are uploaded to templates/<service>/<file name>/template-file
// The file is saved under the data key, and the extension under the ext key
// Vault automatically encodes the file into base64

func main() {
	if memonly.IsMemonly() {
		mlock.Mlock(nil)
	}
	fmt.Println("Version: " + "1.26")
	envPtr := flag.String("env", "dev", "Environment to configure")
	addrPtr := flag.String("addr", "", "API endpoint for the vault")

	trcsubbase.CommonMain(envPtr, addrPtr, nil)
}

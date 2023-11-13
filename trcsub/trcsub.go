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
	envPtr := flag.String("env", "dev", "Environment to configure")
	addrPtr := flag.String("addr", "", "API endpoint for the vault")
	secretIDPtr := flag.String("secretID", "", "Public app role ID")
	appRoleIDPtr := flag.String("appRoleID", "", "Secret app role ID")

	err := trcsubbase.CommonMain(envPtr, addrPtr, nil, secretIDPtr, appRoleIDPtr, nil)
	if err != nil {
		os.Exit(1)
	}
}

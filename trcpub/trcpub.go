package main

import (
	"flag"
	"fmt"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/trcpubbase"
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
	tokenPtr := flag.String("token", "", "Vault access token")
	secretIDPtr := flag.String("secretID", "", "Public app role ID")
	appRoleIDPtr := flag.String("appRoleID", "", "Secret app role ID")
	tokenNamePtr := flag.String("tokenName", "", "Token name used by this "+coreopts.GetFolderPrefix(nil)+"pub to access the vault")

	trcpubbase.CommonMain(envPtr, addrPtr, tokenPtr, nil, secretIDPtr, appRoleIDPtr, tokenNamePtr, nil)
}

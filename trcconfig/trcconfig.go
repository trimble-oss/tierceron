package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/trcconfigbase"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
)

// This assumes that the vault is completely new, and should only be run for the purpose
// of automating setup and initial seeding

func main() {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	fmt.Println("Version: " + "1.27")
	envPtr := flag.String("env", "dev", "Environment to configure")
	addrPtr := flag.String("addr", "", "API endpoint for the vault")
	tokenPtr := flag.String("token", "", "Vault access token")
	secretIDPtr := flag.String("secretID", "", "Secret app role ID")
	regionPtr := flag.String("region", "", "Region to be processed") //If this is blank -> use context otherwise override context.
	appRoleIDPtr := flag.String("appRoleID", "", "Public app role ID")
	tokenNamePtr := flag.String("tokenName", "", "Token name used by this"+coreopts.GetFolderPrefix(nil)+"config to access the vault")

	err := trcconfigbase.CommonMain(envPtr, addrPtr, tokenPtr, nil, secretIDPtr, appRoleIDPtr, tokenNamePtr, regionPtr, nil)
	if err != nil {
		os.Exit(1)
	}
}

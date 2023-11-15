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
	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.Usage = flag.Usage
	envPtr := flagset.String("env", "dev", "Environment to configure")
	addrPtr := flagset.String("addr", "", "API endpoint for the vault")
	tokenPtr := flagset.String("token", "", "Vault access token")
	secretIDPtr := flagset.String("secretID", "", "Secret app role ID")
	regionPtr := flagset.String("region", "", "Region to be processed") //If this is blank -> use context otherwise override context.
	appRoleIDPtr := flagset.String("appRoleID", "", "Public app role ID")
	tokenNamePtr := flagset.String("tokenName", "", "Token name used by this"+coreopts.GetFolderPrefix(nil)+"config to access the vault")

	err := trcconfigbase.CommonMain(envPtr, addrPtr, tokenPtr, nil, secretIDPtr, appRoleIDPtr, tokenNamePtr, regionPtr, flagset, os.Args, nil)
	if err != nil {
		os.Exit(1)
	}
}

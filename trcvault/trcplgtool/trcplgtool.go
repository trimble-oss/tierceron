package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	plgtbase "github.com/trimble-oss/tierceron/trcvault/trcplgtoolbase"
)

// This executable automates the cerification of a plugin docker image.
func main() {
	fmt.Println("Version: " + "1.02")

	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Fprintf(flagset.Output(), "Usage of %s:\n", os.Args[0])
		flagset.PrintDefaults()
	}
	envPtr := flagset.String("env", "dev", "Environment to configure")
	addrPtr := flagset.String("addr", "", "API endpoint for the vault")
	tokenPtr := flagset.String("token", "", "Vault access token")
	regionPtr := flagset.String("region", "", "Region to be processed") //If this is blank -> use context otherwise override context.
	secretIDPtr := flagset.String("secretID", "", "Secret app role ID")
	appRoleIDPtr := flagset.String("appRoleID", "", "Public app role ID")
	tokenNamePtr := flagset.String("tokenName", "", "Token name used by this"+coreopts.GetFolderPrefix(nil)+"config to access the vault")

	err := plgtbase.CommonMain(envPtr, addrPtr, tokenPtr, nil, secretIDPtr, appRoleIDPtr, tokenNamePtr, regionPtr, flagset, os.Args, nil)
	if err != nil {
		os.Exit(1)
	}

}

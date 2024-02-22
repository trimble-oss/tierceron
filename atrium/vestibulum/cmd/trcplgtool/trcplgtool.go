package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/trimble-oss/tierceron/atrium/buildopts/flowcoreopts"
	plgtbase "github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/trcplgtoolbase"
	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/deployopts"
	"github.com/trimble-oss/tierceron/buildopts/harbingeropts"
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/buildopts/tcopts"
	"github.com/trimble-oss/tierceron/buildopts/xencryptopts"
)

// This executable automates the cerification of a plugin docker image.
func main() {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	buildopts.NewOptionsBuilder(buildopts.LoadOptions())
	coreopts.NewOptionsBuilder(coreopts.LoadOptions())
	deployopts.NewOptionsBuilder(deployopts.LoadOptions())
	flowcoreopts.NewOptionsBuilder(flowcoreopts.LoadOptions())
	harbingeropts.NewOptionsBuilder(harbingeropts.LoadOptions())
	tcopts.NewOptionsBuilder(tcopts.LoadOptions())
	xencryptopts.NewOptionsBuilder(xencryptopts.LoadOptions())

	fmt.Println("Version: " + "1.05")

	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Fprintf(flagset.Output(), "Usage of %s:\n", os.Args[0])
		flagset.PrintDefaults()
	}
	envPtr := flagset.String("env", "dev", "Environment to configure")
	addrPtr := flagset.String("addr", "", "API endpoint for the vault")
	tokenPtr := flagset.String("token", "", "Vault access token")
	regionPtr := flagset.String("region", "", "Region to be processed") //If this is blank -> use context otherwise override context.
	secretIDPtr := flagset.String("secretID", "", "Secret for app role ID")
	appRoleIDPtr := flagset.String("appRoleID", "", "Public app role ID")
	tokenNamePtr := flagset.String("tokenName", "", "Token name used by this"+coreopts.BuildOptions.GetFolderPrefix(nil)+"config to access the vault")

	err := plgtbase.CommonMain(envPtr, addrPtr, tokenPtr, nil, secretIDPtr, appRoleIDPtr, tokenNamePtr, regionPtr, flagset, os.Args, nil)
	if err != nil {
		os.Exit(1)
	}

}

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	buildloadopts "github.com/trimble-oss/tierceron/buildopts"
	coreloadopts "github.com/trimble-oss/tierceron/buildopts/coreopts"
	deployloadopts "github.com/trimble-oss/tierceron/buildopts/deployopts"
	tcloadopts "github.com/trimble-oss/tierceron/buildopts/tcopts"
	xencryptloadopts "github.com/trimble-oss/tierceron/buildopts/xencryptopts"

	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/deployopts"
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/buildopts/tcopts"
	"github.com/trimble-oss/tierceron/buildopts/xencryptopts"
	"github.com/trimble-oss/tierceron/pkg/cli/trcctlbase"
	"github.com/trimble-oss/tierceron/pkg/core"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

// This is a controller program that can act as any command line utility.
// The swiss army knife of tierceron if you will.
func main() {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	buildopts.NewOptionsBuilder(buildloadopts.LoadOptions())
	coreopts.NewOptionsBuilder(coreloadopts.LoadOptions())
	deployopts.NewOptionsBuilder(deployloadopts.LoadOptions())
	tcopts.NewOptionsBuilder(tcloadopts.LoadOptions())
	xencryptopts.NewOptionsBuilder(xencryptloadopts.LoadOptions())
	fmt.Println("Version: " + "1.36")
	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Fprintf(flagset.Output(), "Usage of %s:\n", os.Args[0])
		flagset.PrintDefaults()
	}
	envPtr := flagset.String("env", "", "Environment to be seeded") //If this is blank -> use context otherwise override context.
	pluginNamePtr := flagset.String("pluginName", "", "Specifies which templates to filter")
	tokenPtr := flagset.String("token", "", "Vault access token")
	uploadCertPtr := flagset.Bool("certs", false, "Upload certs if provided")
	prodPtr := flagset.Bool("prod", false, "Prod only seeds vault with staging environment")
	flagset.Bool("pluginInfo", false, "Lists all plugins")
	flagset.Bool("novault", false, "Don't pull configuration data from vault.")
	addrPtr := flagset.String("addr", "", "API endpoint for the vault")
	var envContext string

	var ctl string
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") { //This pre checks arguments for ctl switch to allow parse to work with non "-" flags.
		ctl = os.Args[1]
		ctlSplit := strings.Split(ctl, " ")
		if len(ctlSplit) >= 2 {
			fmt.Println("Invalid arguments - only 1 non flag argument available at a time.")
			return
		}

		if len(os.Args) > 2 {
			os.Args = os.Args[1:]
		}
	}
	flagset.Parse(os.Args[1:])
	if flagset.NFlag() == 0 {
		flagset.Usage()
		os.Exit(0)
	}

	if ctl != "" {
		var err error
		if strings.Contains(ctl, "context") {
			contextSplit := strings.Split(ctl, "=")
			if len(contextSplit) == 1 {
				*envPtr, envContext, err = eUtils.GetSetEnvContext(*envPtr, envContext)
				if err != nil {
					fmt.Println(err.Error())
					return
				}
				fmt.Println("Current context is set to " + envContext)
			} else if len(contextSplit) == 2 {
				envContext = contextSplit[1]
				*envPtr, envContext, err = eUtils.GetSetEnvContext(*envPtr, envContext)
				if err != nil {
					fmt.Println(err.Error())
					return
				}
			}
		}
	}

	driverConfig := config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			ExitOnFailure: true,
		},
	}

	err := trcctlbase.CommonMain(envPtr,
		addrPtr,
		nil,
		tokenPtr,
		pluginNamePtr,
		uploadCertPtr,
		prodPtr,
		flagset,
		os.Args,
		&driverConfig)
	if err != nil {
		os.Exit(1)
	}
}

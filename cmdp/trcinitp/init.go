package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/deployopts"
	"github.com/trimble-oss/tierceron/buildopts/harbingeropts"
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/buildopts/tcopts"
	"github.com/trimble-oss/tierceron/buildopts/xencryptopts"
	trcinitbase "github.com/trimble-oss/tierceron/pkg/cli/trcinitbase"
	"github.com/trimble-oss/tierceron/pkg/core"
	"github.com/trimble-oss/tierceron/pkg/core/cache"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

// This assumes that the vault is completely new, and should only be run for the purpose
// of automating setup and initial seeding

func main() {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	buildopts.NewOptionsBuilder(buildopts.LoadOptions())
	coreopts.NewOptionsBuilder(coreopts.LoadOptions())
	deployopts.NewOptionsBuilder(deployopts.LoadOptions())
	harbingeropts.NewOptionsBuilder(harbingeropts.LoadOptions())
	tcopts.NewOptionsBuilder(tcopts.LoadOptions())
	xencryptopts.NewOptionsBuilder(xencryptopts.LoadOptions())

	trcinitbase.PrintVersion()
	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Fprintf(flagset.Output(), "Usage of %s:\n", os.Args[0])
		flagset.PrintDefaults()
	}
	env := "local"
	addr := coreopts.BuildOptions.GetVaultHostPort()
	tokenPtr := flagset.String("token", "", "Vault access token, only use if in dev mode or reseeding")
	uploadCertPtr := flagset.Bool("certs", false, "Upload certs if provided")
	driverConfig := config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			TokenCache:    cache.NewTokenCache(fmt.Sprintf("config_token_%s_unrestricted", env), tokenPtr, &addr),
			ExitOnFailure: true,
		},
	}

	trcinitbase.CommonMain(&env, nil, nil, uploadCertPtr, flagset, os.Args, &driverConfig)
}

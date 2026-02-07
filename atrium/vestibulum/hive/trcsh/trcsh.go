package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/memonly"
	"github.com/trimble-oss/tierceron-core/v2/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig"
	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig/cache"
	"github.com/trimble-oss/tierceron/atrium/buildopts/flowcoreopts"
	"github.com/trimble-oss/tierceron/atrium/buildopts/flowopts"
	"github.com/trimble-oss/tierceron/atrium/buildopts/testopts"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcshbase"
	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/cursoropts"
	"github.com/trimble-oss/tierceron/buildopts/deployopts"
	"github.com/trimble-oss/tierceron/buildopts/harbingeropts"
	"github.com/trimble-oss/tierceron/buildopts/kernelopts"
	"github.com/trimble-oss/tierceron/buildopts/pluginopts"
	"github.com/trimble-oss/tierceron/buildopts/saltyopts"
	"github.com/trimble-oss/tierceron/buildopts/tcopts"
	"github.com/trimble-oss/tierceron/buildopts/xencryptopts"
	tiercerontls "github.com/trimble-oss/tierceron/pkg/tls"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

var logger *log.Logger

func init() {
	// Initialize kernelopts early so IsKernelZ() is available in plugin init() functions
	kernelopts.NewOptionsBuilder(kernelopts.LoadOptions())

	// Create log file early so stderr redirection happens before plugin init() functions
	var logErr error
	logger, logErr = trcshbase.CreateLogFile()
	if logErr != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log file: %v\n", logErr)
		os.Exit(1)
	}
}

// This is a controller program that can act as any command line utility.
// The Tierceron Shell runs tierceron and kubectl commands in a secure shell.
func main() {
	if os.Geteuid() == 0 {
		eUtils.LogSyncAndExit(logger, "ERROR: trcsh must not run as root or with sudo privileges (FUNDAMENTALS Law #3)", 1)
	}

	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	flowcoreopts.NewOptionsBuilder(flowcoreopts.LoadOptions())
	testopts.NewOptionsBuilder(testopts.LoadOptions())
	flowopts.NewOptionsBuilder(flowopts.LoadOptions())
	pluginopts.NewOptionsBuilder(pluginopts.LoadOptions())
	buildopts.NewOptionsBuilder(buildopts.LoadOptions())
	coreopts.NewOptionsBuilder(coreopts.LoadOptions())
	deployopts.NewOptionsBuilder(deployopts.LoadOptions())
	harbingeropts.NewOptionsBuilder(harbingeropts.LoadOptions())
	tcopts.NewOptionsBuilder(tcopts.LoadOptions())
	xencryptopts.NewOptionsBuilder(xencryptopts.LoadOptions())
	saltyopts.NewOptionsBuilder(saltyopts.LoadOptions())
	cursoropts.NewOptionsBuilder(cursoropts.LoadOptions())

	// Safety check: Prevent non-Kubernetes variants from running in Kubernetes
	if !coreopts.BuildOptions.IsKubeRunnable() {
		if _, aksExists := os.LookupEnv("KUBERNETES_SERVICE_HOST"); aksExists {
			fmt.Fprintln(os.Stderr, "ERROR: This trcsh variant is not permitted to run in AKS/Kubernetes environments")
			os.Exit(1)
		}
		if _, err := os.Stat("/var/run/secrets/kubernetes.io"); err == nil {
			fmt.Fprintln(os.Stderr, "ERROR: This trcsh variant is not permitted to run in AKS/Kubernetes environments")
			os.Exit(1)
		}
	}

	eUtils.InitHeadless(true)

	tiercerontls.InitRoot()

	fmt.Fprintln(os.Stderr, "trcsh Version: "+"1.26")
	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Fprintf(flagset.Output(), "Usage of %s:\n", os.Args[0])
		flagset.PrintDefaults()
	}
	envPtr := flagset.String("env", "", "Environment to be processed") // If this is blank -> use context otherwise override context.

	driverConfig := config.DriverConfig{
		CoreConfig: &coreconfig.CoreConfig{
			ExitOnFailure: true,
			TokenCache:    cache.NewTokenCacheEmpty(),
			Log:           logger,
		},
	}

	err := trcshbase.CommonMain(envPtr, nil, flagset, os.Args, nil, &driverConfig)
	if err != nil {
		os.Exit(1)
	}
}

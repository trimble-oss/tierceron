package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig"

	"github.com/trimble-oss/tierceron/atrium/buildopts/flowopts"
	"github.com/trimble-oss/tierceron/atrium/buildopts/testopts"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trccarrier/carrierfactory"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcflow/deploy"
	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/cursoropts"
	"github.com/trimble-oss/tierceron/buildopts/deployopts"
	"github.com/trimble-oss/tierceron/buildopts/harbingeropts"
	"github.com/trimble-oss/tierceron/buildopts/saltyopts"
	"github.com/trimble-oss/tierceron/buildopts/tcopts"
	"github.com/trimble-oss/tierceron/buildopts/xencryptopts"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

var signalChannel chan os.Signal

// This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func main() {
	// Set up global signal capture.
	signalChannel = make(chan os.Signal, 1)
	signal.Notify(signalChannel,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	buildopts.NewOptionsBuilder(buildopts.LoadOptions())
	coreopts.NewOptionsBuilder(coreopts.LoadOptions())
	deployopts.NewOptionsBuilder(deployopts.LoadOptions())
	flowopts.NewOptionsBuilder(flowopts.LoadOptions())
	harbingeropts.NewOptionsBuilder(harbingeropts.LoadOptions())
	tcopts.NewOptionsBuilder(tcopts.LoadOptions())
	testopts.NewOptionsBuilder(testopts.LoadOptions())
	xencryptopts.NewOptionsBuilder(xencryptopts.LoadOptions())
	saltyopts.NewOptionsBuilder(saltyopts.LoadOptions())
	cursoropts.NewOptionsBuilder(cursoropts.LoadOptions())

	// Read from environment variables with flag overrides
	defaultAddr := os.Getenv("VAULT_ADDR")
	if defaultAddr == "" {
		defaultAddr = coreopts.BuildOptions.GetVaultHostPort()
	}
	defaultToken := os.Getenv("VAULT_TOKEN")
	defaultConfigRole := os.Getenv("CONFIG_ROLE")
	defaultPubRole := os.Getenv("PUB_ROLE")
	defaultEnv := os.Getenv("DEPLOYMENT_ENV")
	if defaultEnv == "" {
		defaultEnv = "dev"
	}
	defaultLog := os.Getenv("LOG_FILE")
	if defaultLog == "" {
		defaultLog = "./trcshcuratortest.log"
	}

	logFilePtr := flag.String("log", defaultLog, "Output path for log file")
	addrPtr := flag.String("addr", defaultAddr, "API endpoint for the vault")
	tokenPtr := flag.String("token", defaultToken, "Vault access token")
	configRole := flag.String("configrole", defaultConfigRole, "Vault config access")
	pubRole := flag.String("pubrole", defaultPubRole, "Vault pub access")
	envPtr := flag.String("env", defaultEnv, "Environment to configure")
	flag.Parse()

	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	logger := log.New(f, "[trcdbplugin]", log.LstdFlags)

	driverConfig := &config.DriverConfig{
		CoreConfig: &coreconfig.CoreConfig{
			ExitOnFailure: true,
			Log:           logger,
		},
	}
	eUtils.CheckError(driverConfig.CoreConfig, err, true)

	// Grabbing configs
	envMap := buildopts.BuildOptions.GetTestDeployConfig(tokenPtr)
	envMap["vaddress"] = *addrPtr
	envMap["caddress"] = *addrPtr
	envMap["ctoken"] = *tokenPtr
	envMap["token"] = *tokenPtr
	envMap["tokenptr"] = tokenPtr
	envMap["ctokenptr"] = tokenPtr
	carrierfactory.InitLogger(logger)

	envMap["configrole"] = *configRole
	envMap["pubrole"] = *pubRole

	// deploy.PluginDeployFlow(pluginConfig, logger)
	go carrierfactory.Init(coreopts.BuildOptions.InitPluginConfig, deploy.PluginDeployEnvFlow, deploy.PluginDeployFlow, true, logger)
	envMap["env"] = *envPtr
	envMap["insecure"] = true
	envMap["syncOnce"] = &sync.Once{}
	carrierfactory.PushEnv(envMap)
	carrierfactory.TestInit()

	for {
		select {
		case <-signalChannel:
			eUtils.LogErrorMessage(driverConfig.CoreConfig, "Receiving shutdown presumably from vault.", true)
			os.Exit(0)
		}
	}
}

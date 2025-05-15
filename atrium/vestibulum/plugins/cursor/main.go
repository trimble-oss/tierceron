package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	prod "github.com/trimble-oss/tierceron-core/v2/prod"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/plugins/cursor/cursorlib"

	"github.com/trimble-oss/tierceron/atrium/buildopts/flowcoreopts"
	"github.com/trimble-oss/tierceron/atrium/buildopts/flowopts"
	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/cursoropts"
	"github.com/trimble-oss/tierceron/buildopts/deployopts"
	"github.com/trimble-oss/tierceron/buildopts/harbingeropts"
	memonly "github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/saltyopts"
	"github.com/trimble-oss/tierceron/buildopts/tcopts"
	"github.com/trimble-oss/tierceron/buildopts/xencryptopts"
	"github.com/trimble-oss/tierceron/pkg/core"
	tiercerontls "github.com/trimble-oss/tierceron/pkg/tls"

	eUtils "github.com/trimble-oss/tierceron/pkg/utils"

	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/sdk/plugin"
	"golang.org/x/sys/unix"
)

var logger *log.Logger

func main() {
	executableName := os.Args[0]
	if memonly.IsMemonly() {
		mLockErr := unix.Mlockall(unix.MCL_CURRENT | unix.MCL_FUTURE)
		if mLockErr != nil {
			fmt.Println(mLockErr)
			os.Exit(-1)
		}
	}
	buildopts.NewOptionsBuilder(buildopts.LoadOptions())
	coreopts.NewOptionsBuilder(coreopts.LoadOptions())
	deployopts.NewOptionsBuilder(deployopts.LoadOptions())
	flowcoreopts.NewOptionsBuilder(flowcoreopts.LoadOptions())
	flowopts.NewOptionsBuilder(flowopts.LoadOptions())
	harbingeropts.NewOptionsBuilder(harbingeropts.LoadOptions())
	tcopts.NewOptionsBuilder(tcopts.LoadOptions())
	xencryptopts.NewOptionsBuilder(xencryptopts.LoadOptions())
	saltyopts.NewOptionsBuilder(saltyopts.LoadOptions())
	cursoropts.NewOptionsBuilder(cursoropts.LoadOptions())
	tiercerontls.InitRoot()

	eUtils.InitHeadless(true)
	logFile := cursoropts.BuildOptions.GetLogPath()
	f, logErr := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if logErr != nil {
		logFile = "./trccursor.log"
		f, logErr = os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	}
	logger = log.New(f, fmt.Sprintf("[%s]", cursoropts.BuildOptions.GetPluginName(true)), log.LstdFlags)
	eUtils.CheckError(&core.CoreConfig{
		ExitOnFailure: true,
		Log:           logger,
	}, logErr, true)

	cursorlib.InitLogger(logger)
	logger.Println("Version: 1.1")
	logger.Println("Beginning plugin startup.")
	if strings.HasSuffix(executableName, "-prod") {
		logger.Println("Running prod plugin")
		prod.SetProd(true)
	}

	buildopts.BuildOptions.SetLogger(logger.Writer())

	if os.Getenv(api.PluginMetadataModeEnv) == "true" {
		logger.Println("Metadata init.")
	}

	apiClientMeta := api.PluginAPIClientMeta{}
	flags := apiClientMeta.FlagSet()

	logger.Println("Running plugin with cert validation...")

	args := os.Args
	args = append(args, fmt.Sprintf("--client-cert=%s", "../certs/serv_cert.pem"))
	args = append(args, fmt.Sprintf("--client-key=%s", "../certs/serv_key.pem"))

	argErr := flags.Parse(args[1:])
	if argErr != nil {
		logger.Fatal(argErr)
	}
	logger.Print("Warming up...")

	tlsConfig := apiClientMeta.GetTLSConfig()
	tlsProviderFunc := api.VaultPluginTLSProvider(tlsConfig)

	pluginName := cursoropts.BuildOptions.GetPluginName(true)

	logger.Print("Starting server...")
	err := plugin.Serve(cursorlib.GetCursorPluginOpts(pluginName, tlsProviderFunc))
	if err != nil {
		logger.Fatal("Plugin shutting down")
	}
}

package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"strings"

	prod "github.com/trimble-oss/tierceron-core/v2/prod"
	"github.com/trimble-oss/tierceron/atrium/buildopts/flowcoreopts"
	"github.com/trimble-oss/tierceron/atrium/buildopts/flowopts"
	"github.com/trimble-oss/tierceron/atrium/buildopts/localopts"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trccarrier/carrierfactory"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcflow/deploy"
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

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/sdk/plugin"
	"golang.org/x/sys/unix"
)

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
		logFile = "./trcplugincurator.log"
		f, logErr = os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	}
	logger := log.New(f, fmt.Sprintf("[%s]", cursoropts.BuildOptions.GetPluginName(true)), log.LstdFlags)
	eUtils.CheckError(&core.CoreConfig{
		ExitOnFailure: true,
		Log:           logger,
	}, logErr, true)
	logger.Println("Beginning plugin startup.")
	if strings.HasSuffix(executableName, "-prod") {
		logger.Println("Running prod plugin")
		prod.SetProd(true)
	} else {
		logger.Println("Running non-prod plugin")
	}
	buildopts.BuildOptions.SetLogger(logger.Writer())
	carrierfactory.InitLogger(logger)

	e := os.Remove(fmt.Sprintf("%s/trcsnap.sock", cursoropts.BuildOptions.GetCapPath()))
	if e != nil {
		logger.Println("Unable to refresh socket.  Uneccessary.")
	}

	carrierfactory.Init(coreopts.BuildOptions.InitPluginConfig, deploy.PluginDeployEnvFlow, deploy.PluginDeployFlow, true, logger)

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

	var tlsProviderOverrideFunc func() (*tls.Config, error)
	if localopts.IsLocal() {
		tlsProviderOverrideFunc = func() (*tls.Config, error) {
			if tlsProviderFunc == nil {
				return nil, nil
			}
			logger.Print("Tls providing...")
			tlsConfigProvidedConfig, err := tlsProviderFunc()
			if err != nil {
				return nil, err
			}
			logger.Print("Tls provide local...")
			tlsConfigProvidedConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
				for _, certChain := range verifiedChains {
					if len(certChain) > 0 {
						// TODO: is verifiedChains already verified against private key?
						// If not, we could possibly validate that here.
						return nil
					}
				}
				return nil
			}
			return tlsConfigProvidedConfig, nil
		}
	} else {
		tlsProviderOverrideFunc = tlsProviderFunc
	}

	logger.Print("Starting server...")
	err := plugin.Serve(&plugin.ServeOpts{
		BackendFactoryFunc: carrierfactory.TrcFactory,
		TLSProviderFunc:    tlsProviderOverrideFunc,
		Logger: hclog.New(&hclog.LoggerOptions{
			Level:      hclog.Trace,
			Output:     logger.Writer(),
			JSONFormat: false,
		}),
	})
	if err != nil {
		logger.Fatal("Plugin shutting down")
	}
}

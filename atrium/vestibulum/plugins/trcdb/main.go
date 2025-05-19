package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"runtime/debug"
	"strings"

	prod "github.com/trimble-oss/tierceron-core/v2/prod"
	"github.com/trimble-oss/tierceron/atrium/buildopts/flowcoreopts"
	"github.com/trimble-oss/tierceron/atrium/buildopts/flowopts"
	"github.com/trimble-oss/tierceron/atrium/buildopts/localopts"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/factory"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcflow/flumen"
	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/deployopts"
	"github.com/trimble-oss/tierceron/buildopts/harbingeropts"
	memonly "github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
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

	tiercerontls.InitRoot()

	logFile := "/var/log/trcplugindb.log"
	f, logErr := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	logger := log.New(f, "[trcplugindb]", log.LstdFlags)
	eUtils.CheckError(&core.CoreConfig{
		ExitOnFailure: true,
		Log:           logger,
	}, logErr, true)
	logger.Println("Beginning plugin startup.")
	if strings.HasSuffix(executableName, "-prod") {
		logger.Println("Running prod plugin")
		prod.SetProd(true)
	}
	buildopts.BuildOptions.SetLogger(func(query string, args ...interface{}) {
		logger.Println(query)
	})
	buildopts.BuildOptions.SetErrorLogger(logger.Writer())
	defer func() {
		if e := recover(); e != nil {
			logger.Printf("%s: %s", e, debug.Stack())
		}
	}()

	factory.Init(buildopts.BuildOptions.ProcessPluginEnvConfig, flumen.BootFlowMachine, true, logger)
	memprotectopts.MemProtectInit(logger)

	apiClientMeta := api.PluginAPIClientMeta{}
	flags := apiClientMeta.FlagSet()

	args := os.Args

	logger.Println("Running plugin with cert validation...")
	if localopts.IsLocal() {
		logger.Println("Running in developer mode with self signed certs.")
		args = append(args, "--tls-skip-verify=true")
	} else {
		logger.Println("Running plugin with cert validation...")
		args = append(args, fmt.Sprintf("--client-cert=%s", "../certs/serv_cert.pem"))
		args = append(args, fmt.Sprintf("--client-key=%s", "../certs/serv_key.pem"))
	}

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
			logger.Print("Tls providing...")
			tlsConfigProvidedConfig, err := tlsProviderFunc()
			if err != nil {
				return nil, err
			}
			logger.Print("Tls provider...")
			logger.Print("Tls provide local...")
			serverIP := net.ParseIP("127.0.0.1") // Change to local IP for self signed cert local debugging
			tlsConfigProvidedConfig.VerifyPeerCertificate = func(certificates [][]byte, verifiedChains [][]*x509.Certificate) error {
				for _, certChain := range verifiedChains {
					for _, cert := range certChain {
						if cert.IPAddresses != nil {
							for _, ip := range cert.IPAddresses {
								if ip.Equal(serverIP) {
									return nil
								}
							}
						}
					}
				}
				logger.Print("TLS certificate verification failed (IP SAN mismatch)")
				return errors.New("TLS certificate verification failed (IP SAN mismatch)")
			}
			return tlsConfigProvidedConfig, nil
		}
	} else {
		tlsProviderOverrideFunc = tlsProviderFunc
	}

	logger.Print("Starting server...")
	err := plugin.Serve(&plugin.ServeOpts{
		BackendFactoryFunc: factory.TrcFactory,
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

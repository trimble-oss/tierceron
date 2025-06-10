package hcore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/plugincoreopts"

	"github.com/trimble-oss/tierceron-core/v2/core"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron-nute-core/mashupsdk"
	"github.com/trimble-oss/tierceron/atrium/speculatio/fenestra/data"
	"github.com/trimble-oss/tierceron/atrium/speculatio/fenestra/fenestrabase"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/pluginlib" // Update package path as needed
	"gopkg.in/yaml.v2"
)

var configContext *tccore.ConfigContext
var sender chan error
var serverAddr *string //another way to do this...
var dfstat *tccore.TTDINode

var DetailedElements []*mashupsdk.MashupDetailedElement

const (
	HELLO_CERT  = "./hello.crt"
	HELLO_KEY   = "./hellokey.key"
	COMMON_PATH = "./config.yml"
)

const (
	FENESTRA_START = iota
	FENESTRA_QUERY
)

func templateIfy(configKey string) string {
	if strings.Contains(HELLO_CERT, ".crt") || strings.Contains(HELLO_CERT, ".key") {
		return fmt.Sprintf("Common/%s.mf.tmpl", configKey[2])
	} else {
		commonBasis := strings.Split(configKey, ".")[1]
		return commonBasis[1:]
	}
}

func receiver(receive_chan *chan core.KernelCmd) {
	for {
		event := <-*receive_chan
		switch {
		case event.Command == tccore.PLUGIN_EVENT_START:
			go start(event.PluginName)
		case event.Command == tccore.PLUGIN_EVENT_STOP:
			go stop(event.PluginName)
			sender <- errors.New("hello shutting down")
			return
		case event.Command == tccore.PLUGIN_EVENT_STATUS:
			//TODO
		case event.Command == FENESTRA_QUERY:

		default:
			//TODO
		}
	}
}

func init() {
	if plugincoreopts.BuildOptions.IsPluginHardwired() {
		return
	}
	peerLib, err := os.Open("plugins/fenestra.so")
	if err != nil {
		fmt.Println("Unable to sha256 plugin")
		return
	}

	defer peerLib.Close()

	h := sha256.New()
	if _, err := io.Copy(h, peerLib); err != nil {
		fmt.Printf("Unable to copy file for sha256 of plugin: %s\n", err)
		return
	}
	sha := hex.EncodeToString(h.Sum(nil))

	fmt.Printf("Fenestra Version: %s\n", sha)
}

func send_dfstat() {
	if configContext == nil || configContext.DfsChan == nil || dfstat == nil {
		fmt.Println("Dataflow Statistic channel not initialized properly for fenestra.")
		return
	}
	dfsctx, _, err := dfstat.GetDeliverStatCtx()
	if err != nil {
		configContext.Log.Println("Failed to get dataflow statistic context: ", err)
		send_err(err)
		return
	}
	pluginlib.SendDfStat(configContext, dfsctx, dfstat)
}

func send_err(err error) {
	if configContext == nil || configContext.ErrorChan == nil || err == nil {
		fmt.Println("Failure to send error message, error channel not initialized properly for fenestra.")
		return
	}
	if dfstat != nil {
		dfsctx, _, err := dfstat.GetDeliverStatCtx()
		if err != nil {
			configContext.Log.Println("Failed to get dataflow statistic context: ", err)
			return
		}
		dfstat.UpdateDataFlowStatistic(dfsctx.FlowGroup,
			dfsctx.FlowName,
			dfsctx.StateName,
			dfsctx.StateCode,
			2,
			func(msg string, err error) {
				configContext.Log.Println(msg, err)
			})
		pluginlib.SendDfStat(configContext, dfsctx, dfstat)
	}
	*configContext.ErrorChan <- err
}

func start(pluginName string) {
	if configContext == nil {
		fmt.Println("no config context initialized for fenestra")
		return
	}
	var config *map[string]any
	var configCert []byte
	var configKey []byte
	var ok bool

	if config, ok = (*configContext.Config)[COMMON_PATH].(*map[string]any); !ok {
		configBytes := (*configContext.Config)[COMMON_PATH].([]byte)
		err := yaml.Unmarshal(configBytes, config)
		if err != nil {
			configContext.Log.Println("Missing common configs")
			send_err(err)
			return
		}
	}
	if configCert, ok = (*configContext.ConfigCerts)[HELLO_CERT]; !ok {
		if configCert, ok = (*configContext.ConfigCerts)[tccore.TRCSHHIVEK_CERT]; !ok {
			configContext.Log.Println("Missing config cert")
			send_err(errors.New("Missing config cert"))
			return
		}
	}
	if configKey, ok = (*configContext.ConfigCerts)[HELLO_KEY]; !ok {
		if configKey, ok = (*configContext.ConfigCerts)[tccore.TRCSHHIVEK_CERT]; !ok {
			configContext.Log.Println("Missing config key")
			send_err(errors.New("Missing config key"))
			return
		}
	}
	callerCreds := flag.String("CREDS", "", "Credentials of caller")
	insecure := flag.Bool("tls-skip-validation", false, "Skip server validation")
	headless := flag.Bool("headless", false, "Run headless")
	serverheadless := flag.Bool("serverheadless", false, "Run server completely headless")
	envPtr := flag.String("env", "QA", "Environment to configure")
	flag.Parse()

	fenestrabase.CommonMain([]byte{},
		configCert,
		configKey,
		callerCreds,    // For ipc
		insecure,       // Run server without tls
		headless,       // fake data
		serverheadless, // No gui?
		envPtr)

	if config != nil {
		if portInterface, ok := (*config)["grpc_server_port"]; ok {
			var fenestraPort int
			if port, ok := portInterface.(int); ok {
				fenestraPort = port
			} else {
				var err error
				fenestraPort, err = strconv.Atoi(portInterface.(string))
				if err != nil {
					configContext.Log.Printf("Failed to process server port: %v", err)
					send_err(err)
					return
				}
			}
			configContext.Log.Printf("Server listening on :%d\n", fenestraPort)
			configContext.Log.Println("Starting server")

			go func(cmd_send_chan *chan tccore.KernelCmd) {
				if cmd_send_chan != nil {
					*cmd_send_chan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_START}
				}
			}(configContext.CmdSenderChan)
			dfstat = tccore.InitDataFlow(nil, configContext.ArgosId, false)
			dfstat.UpdateDataFlowStatistic("System",
				pluginName,
				"Start up",
				"1",
				1,
				func(msg string, err error) {
					configContext.Log.Println(msg, err)
				})
			send_dfstat()
		} else {
			configContext.Log.Println("Missing config: gprc_server_port")
			send_err(errors.New("missing config: gprc_server_port"))
			return
		}
	} else {
		configContext.Log.Println("Missing common configs")
		send_err(errors.New("missing common configs"))
		return
	}

}

func stop(pluginName string) {
	if configContext != nil {
		configContext.Log.Println("Healthcheck received shutdown message from kernel.")
		configContext.Log.Println("Stopping server")
	}
	if configContext != nil {
		configContext.Log.Println("Stopped server")
		configContext.Log.Println("Stopped server for fenestra.")
		dfstat.UpdateDataFlowStatistic("System",
			pluginName,
			"Shutdown",
			"0",
			1, func(msg string, err error) {
				if err != nil {
					configContext.Log.Println(tccore.SanitizeForLogging(err.Error()))
				} else {
					configContext.Log.Println(tccore.SanitizeForLogging(msg))
				}
			})
		send_dfstat()
		*configContext.CmdSenderChan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_STOP}
	}
	dfstat = nil
}

func SetConfigContext(cc *tccore.ConfigContext) {
	configContext = cc
	// TODO: pull these two from ConfigContext
	insecure := false
	headless := false

	if headless {
		DetailedElements = data.GetHeadlessData(&insecure, configContext.Log)
	} else {
		DetailedElements = data.GetData(&insecure, configContext.Log, &configContext.Env)
	}

}

func GetConfigContext(pluginName string) *tccore.ConfigContext { return configContext }

func GetConfigPaths(pluginName string) []string {
	return []string{
		COMMON_PATH,
		HELLO_CERT,
		HELLO_KEY,
	}
}

func PostInit(configContext *tccore.ConfigContext) {
	configContext.Start = start
	sender = *configContext.ErrorChan
	go receiver(configContext.CmdReceiverChan)
}

func Init(pluginName string, properties *map[string]any) {
	var err error

	configContext, err = pluginlib.Init(pluginName, properties, PostInit)
	if err != nil {
		(*properties)["log"].(*log.Logger).Printf("Initialization error: %v", err)
		return
	}
	var certbytes []byte
	var keybytes []byte
	if cert, ok := (*properties)[HELLO_CERT].([]byte); ok && len(cert) > 0 {
		certbytes = cert
		(*configContext.ConfigCerts)[HELLO_CERT] = certbytes
	}
	if key, ok := (*properties)[HELLO_CERT].([]byte); ok && len(key) > 0 {
		keybytes = key
		(*configContext.ConfigCerts)[HELLO_KEY] = keybytes
	}
	if _, ok := (*properties)[COMMON_PATH]; !ok {
		fmt.Println("Missing common config components")
		return
	}
}

func GetPluginMessages(pluginName string) []string {
	return []string{}
}

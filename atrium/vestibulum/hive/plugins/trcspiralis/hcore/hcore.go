package hcore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/plugincoreopts"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"gopkg.in/yaml.v2"
)

var (
	configContextMap map[string]*tccore.ConfigContext
	sender           chan error
	serverAddr       *string // another way to do this...
	dfstat           *tccore.TTDINode
)

const (
	HELLO_CERT  = "./hello.crt"
	HELLO_KEY   = "./hellokey.key"
	COMMON_PATH = "./config.yml"
)

func templateIfy(configKey string) string {
	if strings.Contains(HELLO_CERT, ".crt") || strings.Contains(HELLO_CERT, ".key") {
		return fmt.Sprintf("Common/%s.mf.tmpl", string(configKey[2]))
	} else {
		commonBasis := strings.Split(configKey, ".")[1]
		return commonBasis[1:]
	}
}

func receiverSpiralis(configContext *tccore.ConfigContext, receive_chan *chan tccore.KernelCmd) {
	for {
		event := <-*receive_chan
		switch {
		case event.Command == tccore.PLUGIN_EVENT_START:
			go configContext.Start(event.PluginName)
		case event.Command == tccore.PLUGIN_EVENT_STOP:
			go stop(event.PluginName)
			sender <- errors.New("spiralis shutting down")
			return
		case event.Command == tccore.PLUGIN_EVENT_STATUS:
			// TODO
		default:
			// TODO
		}
	}
}

func init() {
	if plugincoreopts.BuildOptions.IsPluginHardwired() {
		return
	}
	peerExe, err := os.Open("plugins/spiralis.so")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Spiralis unable to sha256 plugin")
		return
	}

	defer peerExe.Close()

	h := sha256.New()
	if _, err := io.Copy(h, peerExe); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to copy file for sha256 of plugin: %s\n", err)
		return
	}
	sha := hex.EncodeToString(h.Sum(nil))
	fmt.Fprintf(os.Stderr, "Spiralis Version: %s\n", sha)
}

func send_dfstat(pluginName string) {
	if configContext, ok := configContextMap[pluginName]; ok {
		if configContext == nil || configContext.DfsChan == nil || dfstat == nil {
			fmt.Fprintln(os.Stderr, "Dataflow Statistic channel not initialized properly for spiralis.")
			return
		}
		dfsctx, _, err := dfstat.GetDeliverStatCtx()
		if err != nil {
			configContext.Log.Println("Failed to get dataflow statistic context: ", err)
			send_err(pluginName, err)
			return
		}
		tccore.SendDfStat(configContext, dfsctx, dfstat)
	}
}

func send_err(pluginName string, err error) {
	if configContext, ok := configContextMap[pluginName]; ok {
		if configContext == nil || configContext.ErrorChan == nil || err == nil {
			fmt.Fprintln(os.Stderr, "Failure to send error message, error channel not initialized properly for spiralis.")
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
			tccore.SendDfStat(configContext, dfsctx, dfstat)
		}
		*configContext.ErrorChan <- err
	}
}

func start(pluginName string) {
	if configContextMap == nil {
		fmt.Fprintln(os.Stderr, "no config context initialized for spiralis")
		return
	}

	if configContext, ok := configContextMap[pluginName]; ok {
		var config *map[string]any
		var ok bool
		if config, ok = (*configContext.Config)[COMMON_PATH].(*map[string]any); !ok {
			configBytes := (*configContext.Config)[COMMON_PATH].([]byte)
			err := yaml.Unmarshal(configBytes, &config)
			if err != nil {
				configContext.Log.Println("Missing common configs")
				send_err(pluginName, err)
				return
			}
		}

		if config != nil {
			if portInterface, ok := (*config)["grpc_server_port"]; ok {
				var spiralisPort int
				if port, ok := portInterface.(int); ok {
					spiralisPort = port
				} else {
					var err error
					spiralisPort, err = strconv.Atoi(portInterface.(string))
					if err != nil {
						configContext.Log.Printf("Failed to process server port: %v", err)
						send_err(pluginName, err)
						return
					}
				}
				configContext.Log.Printf("Server listening on :%d\n", spiralisPort)
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
				send_dfstat(pluginName)
			} else {
				configContext.Log.Println("Missing config: gprc_server_port")
				send_err(pluginName, errors.New("missing config: gprc_server_port"))
				return
			}
		} else {
			configContext.Log.Println("Missing common configs")
			send_err(pluginName, errors.New("missing common configs"))
			return
		}
	}
}

func stop(pluginName string) {
	configContext := configContextMap[pluginName]

	if configContext != nil {
		configContext.Log.Println("Spiralis received shutdown message from kernel.")
		configContext.Log.Println("Stopping server")
	}
	if configContext != nil {
		configContext.Log.Println("Stopped server")
		configContext.Log.Println("Stopped server for spiralis.")
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
		send_dfstat(pluginName)
		*configContext.CmdSenderChan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_STOP}
	}
	dfstat = nil
}

func GetConfigContext(pluginName string) *tccore.ConfigContext {
	return configContextMap[pluginName]
}

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
	go receiverSpiralis(configContext, configContext.CmdReceiverChan)
}

func Init(pluginName string, properties *map[string]any) {
	var err error
	if configContextMap == nil {
		configContextMap = map[string]*tccore.ConfigContext{}
	}
	configContextMap[pluginName], err = tccore.InitPost(pluginName, properties, PostInit)
	if err != nil && properties != nil && (*properties)["log"] != nil {
		(*properties)["log"].(*log.Logger).Printf("Initialization error: %v", err)
		return
	}
	var certbytes []byte
	var keybytes []byte
	if cert, ok := (*properties)[HELLO_CERT].([]byte); ok && len(cert) > 0 {
		certbytes = cert
		(*configContextMap[pluginName].ConfigCerts)[HELLO_CERT] = certbytes
	}
	if key, ok := (*properties)[HELLO_KEY].([]byte); ok && len(key) > 0 {
		keybytes = key
		(*configContextMap[pluginName].ConfigCerts)[HELLO_KEY] = keybytes
	}
	if _, ok := (*properties)[COMMON_PATH]; !ok {
		fmt.Fprintln(os.Stderr, "Missing common config components")
		return
	}
}

func GetPluginMessages(pluginName string) []string {
	return []string{}
}

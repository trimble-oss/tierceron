package hcore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/plugincoreopts"
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcrosea/rosea"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"gopkg.in/yaml.v2"
)

var configContext *tccore.ConfigContext
var sender chan error
var dfstat *tccore.TTDINode

var projectServices [][]any

const (
	COMMON_PATH = "./config.yml"
)

func receiver(receive_chan chan tccore.KernelCmd) {
	for {
		event := <-receive_chan
		switch {
		case event.Command == tccore.PLUGIN_EVENT_START:
			go start(event.PluginName)
		case event.Command == tccore.PLUGIN_EVENT_STOP:
			go stop(event.PluginName)
			sender <- errors.New("rosea shutting down")
			return
		case event.Command == tccore.PLUGIN_EVENT_STATUS:
			//TODO
		default:
			//TODO
		}
	}
}

func init() {
	if plugincoreopts.BuildOptions.IsPluginHardwired() {
		return
	}
	peerExe, err := os.Open("plugins/rosea.so")
	if err != nil {
		fmt.Println("Unable to sha256 plugin")
		return
	}

	defer peerExe.Close()

	h := sha256.New()
	if _, err := io.Copy(h, peerExe); err != nil {
		fmt.Printf("Unable to copy file for sha256 of plugin: %s\n", err)
		return
	}
	sha := hex.EncodeToString(h.Sum(nil))
	fmt.Printf("rosea Version: %s\n", sha)
}

func send_dfstat() {
	if configContext == nil || configContext.DfsChan == nil || dfstat == nil {
		fmt.Println("Dataflow Statistic channel not initialized properly for rosea.")
		return
	}
	dfsctx, _, err := dfstat.GetDeliverStatCtx()
	if err != nil {
		configContext.Log.Println("Failed to get dataflow statistic context: ", err)
		send_err(err)
		return
	}
	tccore.SendDfStat(configContext, dfsctx, dfstat)
}

func send_err(err error) {
	if configContext == nil || configContext.ErrorChan == nil || err == nil {
		fmt.Println("Failure to send error message, error channel not initialized properly for rosea.")
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

func chat_receiver(chat_receive_chan chan *tccore.ChatMsg) {
	for {
		event := <-chat_receive_chan
		switch {
		case event == nil:
			continue
		case *event.Name == "SHUTDOWN":
			configContext.Log.Println("rosea shutting down message receiver")
			return
		case (event.Response != nil && *event.Response == "Service unavailable") || (event.TrcdbExchange != nil):
			if *event.Response == "Service unavailable" {
				time.Sleep(5 * time.Second)
				FetchSocii(configContext)
			} else {
				if event.TrcdbExchange != nil && len(event.TrcdbExchange.Response.Rows) != 0 {
					projectServices = event.TrcdbExchange.Response.Rows
					err := rosea.BootInit(projectServices)
					if err != nil {
						configContext.Log.Printf("Rosea Initialization error: %v", err)
						return
					}
				} else {
					time.Sleep(5 * time.Second)
					FetchSocii(configContext)
				}
			}

		default:
			configContext.Log.Println("rosea received chat message")
		}
	}
}

func start(pluginName string) {
	if configContext == nil {
		fmt.Println("no config context initialized for rosea")
		return
	}
	var config *map[string]any
	var ok bool
	if len(*configContext.Config) > 0 {
		if config, ok = (*configContext.Config)[COMMON_PATH].(*map[string]any); !ok {
			configBytes := (*configContext.Config)[COMMON_PATH].([]byte)
			err := yaml.Unmarshal(configBytes, &config)
			if err != nil {
				configContext.Log.Println("Missing common configs")
				send_err(err)
				return
			}
		}
	}

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
	if configContext.CmdSenderChan != nil {
		*configContext.CmdSenderChan <- tccore.KernelCmd{
			Command: tccore.PLUGIN_EVENT_START,
		}
	}
}

func stop(pluginName string) {
	if configContext != nil {
		configContext.Log.Println("rosea received shutdown message from kernel.")
	}
	if configContext != nil {
		configContext.Log.Println("Stopped server for rosea.")
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

func GetConfigContext(pluginName string) *tccore.ConfigContext { return configContext }

func GetConfigPaths(pluginName string) []string {
	return []string{
		COMMON_PATH,
	}
}

func PostInit(configContext *tccore.ConfigContext) {
	configContext.Start = start
	sender = *configContext.ErrorChan
	go receiver(*configContext.CmdReceiverChan)
}

func Init(pluginName string, properties *map[string]any) {
	var err error

	configContext, err = tccore.Init(properties,
		tccore.TRCSHHIVEK_CERT,
		tccore.TRCSHHIVEK_KEY,
		COMMON_PATH,
		"rosea",
		start,
		receiver,
		chat_receiver,
	)
	if err != nil {
		(*properties)["log"].(*log.Logger).Printf("Initialization error: %v", err)
		return
	}
	if _, ok := (*properties)[COMMON_PATH]; !ok {
		fmt.Println("Missing common config components")
		return
	}

	FetchSocii(configContext)
}

// GetDatabaseName - returns a name to be used by TrcDb.
func GetDatabaseName() string {
	return "TrcDb"
}

func GetPluginMessages(pluginName string) []string {
	return []string{}
}

func FetchSocii(ctx *tccore.ConfigContext) {
	ctx.Log.Println("Sending request for argos socii.")

	id := "SYSTEM"
	chatResultMsg := tccore.ChatMsg{
		ChatId: &id,
	}
	name := "rosea"
	chatResultMsg.Name = &name
	chatResultMsg.Query = &[]string{"trcdb"}
	chatResultMsg.TrcdbExchange = &tccore.TrcdbExchange{
		Query:     fmt.Sprintf("SELECT * FROM %s.%s", GetDatabaseName(), flowcore.ArgosSociiFlow.TableName()),
		Flows:     []string{flowcore.ArgosSociiFlow.TableName()},
		Operation: "SELECT",
	}
	if ctx.ChatSenderChan != nil {
		go func() {
			*ctx.ChatSenderChan <- &chatResultMsg
		}()
	}
}

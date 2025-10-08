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

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"gopkg.in/yaml.v2"
)

var configContext *tccore.ConfigContext
var sender chan error
var dfstat *tccore.TTDINode

var trcdbArgosQuery *tccore.ChatMsg
var pipelineArgosIds chan *[]string = make(chan *[]string)

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
			sender <- errors.New("descartes shutting down")
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
	peerExe, err := os.Open("plugins/descartes.so")
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
	fmt.Printf("descartes Version: %s\n", sha)
}

func send_dfstat() {
	if configContext == nil || configContext.DfsChan == nil || dfstat == nil {
		fmt.Println("Dataflow Statistic channel not initialized properly for descartes.")
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
		fmt.Println("Failure to send error message, error channel not initialized properly for descartes.")
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
			fallthrough
		case *event.Name == "SHUTDOWN":
			configContext.Log.Println("descartes shutting down message receiver")
			return
		case event.Response != nil && *event.Response == "Service unavailable" && event.ChatId != nil && *event.ChatId == "trcdb":
			time.Sleep(5 * time.Second)
			// Retry pipeline status query
			configContext.Log.Println("descartes unable to access trcdb service.")
			if configContext.ChatSenderChan != nil {
				trcdbArgosQuery.Response = new(string)
				*configContext.ChatSenderChan <- trcdbArgosQuery
			}
		case event.ChatId != nil && (*event).ChatId != nil && *event.ChatId == "PROGRESS":
			configContext.Log.Println("Sending progress results back to kernel.")
			progressResp := "Running Descartes Diagnostics..."
			(*event).Response = &progressResp
			*configContext.ChatSenderChan <- event
		case event.ChatId != nil && (*event).ChatId != nil && *event.ChatId != "PROGRESS":
			configContext.Log.Println("descartes request")
			event.StatisticsDoc = processRequest(event.ChatId)
			configContext.Log.Println("Sending statistics document to kernel.")
			*configContext.ChatSenderChan <- event
		case (*event).TrcdbExchange != nil && (*event).ChatId != nil && *event.ChatId == "trcdb":
			configContext.Log.Println("Descartes received trcdb exchange message")
			if (*event).TrcdbExchange.Response.Rows == nil || len((*event).TrcdbExchange.Response.Rows) == 0 {
				configContext.Log.Println("Descartes received no results from trcdb exchange.")
				pipelineArgosIds <- nil
			} else {
				argosIDs := []string{}
				for _, row := range (*event).TrcdbExchange.Response.Rows {
					if len(row) == 1 && row[0] != nil {
						argosIDs = append(argosIDs, row[0].(string))
					}
				}

				pipelineArgosIds <- &argosIDs
			}
		default:
			configContext.Log.Println("descartes received chat message")
		}
	}
}

func processRequest(chatId *string) *tccore.StatisticsDoc {
	switch {
	case *chatId == "trcdb":
		chatResultMsg := tccore.ChatMsg{
			ChatId: chatId,
		}
		name := "descartes"
		chatResultMsg.Name = &name
		chatResultMsg.Response = new(string)
		chatResultMsg.Query = &[]string{"trcdb"}
		chatResultMsg.TrcdbExchange = &tccore.TrcdbExchange{} //TODO: Securely load query for argos ids
		trcdbArgosQuery = &chatResultMsg
		*configContext.ChatSenderChan <- trcdbArgosQuery
		argosIds := <-pipelineArgosIds
		if argosIds == nil || len(*argosIds) == 0 {
			configContext.Log.Println("No argos ids found for statistics document")
			return nil
		}
		statDocs := make([]any, len(*argosIds))
		for i, v := range *argosIds {
			statDocs[i] = v
		}
		return &tccore.StatisticsDoc{
			StatDocs: statDocs,
		}
	default:
		configContext.Log.Println("Unknown chat ID")
	}
	return nil
}

func start(pluginName string) {
	if configContext == nil {
		fmt.Println("no config context initialized for descartes")
		return
	}
	var config map[string]any
	var ok bool
	if config, ok = (*configContext.Config)[COMMON_PATH].(map[string]any); !ok {
		configBytes := (*configContext.Config)[COMMON_PATH].([]byte)
		err := yaml.Unmarshal(configBytes, &config)
		if err != nil {
			configContext.Log.Println("Missing common configs")
			send_err(err)
			return
		}
	}

	if config != nil {
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
		configContext.Log.Println("Missing common configs")
		send_err(errors.New("missing common configs"))
		return
	}
}

func stop(pluginName string) {
	if configContext != nil {
		configContext.Log.Println("descartes received shutdown message from kernel.")
	}
	if configContext != nil {
		configContext.Log.Println("Stopped server for descartes.")
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
		"descartes",
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
}

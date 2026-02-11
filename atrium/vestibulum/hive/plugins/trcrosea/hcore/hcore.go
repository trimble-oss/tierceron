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
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcrosea/hcore/flowutil"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcrosea/rosea"
	roseacore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcrosea/rosea/core"
	editor "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcrosea/rosea/editor"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
)

var (
	configContext *tccore.ConfigContext
	sender        chan error
	dfstat        *tccore.TTDINode
)

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
	peerExe, err := os.Open("plugins/rosea.so")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Rosea unable to sha256 plugin")
		return
	}

	defer peerExe.Close()

	h := sha256.New()
	if _, err := io.Copy(h, peerExe); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to copy file for sha256 of plugin: %s\n", err)
		return
	}
	sha := hex.EncodeToString(h.Sum(nil))
	fmt.Fprintf(os.Stderr, "rosea Version: %s\n", sha)
}

func send_dfstat() {
	if configContext == nil || configContext.DfsChan == nil || dfstat == nil {
		fmt.Fprintln(os.Stderr, "Dataflow Statistic channel not initialized properly for rosea.")
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
		fmt.Fprintln(os.Stderr, "Failure to send error message, error channel not initialized properly for rosea.")
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
		case (*event).ChatId != nil && *event.ChatId == "rosea":
			// Handle rosea editor request from trcshcmd
			configContext.Log.Println("rosea received rosea editor request")

			// Extract data from HookResponse
			if event.HookResponse != nil {
				if dataMap, ok := event.HookResponse.(map[string]interface{}); ok {
					filename := dataMap["filename"].(string)
					content := dataMap["content"].([]byte)
					memfs := dataMap["memfs"]

					configContext.Log.Printf("Opening rosea editor for file: %s\n", filename)

					// Launch editor with the file content
					// Store memfs and filename for save operation
					roseacore.SetRoseaContext(memfs, filename)

					// Initialize and run editor synchronously
					editorModel := editor.InitRoseaEditor(filename, &content)
					editorErr := rosea.RunRoseaEditor(editorModel)

					// Send completion message back to trcsh with original RoutingId
					completionMsg := "Editor closed"
					if editorErr != nil {
						configContext.Log.Printf("Error running rosea editor: %v\n", editorErr)
						completionMsg = fmt.Sprintf("Editor error: %v", editorErr)
					}

					// Send response back using the routing ID from the request
					if event.RoutingId != nil && configContext.ChatSenderChan != nil {
						pluginName := "rosea"
						responseMsg := &tccore.ChatMsg{
							Name:      &pluginName,
							Query:     &[]string{"trcsh"},
							RoutingId: event.RoutingId,
							Response:  &completionMsg,
						}
						*configContext.ChatSenderChan <- responseMsg
					}

					configContext.Log.Println("rosea editor session completed")
				}
			}
		case (*event).ChatId != nil && *event.ChatId != "PROGRESS":
			chatMsgHookCtxRef := flowutil.GetChatMsgHookCtx()
			tccore.CallSelectedChatMsgHook(*chatMsgHookCtxRef, event)
		default:
			configContext.Log.Println("rosea received chat message")
		}
	}
}

func start(pluginName string) {
	if configContext == nil {
		fmt.Fprintln(os.Stderr, "no config context initialized for rosea")
		return
	}

	// Validate Config exists before accessing
	if configContext.Config == nil {
		if configContext.Log != nil {
			configContext.Log.Println("Warning: Config is nil in rosea start, continuing without config")
		}
		// Continue without config - KernelZ mode doesn't need it
		return
	}

	// var config *map[string]any
	// var ok bool
	// if len(*configContext.Config) > 0 {
	// Check if COMMON_PATH exists in Config
	// configData, exists := (*configContext.Config)[COMMON_PATH]
	// if !exists {
	// 	if configContext.Log != nil {
	// 		configContext.Log.Println("Warning: COMMON_PATH not found in Config, continuing without common config")
	// 	}
	// 	// Continue without common config - KernelZ mode doesn't need it
	// 	return
	// } else {
	// 	// Try to get as map first
	// 	if config, ok = configData.(*map[string]any); !ok {
	// 		// Try as bytes
	// 		if configBytes, ok := configData.([]byte); ok {
	// 			err := yaml.Unmarshal(configBytes, &config)
	// 			if err != nil {
	// 				if configContext.Log != nil {
	// 					configContext.Log.Printf("Warning: Failed to unmarshal common config: %v\n", err)
	// 				}
	// 				send_err(err)
	// 				return
	// 			}
	// 		} else {
	// 			if configContext.Log != nil {
	// 				configContext.Log.Println("Warning: COMMON_PATH data is not map or byte array, continuing anyway")
	// 			}
	// 			return
	// 		}
	// 	}
	// }
	//	}

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

	// Validate properties pointer
	if properties == nil {
		fmt.Fprintln(os.Stderr, "Rosea Init: properties is nil")
		return
	}

	// Refuse to run on Kubernetes
	if isK8s, ok := (*properties)["isKubernetes"].(bool); ok && isK8s {
		if logger, ok := (*properties)["log"].(*log.Logger); ok && logger != nil {
			logger.Printf("Rosea plugin is not allowed to run on Kubernetes. Refusing to initialize.")
		}
		(*properties)["pluginRefused"] = true
		return
	}

	// Check if running in KernelZ mode (editor-only mode)
	isKernelZ := false
	if kernelZ, ok := (*properties)["isKernelZ"].(bool); ok && kernelZ {
		isKernelZ = true
	}

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
		if logger, ok := (*properties)["log"].(*log.Logger); ok && logger != nil {
			logger.Printf("Rosea initialization error: %v", err)
		} else {
			fmt.Fprintf(os.Stderr, "Rosea initialization error: %v\n", err)
		}
		return
	}

	// Check for COMMON_PATH config - only warn if missing in non-KernelZ mode
	if _, ok := (*properties)[COMMON_PATH]; !ok {
		if !isKernelZ {
			if configContext != nil && configContext.Log != nil {
				configContext.Log.Println("Warning: Missing common config components, continuing anyway")
			} else {
				fmt.Fprintln(os.Stderr, "Warning: Missing common config components")
			}
		}
	}

	if configContext != nil && configContext.ChatSenderChan != nil {
		flowutil.InitChatSenderChan(configContext.ChatSenderChan)
	}

	// Only fetch Socii if not running in KernelZ mode (editor-only)
	if !isKernelZ && configContext != nil {
		go FetchSocii(configContext) // Init must be non blocking
	}
}

func GetPluginMessages(pluginName string) []string {
	return []string{}
}

func FetchSocii(ctx *tccore.ConfigContext) {
	ctx.Log.Println("Sending request for argos socii.")
	chatResponseMsg := tccore.CallChatQueryChan(flowutil.GetChatMsgHookCtx(),
		"rosea", // From rainier
		&tccore.TrcdbExchange{
			Flows:     []string{flowcore.ArgosSociiFlow.TableName()},                                 // Flows
			Query:     fmt.Sprintf("SELECT * FROM %s.%s", "%s", flowcore.ArgosSociiFlow.TableName()), // Query letting engine provide database name
			Operation: "SELECT",                                                                      // query operation
		},
		flowutil.GetChatSenderChan(),
	)
	if chatResponseMsg.TrcdbExchange != nil && len(chatResponseMsg.TrcdbExchange.Response.Rows) > 0 {
		projectServices = chatResponseMsg.TrcdbExchange.Response.Rows
		err := rosea.BootInit(projectServices)
		if err != nil {
			ctx.Log.Printf("Rosea Initialization error: %v", err)
			return
		}
		ctx.Log.Println("rosea initialized with argos socii data.")
	} else {
		ctx.Log.Println("rosea failed to initialize with argos socii data.")
		if chatResponseMsg.Response != nil && *chatResponseMsg.Response == "Service unavailable" || len(chatResponseMsg.TrcdbExchange.Response.Rows) == 0 {
			ctx.Log.Println("Service unavailable, retrying in 5 seconds.")
			time.Sleep(5 * time.Second)
			FetchSocii(ctx)
		}
	}
}

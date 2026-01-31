package hcore

import (
	"errors"
	"fmt"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron/pkg/core/util/hive/plugins/trcshcmd/shellcmd"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

var (
	configContext *tccore.ConfigContext
	driverConfig  *config.DriverConfig
)

func receiver(receive_chan chan tccore.KernelCmd) {
	for {
		event := <-receive_chan
		switch {
		case event.Command == tccore.PLUGIN_EVENT_START:
			go start(event.PluginName)
		case event.Command == tccore.PLUGIN_EVENT_STOP:
			go stop(event.PluginName)
			if configContext != nil && configContext.ErrorChan != nil {
				*configContext.ErrorChan <- errors.New("trcshcmd shutting down")
			}
			return
		case event.Command == tccore.PLUGIN_EVENT_STATUS:
			// TODO
		default:
			// TODO
		}
	}
}

func chat_receiver(chat_receive_chan chan *tccore.ChatMsg) {
	for {
		event := <-chat_receive_chan
		switch {
		case event == nil:
			continue
		case event.Name != nil && *event.Name == "SHUTDOWN":
			if configContext != nil {
				configContext.Log.Println("trcshcmd shutting down message receiver")
			}
			return
		case event.Response != nil && *event.Response != "":
			// Handle shell command requests
			cmdType := *event.Response

			// Check if this is a shell command type
			if cmdType == shellcmd.CmdTrcConfig || cmdType == shellcmd.CmdTrcPub ||
				cmdType == shellcmd.CmdTrcSub || cmdType == shellcmd.CmdTrcX ||
				cmdType == shellcmd.CmdTrcInit || cmdType == shellcmd.CmdTrcPlgtool ||
				cmdType == shellcmd.CmdKubectl {

				if configContext != nil {
					configContext.Log.Printf("Received shell command request: %s\n", cmdType)
				}

				// Extract args from HookResponse field
				var args []string
				if event.HookResponse != nil {
					if argsList, ok := event.HookResponse.([]string); ok {
						args = argsList
					}
				}

				// Execute command - output written to MemFs
				memFs := shellcmd.ExecuteShellCommand(cmdType, args, driverConfig)

				// Return the MemFs in HookResponse
				responseMsg := fmt.Sprintf("Command %s completed", cmdType)
				event.Response = &responseMsg
				event.HookResponse = memFs

				// Send response back to requesting plugin
				if configContext != nil && configContext.ChatSenderChan != nil {
					*configContext.ChatSenderChan <- event
				}
			}
		default:
			if configContext != nil {
				configContext.Log.Println("trcshcmd received non-command chat message")
			}
		}
	}
}

func start(pluginName string) {
	if configContext == nil {
		fmt.Println("no config context initialized for trcshcmd")
		return
	}

	configContext.Log.Printf("Shell command plugin %s started\n", pluginName)
}

func stop(pluginName string) {
	if configContext == nil {
		return
	}
	configContext.Log.Printf("Shell command plugin %s stopped\n", pluginName)
}

func GetConfigPaths(pluginName string) []string {
	return []string{}
}

func Init(pluginName string, properties *map[string]any) {
	// Check if running in Kubernetes - refuse to initialize
	if isKubernetes, ok := (*properties)["isKubernetes"].(bool); ok && isKubernetes {
		(*properties)["pluginRefused"] = true
		return
	}

	// Get DriverConfig from properties
	if dc, ok := (*properties)["driverConfig"].(*config.DriverConfig); ok {
		driverConfig = dc
	}

	var err error
	configContext, err = tccore.Init(
		properties,
		"",
		"",
		"",
		"trcshcmd",
		func(s string) {},
		receiver,
		chat_receiver,
	)
	if err != nil {
		fmt.Printf("Failure to init context for trcshcmd: %v\n", err)
		return
	}
}

// Start sends the START command to the trcshcmd plugin
func Start() {
	if configContext != nil && configContext.CmdSenderChan != nil {
		*configContext.CmdSenderChan <- tccore.KernelCmd{
			PluginName: "trcshcmd",
			Command:    tccore.PLUGIN_EVENT_START,
		}
	}
}

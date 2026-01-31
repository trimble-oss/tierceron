package hcore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/plugincoreopts"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron/pkg/core/util/hive/shellcmd"
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
				*configContext.ErrorChan <- errors.New("trcshellcmd shutting down")
			}
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
	peerExe, err := os.Open("plugins/trcshellcmd.so")
	if err != nil {
		fmt.Fprintln(os.Stderr, "trcshellcmd unable to sha256 plugin")
		return
	}

	defer peerExe.Close()

	h := sha256.New()
	if _, err := io.Copy(h, peerExe); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to copy file for sha256 of plugin: %s\n", err)
		return
	}
	sha := hex.EncodeToString(h.Sum(nil))
	fmt.Fprintf(os.Stderr, "trcshellcmd Version: %s\n", sha)
}

func chat_receiver(chat_receive_chan chan *tccore.ChatMsg) {
	for {
		event := <-chat_receive_chan
		switch {
		case event == nil:
			continue
		case event.Name != nil && *event.Name == "SHUTDOWN":
			configContext.Log.Println("trcshellcmd shutting down message receiver")
			return
		case event.Response != nil && *event.Response != "":
			// Handle shell command requests
			cmdType := *event.Response

			// Check if this is a shell command type
			if cmdType == shellcmd.CmdTrcConfig || cmdType == shellcmd.CmdTrcPub ||
				cmdType == shellcmd.CmdTrcSub || cmdType == shellcmd.CmdTrcX ||
				cmdType == shellcmd.CmdTrcInit || cmdType == shellcmd.CmdTrcPlgtool ||
				cmdType == shellcmd.CmdKubectl {

				configContext.Log.Printf("Received shell command request: %s\n", cmdType)

				// Extract args from HookResponse field
				var args []string
				if event.HookResponse != nil {
					if argsList, ok := event.HookResponse.([]string); ok {
						args = argsList
					}
				}

				// Execute command
				result := shellcmd.ExecuteShellCommand(cmdType, args, driverConfig)

				// Send result back via ChatMsg
				responseMsg := fmt.Sprintf("Command %s completed: exit_code=%d", cmdType, result.ExitCode)
				if result.Error != "" {
					responseMsg = fmt.Sprintf("Command %s failed: %s", cmdType, result.Error)
				}
				event.Response = &responseMsg
				event.HookResponse = result

				// Send response back to requesting plugin
				if configContext.ChatSenderChan != nil {
					*configContext.ChatSenderChan <- event
				}
			}
		default:
			configContext.Log.Println("trcshellcmd received non-command chat message")
		}
	}
}

func start(pluginName string) {
	if configContext == nil {
		fmt.Fprintln(os.Stderr, "no config context initialized for trcshellcmd")
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

func GetConfigContext(pluginName string) *tccore.ConfigContext {
	return configContext
}

func Init(pluginName string, properties *map[string]any) {
	// Check if running in Kubernetes - refuse to initialize
	if isKubernetes, ok := (*properties)["isKubernetes"].(bool); ok && isKubernetes {
		(*properties)["pluginRefused"] = true
		if logger, ok := (*properties)["log"]; ok {
			logger.(*os.File).WriteString("trcshellcmd plugin refused to initialize in Kubernetes environment\n")
		}
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
		"trcshellcmd",
		func(s string) {},
		receiver,
		chat_receiver,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failure to init context for trcshellcmd: %v\n", err)
		return
	}
}

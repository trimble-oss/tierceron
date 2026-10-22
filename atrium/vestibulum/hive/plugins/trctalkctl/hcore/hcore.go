package hcore

import (
	"context"
	"log"
	"strings"
	"time"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcshtalk/ttcore/common"
)

type commandPlugin struct {
	config *tccore.ConfigContext
	ctx    context.Context
	cancel context.CancelFunc
	send   func(context.Context, string, string) (string, error)
}

func GetConfigPaths(pluginName string) []string {
	return nil
}

func Init(pluginName string, properties *map[string]any) {
	InitWithSender(pluginName, properties, common.SendHubCommand)
}

func InitWithSender(pluginName string, properties *map[string]any, send func(context.Context, string, string) (string, error)) {
	if properties == nil || *properties == nil {
		return
	}
	logger, _ := (*properties)["log"].(*log.Logger)
	isKernelZ, _ := (*properties)["isKernelZ"].(bool)
	isKubernetes, _ := (*properties)["isKubernetes"].(bool)
	if !isKernelZ || isKubernetes || logger == nil {
		(*properties)["pluginRefused"] = true
		if logger != nil {
			logger.Println("Talkctl requires a TrcshZ kernel and its logger.")
		}
		return
	}
	if send == nil {
		send = common.SendHubCommand
	}
	ctx, cancel := context.WithCancel(context.Background())
	plugin := &commandPlugin{ctx: ctx, cancel: cancel, send: send}
	config, err := tccore.InitWithPlugin(pluginName, properties, "", "", "", "talkctl", plugin.start, plugin.receiver, plugin.chatReceiver)
	if err != nil {
		cancel()
		(*properties)["pluginRefused"] = true
		logger.Printf("Talkctl initialization failed: %v", err)
		return
	}
	plugin.config = config
}

func (plugin *commandPlugin) notify(pluginName string, command int) {
	if plugin.config == nil || plugin.config.CmdSenderChan == nil || *plugin.config.CmdSenderChan == nil {
		return
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case *plugin.config.CmdSenderChan <- tccore.KernelCmd{PluginName: pluginName, Command: command}:
	case <-timer.C:
	}
}

func (plugin *commandPlugin) start(pluginName string) {
	plugin.notify(pluginName, tccore.PLUGIN_EVENT_START)
}

func (plugin *commandPlugin) receiver(commands chan tccore.KernelCmd) {
	for {
		select {
		case event, open := <-commands:
			if !open {
				plugin.cancel()
				return
			}
			switch event.Command {
			case tccore.PLUGIN_EVENT_START:
				plugin.start(event.PluginName)
			case tccore.PLUGIN_EVENT_STOP:
				plugin.cancel()
				plugin.notify(event.PluginName, tccore.PLUGIN_EVENT_STOP)
				return
			}
		case <-plugin.ctx.Done():
			return
		}
	}
}

func shellCommand(event *tccore.ChatMsg) bool {
	return event != nil && !event.IsBroadcast && event.Response == nil &&
		event.Name != nil && *event.Name == "talkctl" &&
		event.Query != nil && len(*event.Query) == 1 && (*event.Query)[0] == "trcsh" &&
		event.ChatId != nil && *event.ChatId == "talkctl" &&
		event.RoutingId != nil && strings.TrimSpace(*event.RoutingId) != ""
}

func (plugin *commandPlugin) chatReceiver(messages chan *tccore.ChatMsg) {
	for {
		select {
		case event, open := <-messages:
			if !open || (event != nil && event.Name != nil && *event.Name == "SHUTDOWN") {
				plugin.cancel()
				return
			}
			if shellCommand(event) {
				go plugin.handleCommand(event)
			}
		case <-plugin.ctx.Done():
			return
		}
	}
}

func (plugin *commandPlugin) handleCommand(event *tccore.ChatMsg) {
	if !shellCommand(event) {
		return
	}
	args, valid := event.HookResponse.([]string)
	prompt := strings.TrimSpace(strings.Join(args, " "))
	result := "Usage: @talkctl <command...>"
	if valid && prompt != "" {
		ctx, cancel := context.WithTimeout(plugin.ctx, 90*time.Second)
		response, err := plugin.send(ctx, *event.RoutingId, prompt)
		cancel()
		if err != nil {
			result = "Talkctl: " + err.Error()
		} else {
			result = response
		}
	}
	if plugin.config == nil || plugin.config.ChatSenderChan == nil || *plugin.config.ChatSenderChan == nil {
		return
	}
	name, command := "talkctl", "talkctl"
	reply := &tccore.ChatMsg{
		Name:      &name,
		Query:     &[]string{"trcsh"},
		ChatId:    &command,
		RoutingId: event.RoutingId,
		Response:  &result,
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case *plugin.config.ChatSenderChan <- reply:
	case <-plugin.ctx.Done():
	case <-timer.C:
	}
}

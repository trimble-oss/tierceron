package shell

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
)

func (m *ShellModel) executeTalkctlCommand(args []string) tea.Msg {
	fail := func(output ...string) tea.Msg {
		return commandResultMsg{output: output}
	}
	if !m.elevatedMode {
		return fail(errorStyle.Render("Error: '@talkctl' command requires elevated access"), "Run 'su' to obtain elevated access")
	}
	if m.chatSenderChan == nil || *m.chatSenderChan == nil {
		return fail(errorStyle.Render("Error: chat channel not available"))
	}
	if len(args) == 0 || strings.TrimSpace(strings.Join(args, " ")) == "" {
		return fail("Usage: @talkctl <command...>")
	}
	id := fmt.Sprintf("talkctl-%d", time.Now().UnixNano())
	responses := make(chan *tccore.ChatMsg, 1)
	GetChatMsgHooks().Set(id, func(msg *tccore.ChatMsg) bool {
		if msg == nil || msg.RoutingId == nil || *msg.RoutingId != id {
			return false
		}
		select {
		case responses <- msg:
		default:
		}
		return msg.HookResponse != nil || msg.Response == nil || *msg.Response != "Service initializing"
	})
	defer GetChatMsgHooks().Remove(id)
	pluginName, command := "trcsh", "talkctl"
	request := &tccore.ChatMsg{
		Name:         &pluginName,
		Query:        &[]string{"talkctl"},
		ChatId:       &command,
		RoutingId:    &id,
		HookResponse: append([]string(nil), args...),
	}
	timer := time.NewTimer(2 * time.Minute)
	defer timer.Stop()
	select {
	case *m.chatSenderChan <- request:
	case <-timer.C:
		return fail("Talkctl request timed out.")
	}
	for {
		select {
		case response := <-responses:
			if response.HookResponse != nil {
				model, ok := response.HookResponse.(tea.Model)
				if !ok {
					return fail("Talkctl returned an unsupported command UI.")
				}
				return editorReadyMsg{model: model}
			}
			if response.Response == nil {
				return fail("")
			}
			if *response.Response != "Service initializing" {
				return commandResultMsg{output: strings.Split(strings.TrimSpace(*response.Response), "\n")}
			}
		case <-timer.C:
			return fail("Talkctl request timed out.")
		}
	}
}

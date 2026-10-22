package shell

import (
	"fmt"
	"strings"
	"time"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
)

func (m *ShellModel) executeTalkctlCommand(args []string) []string {
	if !m.elevatedMode {
		return []string{errorStyle.Render("Error: '@talkctl' command requires elevated access"), "Run 'su' to obtain elevated access"}
	}
	if m.chatSenderChan == nil || *m.chatSenderChan == nil {
		return []string{errorStyle.Render("Error: chat channel not available")}
	}
	if len(args) == 0 || strings.TrimSpace(strings.Join(args, " ")) == "" {
		return []string{"Usage: @talkctl <command...>"}
	}
	id := fmt.Sprintf("talkctl-%d", time.Now().UnixNano())
	responses := make(chan string, 1)
	GetChatMsgHooks().Set(id, func(msg *tccore.ChatMsg) bool {
		if msg == nil || msg.RoutingId == nil || *msg.RoutingId != id {
			return false
		}
		response := ""
		if msg.Response != nil {
			response = *msg.Response
		}
		select {
		case responses <- response:
		default:
		}
		return true
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
		return []string{"Talkctl request timed out."}
	}
	select {
	case response := <-responses:
		return strings.Split(strings.TrimSpace(response), "\n")
	case <-timer.C:
		return []string{"Talkctl request timed out."}
	}
}

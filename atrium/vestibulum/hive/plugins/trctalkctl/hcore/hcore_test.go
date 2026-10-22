package hcore

import (
	"context"
	"errors"
	"io"
	"log"
	"strings"
	"testing"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
)

func TestInitRequiresTrcshZ(t *testing.T) {
	for _, properties := range []map[string]any{
		{"isKernelZ": false},
		{"isKernelZ": true, "isKubernetes": true},
	} {
		properties["log"] = log.New(io.Discard, "", 0)
		Init("talkctl", &properties)
		if refused, _ := properties["pluginRefused"].(bool); !refused {
			t.Fatal("plugin accepted a non-TrcshZ kernel")
		}
	}
}

func commandRequest(id string) *tccore.ChatMsg {
	name, command := "talkctl", "talkctl"
	return &tccore.ChatMsg{
		Name:         &name,
		Query:        &[]string{"trcsh"},
		ChatId:       &command,
		RoutingId:    &id,
		HookResponse: []string{"inspect", "worker", "tenant one"},
	}
}

func TestOnlyShellCommandsAreAccepted(t *testing.T) {
	for _, alter := range []func(*tccore.ChatMsg){
		func(event *tccore.ChatMsg) { event.Query = &[]string{"other"} },
		func(event *tccore.ChatMsg) { event.Query = nil },
		func(event *tccore.ChatMsg) { event.Name = nil },
		func(event *tccore.ChatMsg) { event.ChatId = nil },
		func(event *tccore.ChatMsg) { event.RoutingId = nil },
		func(event *tccore.ChatMsg) { event.IsBroadcast = true },
		func(event *tccore.ChatMsg) { response := "reply"; event.Response = &response },
	} {
		request := commandRequest("shell-1")
		alter(request)
		if shellCommand(request) {
			t.Fatalf("accepted a non-command message: %#v", request)
		}
	}
	if shellCommand(nil) || !shellCommand(commandRequest("shell-1")) {
		t.Fatal("shell envelope validation failed")
	}
}

func TestCommandReturnsEveryResponseLineToShell(t *testing.T) {
	replies := make(chan *tccore.ChatMsg, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	plugin := &commandPlugin{
		ctx: ctx, cancel: cancel,
		config: &tccore.ConfigContext{ChatSenderChan: &replies},
		send: func(ctx context.Context, id string, prompt string) (string, error) {
			if id != "shell-1" || prompt != "inspect worker tenant one" {
				t.Fatalf("incorrect forwarded command: %q %q", id, prompt)
			}
			if _, bounded := ctx.Deadline(); !bounded {
				t.Fatal("command has no deadline")
			}
			return "first\nsecond", nil
		},
	}
	plugin.handleCommand(commandRequest("shell-1"))
	reply := <-replies
	if reply.RoutingId == nil || *reply.RoutingId != "shell-1" || reply.Response == nil || *reply.Response != "first\nsecond" || reply.Query == nil || len(*reply.Query) != 1 || (*reply.Query)[0] != "trcsh" {
		t.Fatalf("incorrect shell reply: %#v", reply)
	}
}

func TestCommandReturnsTransportErrors(t *testing.T) {
	replies := make(chan *tccore.ChatMsg, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	plugin := &commandPlugin{
		ctx: ctx, cancel: cancel,
		config: &tccore.ConfigContext{ChatSenderChan: &replies},
		send: func(context.Context, string, string) (string, error) {
			return "", errors.New("hub unavailable")
		},
	}
	plugin.handleCommand(commandRequest("shell-2"))
	reply := <-replies
	if reply.Response == nil || !strings.Contains(*reply.Response, "hub unavailable") || *reply.RoutingId != "shell-2" {
		t.Fatalf("transport error was not returned: %#v", reply)
	}
}

package hcore

import (
	"context"
	"errors"
	"strings"
	"testing"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
)

type testCommandUI struct {
	model   any
	handled bool
	err     error
	calls   int
}

func (ui *testCommandUI) Open(_ context.Context, _ *tccore.ConfigContext, _ []string) (any, bool, error) {
	ui.calls++
	return ui.model, ui.handled, ui.err
}

func (*testCommandUI) Receive(*tccore.ChatMsg) {}

func TestCommandExtensionCanReturnUIOrUseHub(t *testing.T) {
	for _, tt := range []struct {
		name    string
		ui      *testCommandUI
		wantHub bool
		wantErr string
	}{
		{name: "UI", ui: &testCommandUI{model: &struct{}{}, handled: true}},
		{name: "ordinary command", ui: &testCommandUI{}, wantHub: true},
		{name: "UI error", ui: &testCommandUI{handled: true, err: errors.New("configuration unavailable")}, wantErr: "configuration unavailable"},
		{name: "missing model", ui: &testCommandUI{handled: true}, wantErr: "returned no model"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			replies := make(chan *tccore.ChatMsg, 1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calledHub := false
			plugin := &commandPlugin{
				ctx: ctx, cancel: cancel, ui: tt.ui,
				config: &tccore.ConfigContext{ChatSenderChan: &replies},
				send: func(context.Context, string, string) (string, error) {
					calledHub = true
					return "hub response", nil
				},
			}
			plugin.handleCommand(commandRequest("ui-1"))
			reply := <-replies
			if calledHub != tt.wantHub || tt.ui.calls != 1 {
				t.Fatalf("hub called=%t UI calls=%d", calledHub, tt.ui.calls)
			}
			if reply.RoutingId == nil || *reply.RoutingId != "ui-1" {
				t.Fatal("UI response lost shell correlation")
			}
			if tt.wantErr != "" {
				if reply.Response == nil || !strings.Contains(*reply.Response, tt.wantErr) || reply.HookResponse != nil {
					t.Fatalf("UI error not returned to shell: %#v", reply)
				}
			} else if tt.wantHub {
				if reply.Response == nil || *reply.Response != "hub response" {
					t.Fatalf("ordinary command did not use hub: %#v", reply)
				}
			} else if reply.HookResponse != tt.ui.model || reply.Response != nil {
				t.Fatalf("UI model not returned to shell: %#v", reply)
			}
		})
	}
}

func TestCommandUIRequiresShellEnvelope(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ui := &testCommandUI{handled: true, model: &struct{}{}}
	plugin := &commandPlugin{ctx: ctx, cancel: cancel, ui: ui}
	request := commandRequest("ui-2")
	request.Query = &[]string{"other"}
	plugin.handleCommand(request)
	if ui.calls != 0 {
		t.Fatal("non-shell request opened an extension UI")
	}
}

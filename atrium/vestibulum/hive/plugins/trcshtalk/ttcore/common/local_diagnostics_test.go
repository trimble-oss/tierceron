package common

import (
	"context"
	"testing"

	"github.com/trimble-oss/tierceron-core/v2/core/pluginsync"
)

func TestLocalDiagnosticTargetsFollowKernelRegistry(t *testing.T) {
	SetLocalDiagnosticHandler(func(context.Context, string, string) (string, error) { return "", nil })
	t.Cleanup(func() { SetLocalDiagnosticHandler(nil) })
	target := t.Name() + "-dynamic-plugin"
	if LocalDiagnosticHandler([]string{target}) != nil {
		t.Fatal("unregistered plugin considered local")
	}
	pluginsync.CreatePluginReadyChannel(target)
	if LocalDiagnosticHandler([]string{target}) == nil {
		t.Fatal("newly registered plugin not discovered")
	}
	if LocalDiagnosticHandler(nil) != nil || LocalDiagnosticHandler([]string{target, "unregistered-" + t.Name()}) != nil {
		t.Fatal("empty or partially local target set considered executable locally")
	}
	SetLocalDiagnosticHandler(nil)
	if LocalDiagnosticHandler([]string{target}) != nil {
		t.Fatal("missing transport adapter considered executable")
	}
}

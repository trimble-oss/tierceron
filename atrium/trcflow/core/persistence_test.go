package core

import (
	"testing"

	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
)

func TestWithFlowPersistenceRejectsUnavailableStorage(t *testing.T) {
	machine := &TrcFlowMachineContext{
		FlowMap: map[flowcore.FlowNameType]*TrcFlowContext{
			"ExampleFlow": {},
		},
	}
	callback := func(flowcore.FlowPersistenceContext) error {
		t.Fatal("callback invoked without initialized storage")
		return nil
	}
	for _, flow := range []flowcore.FlowNameType{"", "MissingFlow", "ExampleFlow"} {
		if err := machine.WithFlowPersistence(flow, false, callback); err == nil {
			t.Fatalf("unavailable persistence accepted for %q", flow)
		}
	}
	if err := machine.WithFlowPersistence("ExampleFlow", false, nil); err == nil {
		t.Fatal("nil callback accepted")
	}
	var unavailable *TrcFlowMachineContext
	if err := unavailable.WithFlowPersistence("ExampleFlow", false, callback); err == nil {
		t.Fatal("nil machine accepted")
	}
}

func TestFlowPersistenceRejectsUnauthorizedWrites(t *testing.T) {
	for _, remoteAllowed := range []bool{false, true} {
		context := &flowPersistenceContext{
			flow: &TrcFlowContext{ReadOnly: !remoteAllowed}, writeRemote: false,
		}
		if err := context.WriteRecord("example/record", map[string]any{}); err == nil {
			t.Fatal("remote write accepted without permission")
		}
	}
	context := &flowPersistenceContext{flow: &TrcFlowContext{ReadOnly: true}, writeRemote: true}
	if err := context.WriteRecord("example/record", map[string]any{}); err == nil {
		t.Fatal("remote write accepted for read-only flow")
	}
}

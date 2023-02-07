//go:build !tc
// +build !tc

package flowopts

import (
	"errors"

	flowcore "github.com/trimble-oss/tierceron/trcflow/core"
	trcf "github.com/trimble-oss/tierceron/trcflow/core/flowcorehelper"
)

// Flow names
func GetAdditionalFlows() []flowcore.FlowNameType {
	return []flowcore.FlowNameType{}
}

func GetAdditionalTestFlows() []flowcore.FlowNameType {
	return []flowcore.FlowNameType{} // Noop
}

func GetAdditionalFlowsByState(teststate string) []flowcore.FlowNameType {
	return []flowcore.FlowNameType{}
}

// Process a test flow.
func ProcessTestFlowController(tfmContext *flowcore.TrcFlowMachineContext, trcFlowContext *flowcore.TrcFlowContext) error {
	return errors.New("Table not implemented.")
}

func ProcessFlowController(tfmContext *flowcore.TrcFlowMachineContext, trcFlowContext *flowcore.TrcFlowContext) error {
	return nil
}

func GetFlowDatabaseName() string {
	return trcf.GetFlowDBName()
}

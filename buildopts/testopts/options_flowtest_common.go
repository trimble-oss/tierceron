//go:build !testflow
// +build !testflow

package testopts

import (
	flowcore "github.com/trimble-oss/tierceron/trcflow/core"
)

func GetAdditionalTestFlows() []flowcore.FlowNameType {
	return []flowcore.FlowNameType{}
}

func GetAdditionalFlowsByState(teststate string) []flowcore.FlowNameType {
	return []flowcore.FlowNameType{}
}

func ProcessTestFlowController(tfmContext *flowcore.TrcFlowMachineContext, trcFlowContext *flowcore.TrcFlowContext) error {
	return nil
}

//go:build testflow
// +build testflow

package testopts

import (
	flowcore "github.com/trimble-oss/tierceron/trcflow/core"

	testtcutil "VaultConfig.Test/util"
)

func GetAdditionalTestFlows() []flowcore.FlowNameType {
	return testtcutil.GetAdditionalFlows()
}

func GetAdditionalFlowsByState(teststate string) []flowcore.FlowNameType {
	return testtcutil.GetAdditionalFlowsByState(teststate)
}

func ProcessTestFlowController(tfmContext *flowcore.TrcFlowMachineContext, trcFlowContext *flowcore.TrcFlowContext) error {
	return testtcutil.ProcessTestFlowController(tfmContext, trcFlowContext)
}

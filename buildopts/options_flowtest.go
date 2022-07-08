//go:build testflow
// +build testflow

package buildopts

import (
	flowcore "tierceron/trcflow/core"

	tcutil "VaultConfig.TenantConfig/util"
	testtcutil "VaultConfig.Test/util"
)

func GetAdditionalTestFlows() []flowcore.FlowNameType {
	return testtcutil.GetAdditionalFlows()
}

func GetAdditionalFlowsByState(teststate string) []flowcore.FlowNameType {
	return testtcutil.GetAdditionalFlowsByState(teststate)
}

func ProcessTestFlowController(tfmContext *flowcore.TrcFlowMachineContext, trcFlowContext *flowcore.TrcFlowContext) error {
	return tcutil.ProcessTestFlowController(tfmContext, trcflowContext)
}

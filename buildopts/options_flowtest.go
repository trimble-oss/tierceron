//go:build testflow
// +build testflow

package buildopts

import (
	flowcore "tierceron/trcflow/core"

	testtcutil "VaultConfig.Test/util"
)

func GetAdditionalFlows() []flowcore.FlowNameType {
	return testtcutil.GetAdditionalFlows()
}

func GetAdditionalFlowsByState(teststate string) []flowcore.FlowNameType {
	return testtcutil.GetAdditionalFlowsByState()
}

func ProcessTestFlowController(tfmContext *flowcore.TrcFlowMachineContext, trcFlowContext *flowcore.TrcFlowContext) error {
	return tcutil.ProcessTestFlowController(tfmContext, trcflowContext)
}

func GetTestConfig(token string, wantPluginPaths bool) map[string]interface{} {
	return tcutil.GetTestConfig(token, wantPluginPaths)
}

//go:build tc
// +build tc

package flowopts

import (
	tcutil "VaultConfig.TenantConfig/util"
	flowcore "tierceron/trcflow/core"
)

func GetAdditionalFlows() []flowcore.FlowNameType {
	return tcutil.GetAdditionalFlows()
}

func GetAdditionalTestFlows() []flowcore.FlowNameType {
	return []flowcore.FlowNameType{} // Noop
}

func GetAdditionalFlowsByState(teststate string) []flowcore.FlowNameType {
	return []flowcore.FlowNameType{} // Noop
}

func ProcessFlowController(tfmContext *flowcore.TrcFlowMachineContext, trcFlowContext *flowcore.TrcFlowContext) error {
	return tcutil.ProcessFlowController(tfmContext, trcFlowContext)
}

func ProcessTestFlowController(tfmContext *flowcore.TrcFlowMachineContext, trcFlowContext *flowcore.TrcFlowContext) error {
	return tcutil.ProcessFlowController(tfmContext, trcFlowContext)
}

func GetFlowDatabaseName() string {
	return "FlumDatabase"
}

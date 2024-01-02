package testopts

import (
	flowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"
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

func GetTestConfig(token string, wantPluginPaths bool) map[string]interface{} {
	return nil
}

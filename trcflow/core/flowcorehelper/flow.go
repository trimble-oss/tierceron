package flowcorehelper

//import "tierceron/buildopts/flowopts"

const TierceronFlowConfigurationTableName = "TierceronFlow"
const TierceronFlowDB = "FlumeDatabase"

type FlowStateUpdate struct {
	FlowName    string
	StateUpdate string
}

func GetFlowDBName() string {
	return TierceronFlowDB
}

func UpdateTierceronFlowState(flowName string, newState string) string {
	return "update " + TierceronFlowDB + "." + TierceronFlowConfigurationTableName + " set state=" + newState + " where flowName='" + flowName + "'"
}

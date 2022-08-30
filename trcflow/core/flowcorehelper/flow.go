package flowcorehelper

//import "tierceron/buildopts/flowopts"

const TierceronFlowConfigurationTableName = "TierceronFlow"
const TierceronFlowDB = "FlumeDatabase"

type FlowStateUpdate struct {
	FlowName    string
	StateUpdate string
	SyncFilter  string
}

type CurrentFlowState struct {
	State      int64
	SyncMode   string
	SyncFilter string
}

func GetFlowDBName() string {
	return TierceronFlowDB
}

func UpdateTierceronFlowState(flowName string, newState string, syncFilter string) string {
	return "update " + TierceronFlowDB + "." + TierceronFlowConfigurationTableName + " set lastModified=current_timestamp(), state=" + newState + ", SyncFilter='" + syncFilter + "' where flowName='" + flowName + "'"
}

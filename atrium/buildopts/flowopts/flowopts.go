package flowopts

import (
	"encoding/json"
	"errors"
	"log"

	flowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"
	flowcorehelper "github.com/trimble-oss/tierceron/atrium/trcflow/core/flowcorehelper"
)

// GetAdditionalFlows - override to provide a list of additional business logic based flows.
// These business logic flows have direct access to other flow data via the internal
// sql query engine, the ability to call other flows, and the ability to directly call
// the secret provider for sensitive secrets to access services and features as needed.
func GetAdditionalFlows() []flowcore.FlowNameType {
	return []flowcore.FlowNameType{}
}

// GetAdditionalTestFlows - override to provide a list of additional test flows.  These
// test flows are used to test the flow machine.
func GetAdditionalTestFlows() []flowcore.FlowNameType {
	return []flowcore.FlowNameType{} // Noop
}

// GetAdditionalFlowsByState - override to provide a list of flows given a test state.
// This list of flows will be notified when a given test state is reached.
func GetAdditionalFlowsByState(teststate string) []flowcore.FlowNameType {
	return []flowcore.FlowNameType{}
}

// Process a test flow.
func ProcessTestFlowController(tfmContext *flowcore.TrcFlowMachineContext, trcFlowContext *flowcore.TrcFlowContext) error {
	return errors.New("Flow not implemented.")
}

// ProcessFlowController - override to provide a custom flow controller.  You will need a custom
// flow controller if you define any additional flows other than the default flows:
// 1. DataFlowStatConfigurationsFlow
// 2. AskFlumeFlow
func ProcessFlowController(tfmContext *flowcore.TrcFlowMachineContext, trcFlowContext *flowcore.TrcFlowContext) error {
	return nil
}

// GetFlowDatabaseName - override to provide a custom flow database name.
// The default flow database name is FlumeDatabase
func GetFlowDatabaseName() string {
	return flowcorehelper.GetFlowDBName()
}

// Placeholder
type AskFlumeResponse struct {
	Message string
	Type    string
}

// ProcessAskFlumeEventMapper - override to provide a custom AskFlumeEventMapper processor.
// This processor is used to map AskFlumeMessage events to a custom query.
func ProcessAskFlumeEventMapper(askFlumeContext *flowcore.AskFlumeContext, query *flowcore.AskFlumeMessage, tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) *flowcore.AskFlumeMessage {
	var msg *flowcore.AskFlumeMessage

	sql_query := make(map[string]interface{})

	switch {
	case query.Message == "DataFlowState":
		sql_query["TrcQuery"] = "select distinct argosId, flowName, stateCode, stateName, lastTestedDate from DataflowStatistics where stateName like '%Failed%'"
	default:
		return nil
	}

	// Not enough time to check how to implement above queries, so only runs the query below
	for {
		rows := tfmContext.CallDBQuery(tfContext, sql_query, nil, false, "SELECT", nil, "")
		encoded_rows, err := json.Marshal(rows)
		if err != nil {
			log.Printf("error encoding result from trcdb: %v", err)
		}
		if len(rows) > 0 {
			msg = &flowcore.AskFlumeMessage{
				Id:      query.Id,
				Type:    query.Message,
				Message: string(encoded_rows),
			}
			break
		} else {
			msg = &flowcore.AskFlumeMessage{
				Id:      query.Id,
				Type:    "No results",
				Message: string(encoded_rows),
			}
			break

		}
	}

	return msg
}

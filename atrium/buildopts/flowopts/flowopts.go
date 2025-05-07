package flowopts

import (
	"encoding/json"
	"errors"
	"log"

	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	trcflowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"
	flowcorehelper "github.com/trimble-oss/tierceron/atrium/trcflow/core/flowcorehelper"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcflow/flows"
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
func ProcessTestFlowController(tfmContext flowcore.FlowMachineContext, trcFlowContext flowcore.FlowContext) error {
	return errors.New("flow not implemented")
}

// ProcessFlowController - override to provide a custom flow controller.  You will need a custom
// flow controller if you define any additional flows other than the default flows:
// 1. DataFlowStatConfigurationsFlow
func ProcessFlowController(tfmContext flowcore.FlowMachineContext, tfContext flowcore.FlowContext) error {
	trcFlowContext := tfContext.(*trcflowcore.TrcFlowContext)
	switch trcFlowContext.Flow {
	case trcflowcore.DataFlowStatConfigurationsFlow:
		return flows.ProcessDataFlowStatConfigurations(tfmContext, tfContext)
	}
	return errors.New("flow not implemented")
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
func ProcessAskFlumeEventMapper(askFlumeContext *trcflowcore.AskFlumeContext, query *trcflowcore.AskFlumeMessage, tfmContext *trcflowcore.TrcFlowMachineContext, tfContext *trcflowcore.TrcFlowContext) *trcflowcore.AskFlumeMessage {
	var msg *trcflowcore.AskFlumeMessage

	sql_query := make(map[string]interface{})

	switch {
	case query.Message == "DataFlowState":
		sql_query["TrcQuery"] = "select distinct argosId, flowName, stateCode, stateName, lastTestedDate from DataflowStatistics where stateName like '%Failed%'"
	default:
		return nil
	}

	// Not enough time to check how to implement above queries, so only runs the query below
	for {
		rows, _ := tfmContext.CallDBQuery(tfContext, sql_query, nil, false, "SELECT", nil, "")
		encoded_rows, err := json.Marshal(rows)
		if err != nil {
			log.Printf("error encoding result from trcdb: %v", err)
		}
		if len(rows) > 0 {
			msg = &trcflowcore.AskFlumeMessage{
				Id:      query.Id,
				Type:    query.Message,
				Message: string(encoded_rows),
			}
			break
		} else {
			msg = &trcflowcore.AskFlumeMessage{
				Id:      query.Id,
				Type:    "No results",
				Message: string(encoded_rows),
			}
			break

		}
	}

	return msg
}

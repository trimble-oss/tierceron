package flowopts

import (
	"encoding/json"
	"errors"
	"log"

	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	trcflowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcflow/flows/argossocii"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcflow/flows/dataflowstatistics"
)

// AllowTrcdbInterfaceOverride - by default trcdb plugins cannot expose
// a mariadb interface.  They can always create an internal database but
// only a trcsh kernel compiled to allow custom building of this interface
// will actually create an interface using configurations provided by the plugin.
func AllowTrcdbInterfaceOverride() bool {
	return false
}

// GetAdditionalFlows - override to provide a list of additional business logic based flows.
// These business logic flows have direct access to other flow data via the internal
// sql query engine, the ability to call other flows, and the ability to directly call
// the secret provider for sensitive secrets to access services and features as needed.
func GetAdditionalFlows() []flowcore.FlowDefinition {
	return []flowcore.FlowDefinition{}
}

// GetAdditionalTestFlows - override to provide a list of additional test flows.  These
// test flows are used to test the flow machine.
func GetAdditionalTestFlows() []flowcore.FlowDefinition {
	return []flowcore.FlowDefinition{} // Noop
}

// GetAdditionalFlowsByState - override to provide a list of flows given a test state.
// This list of flows will be notified when a given test state is reached.
func GetAdditionalFlowsByState(teststate string) []flowcore.FlowDefinition {
	return []flowcore.FlowDefinition{}
}

// ProcessTestFlowController - Process a test flow.
func ProcessTestFlowController(tfmContext flowcore.FlowMachineContext, trcFlowContext flowcore.FlowContext) error {
	return errors.New("flow not implemented")
}

// ProcessFlowController - override to provide a custom flow controller.  You will need a custom
// flow controller if you define any additional flows other than the default flows:
// 1. DataFlowStatConfigurationsFlow
func ProcessFlowController(tfmContext flowcore.FlowMachineContext, tfContext flowcore.FlowContext) error {
	trcFlowContext := tfContext.(*trcflowcore.TrcFlowContext)
	switch trcFlowContext.FlowHeader.TableName() {
	case flowcore.DataFlowStatConfigurationsFlow.TableName():
		return dataflowstatistics.ProcessDataFlowStatConfigurations(tfmContext, tfContext)
	case flowcore.ArgosSociiFlow.TableName():
		tfContext.SetFlowLibraryContext(argossocii.GetProcessFlowDefinition())
		return flowcore.ProcessTableConfigurations(tfmContext, tfContext)
	}
	return errors.New("flow not implemented")
}

func GetFlowMachineTemplates() map[string]any {
	pluginConfig := map[string]any{}
	pluginConfig["templatePath"] = []string{
		"trc_templates/TenantDatabase/DataFlowStatistics/DataFlowStatistics.tmpl", // implemented.
		"trc_templates/TrcDb/ArgosSocii/ArgosSocii.tmpl",                          // implemented.
	}
	pluginConfig["flumeTemplatePath"] = []string{
		"trc_templates/FlumeDatabase/TierceronFlow/TierceronFlow.tmpl", // implemented.
	}

	return pluginConfig
}

// AskFlumeResponse - response structure for AskFlume
type AskFlumeResponse struct {
	Message string
	Type    string
}

// ProcessAskFlumeEventMapper - override to provide a custom AskFlumeEventMapper processor.
// This processor is used to map AskFlumeMessage events to a custom query.
func ProcessAskFlumeEventMapper(askFlumeContext *trcflowcore.AskFlumeContext, query *trcflowcore.AskFlumeMessage, tfmContext *trcflowcore.TrcFlowMachineContext, tfContext *trcflowcore.TrcFlowContext) *trcflowcore.AskFlumeMessage {
	var msg *trcflowcore.AskFlumeMessage

	sqlQuery := make(map[string]any)

	switch {
	case query.Message == "DataFlowState":
		sqlQuery["TrcQuery"] = "select distinct argosId, flowName, stateCode, stateName, lastTestedDate from DataflowStatistics where stateName like '%Failed%'"
	default:
		return nil
	}

	// Not enough time to check how to implement above queries, so only runs the query below
	for {
		rows, _ := tfmContext.CallDBQuery(tfContext, sqlQuery, nil, false, "SELECT", nil, "")
		encodedRows, err := json.Marshal(rows)
		if err != nil {
			log.Printf("error encoding result from trcdb: %v", err)
		}
		if len(rows) > 0 {
			msg = &trcflowcore.AskFlumeMessage{
				ID:      query.ID,
				Type:    query.Message,
				Message: string(encodedRows),
			}
			break
		} else {
			msg = &trcflowcore.AskFlumeMessage{
				ID:      query.ID,
				Type:    "No results",
				Message: string(encodedRows),
			}
			break

		}
	}

	return msg
}

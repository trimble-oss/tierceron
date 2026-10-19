package core

import (
	"fmt"

	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	trcdb "github.com/trimble-oss/tierceron/atrium/trcdb"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

var _ flowcore.FlowPersistenceMachine = (*TrcFlowMachineContext)(nil)

type flowPersistenceContext struct {
	machine     *TrcFlowMachineContext
	flow        *TrcFlowContext
	modifier    helperkv.Modifier
	database    string
	writeRemote bool
}

func (tfmContext *TrcFlowMachineContext) WithFlowPersistence(flowName flowcore.FlowNameType, writeRemote bool, action func(flowcore.FlowPersistenceContext) error) error {
	if tfmContext == nil || flowName == "" || action == nil {
		return fmt.Errorf("flow persistence requires a machine, flow name, and callback")
	}
	flow, ok := tfmContext.GetFlowContext(flowName).(*TrcFlowContext)
	if !ok || flow == nil {
		return fmt.Errorf("flow persistence is unavailable for %s", flowName)
	}
	flow.GetPullOnceMu().Lock()
	defer flow.GetPullOnceMu().Unlock()
	flow.GetPersistenceMu().Lock()
	defer flow.GetPersistenceMu().Unlock()

	if flow.GoMod == nil || flow.FlowHeader == nil || flow.QueryLock == nil ||
		tfmContext.TierceronEngine == nil || tfmContext.TierceronEngine.Engine == nil ||
		tfmContext.TierceronEngine.Database == nil {
		return fmt.Errorf("flow persistence is not initialized for %s", flowName)
	}
	if flow.ReadOnly && writeRemote {
		return fmt.Errorf("remote persistence is read-only for %s", flowName)
	}
	modifier := *flow.GoMod
	modifier.SectionPath = ""
	modifier.Version = "0"
	database := flow.FlowHeader.SourceAlias
	if database == "" {
		database = tfmContext.TierceronEngine.Database.Name()
	}
	return action(&flowPersistenceContext{
		machine: tfmContext, flow: flow, modifier: modifier,
		database: database, writeRemote: writeRemote,
	})
}

func (context *flowPersistenceContext) Source() string {
	return context.flow.FlowHeader.Source
}

func (context *flowPersistenceContext) DatabaseName() string {
	return context.database
}

func (context *flowPersistenceContext) Query(query string) ([][]any, error) {
	if query == "" {
		return nil, fmt.Errorf("flow persistence query must not be empty")
	}
	_, _, rows, err := trcdb.Query(context.machine.TierceronEngine, query, context.flow.QueryLock)
	return rows, err
}

func (context *flowPersistenceContext) ReadRecord(path string) (map[string]any, error) {
	if path == "" {
		return nil, fmt.Errorf("persistence record path must not be empty")
	}
	return context.modifier.ReadData(path)
}

func (context *flowPersistenceContext) WriteRecord(path string, data map[string]any) error {
	if !context.writeRemote || context.flow.ReadOnly {
		return fmt.Errorf("remote persistence is read-only")
	}
	if path == "" {
		return fmt.Errorf("persistence record path must not be empty")
	}
	_, err := context.modifier.Write(path, data, context.flow.Logger)
	return err
}

package core

import (
	"context"
	"log"
	"strconv"
	"strings"
	"sync"

	tcflow "github.com/trimble-oss/tierceron-core/v2/flow"
	"github.com/trimble-oss/tierceron/atrium/trcflow/core/flowcorehelper"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	sys "github.com/trimble-oss/tierceron/pkg/vaulthelper/system"
)

type TrcFlowContext struct {
	RemoteDataSource map[string]interface{}
	GoMod            *helperkv.Modifier
	Vault            *sys.Vault

	// Recommended not to store contexts, but because we
	// are working with flows, this is a different pattern.
	// This just means some analytic tools won't be able to
	// perform analysis which are based on the Context.
	ContextNotifyChan chan bool
	Context           context.Context
	CancelContext     context.CancelFunc
	// I flow is complex enough, it can define
	// it's own method for loading TrcDb
	// from vault.
	CustomSeedTrcDb func(*TrcFlowMachineContext, *TrcFlowContext) error

	FlowSource        string       // The name of the flow source identified by project.
	FlowSourceAlias   string       // May be a database name
	Flow              FlowNameType // May be a table name.
	ChangeIdKey       string
	FlowPath          string
	FlowData          interface{}
	ChangeFlowName    string // Change flow table name.
	FlowState         flowcorehelper.CurrentFlowState
	PreviousFlowState flowcorehelper.CurrentFlowState // Temporary storage for previous state
	FlowLock          *sync.Mutex                     //This is for sync concurrent changes to FlowState
	Restart           bool
	Init              bool
	ReadOnly          bool
	DataFlowStatistic FakeDFStat
	Log               *log.Logger
}

var _ tcflow.FlowContext = (*TrcFlowContext)(nil)

func (tfContext *TrcFlowContext) IsInit() bool {
	return tfContext.Init
}

func (tfContext *TrcFlowContext) SetInit(init bool) {
	tfContext.Init = init
}

func (tfContext *TrcFlowContext) IsRestart() bool {
	return tfContext.Restart
}

func (tfContext *TrcFlowContext) SetRestart(restart bool) {
	tfContext.Restart = restart
}

func (tfContext *TrcFlowContext) FlowLocker() {
	if tfContext.FlowLock != nil {
		tfContext.FlowLock.Lock()
	}
}

func (tfContext *TrcFlowContext) FlowUnlocker() {
	if tfContext.FlowLock != nil {
		tfContext.FlowLock.Unlock()
	}
}

func (tfContext *TrcFlowContext) FlowSyncModeMatchAny(syncModes []string) bool {

	if tfContext.FlowState.SyncMode == "" {
		return false
	}
	for _, syncMode := range syncModes {
		if tfContext.FlowState.SyncMode == syncMode {
			return true
		}
	}
	return false
}

func (tfContext *TrcFlowContext) FlowSyncModeMatch(syncMode string, startsWith bool) bool {
	if tfContext.FlowState.SyncMode == "" {
		return false
	}
	if startsWith {
		if len(syncMode) > 0 && len(tfContext.FlowState.SyncMode) > 0 {
			if syncMode == tfContext.FlowState.SyncMode[0:len(syncMode)] {
				return true
			} else {
				return false
			}
		}
	} else {
		if syncMode == tfContext.FlowState.SyncMode {
			return true
		} else {
			return false
		}
	}
	return false
}
func (tfContext *TrcFlowContext) GetFlowSyncMode() string {
	return tfContext.FlowState.SyncMode
}

func (tfContext *TrcFlowContext) SetFlowSyncMode(syncMode string) {
	tfContext.FlowState.SyncMode = syncMode
}

func (tfContext *TrcFlowContext) GetFlowSourceAlias() string {
	return tfContext.FlowState.FlowAlias
}

func (tfContext *TrcFlowContext) SetFlowSourceAlias(flowSourceAlias string) {
	tfContext.FlowState.FlowAlias = flowSourceAlias
}

func (tfContext *TrcFlowContext) SetChangeFlowName(changeFlowName string) {
	tfContext.ChangeFlowName = changeFlowName
}

func (tfContext *TrcFlowContext) GetFlowState() tcflow.CurrentFlowState {
	return tfContext.FlowState
}

func (tfContext *TrcFlowContext) SetFlowState(flowState tcflow.CurrentFlowState) {
	tfContext.FlowLocker()
	tfContext.FlowState = flowState.(flowcorehelper.CurrentFlowState)
	tfContext.FlowUnlocker()
}

func (tfContext *TrcFlowContext) GetPreviousFlowState() tcflow.CurrentFlowState {
	return tfContext.PreviousFlowState
}

func (tfContext *TrcFlowContext) SetPreviousFlowState(flowState tcflow.CurrentFlowState) {
	tfContext.FlowLocker()
	tfContext.PreviousFlowState = flowState.(flowcorehelper.CurrentFlowState)
	tfContext.FlowUnlocker()
}

func (tfContext *TrcFlowContext) GetFlowStateState() int64 {
	return tfContext.FlowState.State
}

func (tfContext *TrcFlowContext) SetFlowData(flowData tcflow.TemplateData) {
	tfContext.FlowData = flowData
}

func (tfContext *TrcFlowContext) HasFlowSyncFilters() bool {
	if strings.TrimSpace(tfContext.FlowState.SyncFilter) == "" || tfContext.FlowState.SyncFilter == "n/a" {
		return false
	}
	return true
}

func (tfContext *TrcFlowContext) GetFlowStateSyncFilterRaw() string {
	return tfContext.FlowState.SyncFilter
}
func (tfContext *TrcFlowContext) GetFlowSyncFilters() []string {
	if tfContext.FlowState.SyncFilter == "" {
		return nil
	}
	return strings.Split(strings.ReplaceAll(tfContext.FlowState.SyncFilter, " ", ""), ",")
	// I don't think we need to trim the spaces here.
	// for i := 0; i < len(syncFilters); i++ {
	// 	syncFilters[i] = strings.TrimSpace(syncFilters[i])
	// }
	//return syncFilters
}

func (tfContext *TrcFlowContext) GetFlowName() string {
	return tfContext.Flow.TableName()
}

func (tfContext *TrcFlowContext) NewFlowStateUpdate(state string, syncMode string) tcflow.FlowStateUpdate {
	return flowcorehelper.FlowStateUpdate{
		FlowName:    tfContext.GetFlowName(),
		StateUpdate: state,
		SyncFilter:  tfContext.FlowState.SyncFilter,
		SyncMode:    syncMode,
		FlowAlias:   tfContext.FlowState.FlowAlias,
	}
}

func (tfContext *TrcFlowContext) GetCurrentFlowStateUpdateByDataSource(dataSource string) chan tcflow.CurrentFlowState {
	if tfContext.RemoteDataSource == nil {
		return nil
	}
	if flowStateController, ok := tfContext.RemoteDataSource[dataSource]; ok {
		return flowStateController.(chan tcflow.CurrentFlowState)
	}
	return nil
}

func (tfContext *TrcFlowContext) UpdateFlowStateByDataSource(dataSource string) {
	if tfContext.RemoteDataSource == nil {
		return
	}
	if flowStateController, ok := tfContext.RemoteDataSource[dataSource]; ok {
		tfContext.FlowState = <-flowStateController.(chan flowcorehelper.CurrentFlowState)
	}
}

func (tfContext *TrcFlowContext) PushState(dataSource string, flowStateUpdate tcflow.FlowStateUpdate) {
	if tfContext.RemoteDataSource == nil {
		return
	}
	if flowStateReceiver, ok := tfContext.RemoteDataSource[dataSource]; ok {
		flowStateReceiver.(chan tcflow.FlowStateUpdate) <- flowStateUpdate
	}
}

func (tfContext *TrcFlowContext) GetUpdatePermission() tcflow.PermissionUpdate {
	return PermissionUpdate{
		TableName:    tfContext.GetFlowName(),
		CurrentState: tfContext.GetFlowStateState(),
	}
}

func (tfContext *TrcFlowContext) GetFlowUpdate(currentFlowState tcflow.CurrentFlowState) tcflow.FlowStateUpdate {
	cfs := currentFlowState.(flowcorehelper.CurrentFlowState)
	return flowcorehelper.FlowStateUpdate{
		FlowName:    tfContext.GetFlowName(),
		StateUpdate: strconv.Itoa(int(cfs.State)),
		SyncFilter:  cfs.SyncFilter,
		SyncMode:    cfs.SyncMode,
		FlowAlias:   cfs.FlowAlias,
	}
}

func (tfContext *TrcFlowContext) GetRemoteDataSourceAttribute(region string, dataSourceAttribute string) interface{} {
	if tfContext.RemoteDataSource == nil {
		return nil
	}
	if len(region) == 0 {
		region = "west"
	}
	if remoteDataSource, ok := tfContext.RemoteDataSource[region].(map[string]interface{}); ok {
		if remoteDataSourceAttribute, ok := remoteDataSource[dataSourceAttribute]; ok {
			return remoteDataSourceAttribute
		}
	}
	return nil
}

func (tfContext *TrcFlowContext) CancelTheContext() bool {
	if tfContext.CancelContext != nil {
		tfContext.CancelContext()
		return true
	} else {
		return false
	}
}

func (tfContext *TrcFlowContext) GetLogger() *log.Logger {
	return tfContext.Log
}

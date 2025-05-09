package core

import (
	"context"
	"log"
	"strconv"
	"strings"
	"sync"

	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	"github.com/trimble-oss/tierceron/atrium/trcflow/core/flowcorehelper"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	sys "github.com/trimble-oss/tierceron/pkg/vaulthelper/system"
)

type TrcFlowContext struct {
	DataSourceRegions []string
	RemoteDataSource  map[string]interface{}
	GoMod             *helperkv.Modifier
	Vault             *sys.Vault

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

	FlowSource            string                // The name of the flow source identified by project.
	FlowSourceAlias       string                // May be a database name
	Flow                  flowcore.FlowNameType // May be a table name.
	ChangeIdKey           string
	FlowPath              string
	FlowData              interface{}
	ChangeFlowName        string // Change flow table name.
	FlowState             flowcorehelper.CurrentFlowState
	FlowStateLock         *sync.RWMutex                   //This is for sync concurrent changes to FlowState
	PreviousFlowState     flowcorehelper.CurrentFlowState // Temporary storage for previous state
	PreviousFlowStateLock *sync.RWMutex
	QueryLock             *sync.Mutex
	Restart               bool
	Init                  bool
	ReadOnly              bool
	DataFlowStatistic     FakeDFStat
	Log                   *log.Logger
}

var _ flowcore.FlowContext = (*TrcFlowContext)(nil)

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

func (tfContext *TrcFlowContext) NotifyFlowComponentLoaded() {
	go func() {
		tfContext.ContextNotifyChan <- true
	}()
}

func (tfContext *TrcFlowContext) WaitFlowLoaded() {
	<-tfContext.ContextNotifyChan
	<-tfContext.ContextNotifyChan
}

func (tfContext *TrcFlowContext) FlowSyncModeMatchAny(syncModes []string) bool {
	tfContext.FlowStateLock.RLock()
	defer tfContext.FlowStateLock.RUnlock()

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
	tfContext.FlowStateLock.RLock()
	defer tfContext.FlowStateLock.RUnlock()

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
	tfContext.FlowStateLock.RLock()
	defer tfContext.FlowStateLock.RUnlock()
	return tfContext.FlowState.SyncMode
}

func (tfContext *TrcFlowContext) GetFlowStateAlias() string {
	tfContext.FlowStateLock.RLock()
	defer tfContext.FlowStateLock.RUnlock()
	return tfContext.FlowState.FlowAlias
}

func (tfContext *TrcFlowContext) SetFlowSyncMode(syncMode string) {
	tfContext.FlowStateLock.Lock()
	defer tfContext.FlowStateLock.Unlock()
	tfContext.FlowState.SyncMode = syncMode
}

func (tfContext *TrcFlowContext) GetFlowSourceAlias() string {
	tfContext.FlowStateLock.RLock()
	defer tfContext.FlowStateLock.RUnlock()
	return tfContext.FlowState.FlowAlias
}

func (tfContext *TrcFlowContext) SetFlowSourceAlias(flowSourceAlias string) {
	tfContext.FlowStateLock.Lock()
	defer tfContext.FlowStateLock.Unlock()
	tfContext.FlowState.FlowAlias = flowSourceAlias
}

func (tfContext *TrcFlowContext) SetChangeFlowName(changeFlowName string) {
	tfContext.ChangeFlowName = changeFlowName
}

func (tfContext *TrcFlowContext) GetFlowState() flowcore.CurrentFlowState {
	tfContext.FlowStateLock.RLock()
	defer tfContext.FlowStateLock.RUnlock()
	return tfContext.FlowState
}

func (tfContext *TrcFlowContext) SetFlowState(flowState flowcore.CurrentFlowState) {
	tfContext.FlowStateLock.Lock()
	tfContext.FlowState = flowState.(flowcorehelper.CurrentFlowState)
	tfContext.FlowStateLock.Unlock()
}

func (tfContext *TrcFlowContext) GetPreviousFlowState() flowcore.CurrentFlowState {
	tfContext.PreviousFlowStateLock.RLock()
	defer tfContext.PreviousFlowStateLock.RUnlock()
	return tfContext.PreviousFlowState
}

func (tfContext *TrcFlowContext) SetPreviousFlowState(flowState flowcore.CurrentFlowState) {
	tfContext.PreviousFlowStateLock.Lock()
	tfContext.PreviousFlowState = flowState.(flowcorehelper.CurrentFlowState)
	tfContext.PreviousFlowStateLock.Unlock()
}

func (tfContext *TrcFlowContext) GetFlowStateState() int64 {
	tfContext.FlowStateLock.RLock()
	defer tfContext.FlowStateLock.RUnlock()
	return tfContext.FlowState.State
}

func (tfContext *TrcFlowContext) SetFlowData(flowData flowcore.TemplateData) {
	tfContext.FlowData = flowData
}

func (tfContext *TrcFlowContext) HasFlowSyncFilters() bool {
	tfContext.FlowStateLock.RLock()
	defer tfContext.FlowStateLock.RUnlock()
	if strings.TrimSpace(tfContext.FlowState.SyncFilter) == "" || tfContext.FlowState.SyncFilter == "n/a" {
		return false
	}
	return true
}

func (tfContext *TrcFlowContext) GetFlowStateSyncFilterRaw() string {
	tfContext.FlowStateLock.RLock()
	defer tfContext.FlowStateLock.RUnlock()
	return tfContext.FlowState.SyncFilter
}
func (tfContext *TrcFlowContext) GetFlowSyncFilters() []string {
	tfContext.FlowStateLock.RLock()
	defer tfContext.FlowStateLock.RUnlock()
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

func (tfContext *TrcFlowContext) NewFlowStateUpdate(state string, syncMode string) flowcore.FlowStateUpdate {
	tfContext.FlowStateLock.RLock()
	defer tfContext.FlowStateLock.RUnlock()
	return flowcorehelper.FlowStateUpdate{
		FlowName:    tfContext.GetFlowName(),
		StateUpdate: state,
		SyncFilter:  tfContext.FlowState.SyncFilter,
		SyncMode:    syncMode,
		FlowAlias:   tfContext.FlowState.FlowAlias,
	}
}

func (tfContext *TrcFlowContext) GetCurrentFlowStateUpdateByDataSource(dataSource string) chan flowcore.CurrentFlowState {
	if tfContext.RemoteDataSource == nil {
		return nil
	}
	if flowStateController, ok := tfContext.RemoteDataSource[dataSource]; ok {
		return flowStateController.(chan flowcore.CurrentFlowState)
	}
	return nil
}

func (tfContext *TrcFlowContext) UpdateFlowStateByDataSource(dataSource string) {
	if tfContext.RemoteDataSource == nil {
		return
	}
	if flowStateController, ok := tfContext.RemoteDataSource[dataSource]; ok {
		tfContext.FlowStateLock.Lock()
		tfContext.FlowState = <-flowStateController.(chan flowcorehelper.CurrentFlowState)
		tfContext.FlowStateLock.Unlock()
	}
}

func (tfContext *TrcFlowContext) PushState(dataSource string, flowStateUpdate flowcore.FlowStateUpdate) {
	if tfContext.RemoteDataSource == nil {
		return
	}
	if flowStateReceiver, ok := tfContext.RemoteDataSource[dataSource]; ok {
		flowStateReceiver.(chan flowcore.FlowStateUpdate) <- flowStateUpdate
	}
}

func (tfContext *TrcFlowContext) GetUpdatePermission() flowcore.PermissionUpdate {
	return PermissionUpdate{
		TableName:    tfContext.GetFlowName(),
		CurrentState: tfContext.GetFlowStateState(),
	}
}

func (tfContext *TrcFlowContext) GetFlowUpdate(currentFlowState flowcore.CurrentFlowState) flowcore.FlowStateUpdate {
	cfs := currentFlowState.(flowcorehelper.CurrentFlowState)
	return flowcorehelper.FlowStateUpdate{
		FlowName:    tfContext.GetFlowName(),
		StateUpdate: strconv.Itoa(int(cfs.State)),
		SyncFilter:  cfs.SyncFilter,
		SyncMode:    cfs.SyncMode,
		FlowAlias:   cfs.FlowAlias,
	}
}

func (tfContext *TrcFlowContext) GetDataSourceRegions(filtered bool) []string {
	filterSyncRegions := []string{}
	if filtered {
		if tfContext.FlowSyncModeMatchAny([]string{"push", "pull"}) {
			syncMode := strings.TrimPrefix(strings.TrimPrefix(tfContext.GetFlowSyncMode(), "pull"), "push")
			if syncMode != "once" {
				if len(syncMode) > 0 {
					if strings.Contains(syncMode, ",") {
						filterSyncRegions = strings.Split(syncMode, ",")
					} else {
						filterSyncRegions = []string{syncMode}
					}
				}
			}
		}
	}
	if len(filterSyncRegions) > 0 {
		filteredRegions := []string{}
		for _, region := range tfContext.DataSourceRegions {
			for _, filterRegion := range filterSyncRegions {
				if strings.Contains(region, filterRegion) {
					filteredRegions = append(filteredRegions, region)
				}
			}
		}
		return filteredRegions
	} else {
		return tfContext.DataSourceRegions
	}
}

func (tfContext *TrcFlowContext) GetRemoteDataSourceAttribute(dataSourceAttribute string, regions ...string) interface{} {
	if tfContext.RemoteDataSource == nil {
		return nil
	}
	if len(regions) > 0 {
		if regionSource, ok := tfContext.RemoteDataSource[regions[0]].(map[string]interface{}); ok {
			if remoteDataSourceAttribute, ok := regionSource[dataSourceAttribute]; ok {
				return remoteDataSourceAttribute
			} else {
				return nil
			}
		}
	}

	if remoteDataSourceAttribute, ok := tfContext.RemoteDataSource[dataSourceAttribute]; ok {
		return remoteDataSourceAttribute
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

func (tfContext *TrcFlowContext) TransitionState(syncMode string) chan flowcore.CurrentFlowState {
	tfContext.UpdateFlowStateByDataSource("flowStateController")
	if syncMode != "" {
		tfContext.SetFlowSyncMode(syncMode)
	}
	stateUpdateChannel := tfContext.GetCurrentFlowStateUpdateByDataSource("flowStateReceiver")

	go func(tfCtx *TrcFlowContext, sPC chan flowcore.CurrentFlowState) {
		tfCtx.SetPreviousFlowState(tfCtx.GetFlowState()) //does get need locking...
		for {
			tfContext.PreviousFlowStateLock.RLock()
			previousState := tfContext.PreviousFlowState
			tfContext.PreviousFlowStateLock.RUnlock()
			select {
			case stateUpdateI := <-tfCtx.GetCurrentFlowStateUpdateByDataSource("flowStateController"):
				stateUpdate := stateUpdateI.(flowcorehelper.CurrentFlowState)
				if syncMode != "" {
					stateUpdate.SyncMode = syncMode
				}
				if previousState.State == stateUpdate.State && previousState.SyncMode == stateUpdate.SyncMode && previousState.SyncFilter == stateUpdate.SyncFilter && previousState.FlowAlias == stateUpdate.FlowAlias {
					continue
				} else if int(previousState.State) != previousStateCheck(int(stateUpdate.State)) && stateUpdate.State != previousState.State {
					sPC <- tfCtx.NewFlowStateUpdate(strconv.Itoa(int(previousState.State)), tfCtx.GetFlowSyncMode())
					continue
				}
				tfContext.SetPreviousFlowState(stateUpdate)
				tfCtx.SetFlowState(stateUpdate)
			}
		}
	}(tfContext, stateUpdateChannel)

	return stateUpdateChannel
}

func previousStateCheck(currentState int) int {
	switch currentState {
	case 0:
		return 3
	case 1:
		return 0
	case 2:
		return 1
	case 3:
		return 2
	default:
		return 3
	}
}

package core

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dolthub/go-mysql-server/sql"
	"github.com/glycerine/bchan"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	"github.com/trimble-oss/tierceron/atrium/trcflow/core/flowcorehelper"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	sys "github.com/trimble-oss/tierceron/pkg/vaulthelper/system"
)

type TrcFlowContext struct {
	FlowLibraryContext *flowcore.FlowLibraryContext // Reference to definition set used to define this flow.
	DataSourceRegions  []string
	RemoteDataSource   map[string]any
	GoMod              *helperkv.Modifier
	Vault              *sys.Vault

	// Recommended not to store contexts, but because we
	// are working with flows, this is a different pattern.
	// This just means some analytic tools won't be able to
	// perform analysis which are based on the Context.
	ContextNotifyChan    chan bool
	FlowLoadedNotifyChan *bchan.Bchan
	Context              context.Context
	CancelContext        context.CancelFunc
	// I flow is complex enough, it can define
	// it's own method for loading TrcDb
	// from vault.
	CustomSeedTrcDb func(flowcore.FlowMachineContext, flowcore.FlowContext) error

	FlowHeader            *flowcore.FlowHeaderType // Header providing information about the flow
	ChangeIdKeys          []string
	FlowPath              string
	FlowData              any
	ChangeFlowName        string // Change flow table name.
	FlowState             flowcorehelper.CurrentFlowState
	FlowStateLock         *sync.RWMutex                   // This is for sync concurrent changes to FlowState
	PreviousFlowState     flowcorehelper.CurrentFlowState // Temporary storage for previous state
	PreviousFlowStateLock *sync.RWMutex
	QueryLock             *sync.Mutex
	Inserter              sql.RowInserter
	LastRefreshed         string

	Restart                 bool
	Init                    bool
	WantsInitNotify         bool
	Preloaded               bool //
	TablesChangesInitted    bool //
	ReadOnly                bool
	DataFlowStatistic       FakeDFStat
	FlowChatMsgReceiverChan *chan *tccore.ChatMsg // Channel for receiving flow messages
	Logger                  *log.Logger
}

var _ flowcore.FlowContext = (*TrcFlowContext)(nil)

func (tfContext *TrcFlowContext) IsInit() bool {
	return tfContext.Init
}

func (tfContext *TrcFlowContext) SetInit(init bool) {
	tfContext.Init = init
}

func (tfContext *TrcFlowContext) IsPreloaded() bool {
	return tfContext.Preloaded
}

func (tfContext *TrcFlowContext) InitNotify() {
	tfContext.WantsInitNotify = true
}

func (tfContext *TrcFlowContext) IsRestart() bool {
	return tfContext.Restart
}

func (tfContext *TrcFlowContext) SetCustomSeedTrcdbFunc(customSeedTrcdb func(flowcore.FlowMachineContext, flowcore.FlowContext) error) {
	tfContext.CustomSeedTrcDb = customSeedTrcdb
}

func (tfContext *TrcFlowContext) SetFlowLibraryContext(flowLibraryContext *flowcore.FlowLibraryContext) {
	tfContext.FlowLibraryContext = flowLibraryContext
}

func (tfContext *TrcFlowContext) GetFlowLibraryContext() *flowcore.FlowLibraryContext {
	return tfContext.FlowLibraryContext
}

func (tfContext *TrcFlowContext) SetRestart(restart bool) {
	tfContext.Restart = restart
}

func (tfContext *TrcFlowContext) NotifyFlowComponentLoaded() {
	// Broadcast to all listeners that flow is loaded
	tfContext.FlowLoadedNotifyChan.Bcast(true)
	go func() {
		// Notify flow context it's loaded.
		tfContext.ContextNotifyChan <- true
	}()
}

func (tfContext *TrcFlowContext) NotifyFlowComponentNeedsRestart() {
	if tfContext.GetFlowStateState() != 3 {
		tfContext.PushState("flowStateReceiver", tfContext.NewFlowStateUpdate("3", tfContext.GetFlowSyncMode()))
	} else {
		return
	}
	for {
		if tfContext.GetFlowStateState() == 0 {
			break
		} else {
			time.Sleep(100 * time.Millisecond)
		}
	}
	// When state is set to 0, set state to 1 to trigger reload.
	if tfContext.GetFlowStateState() != 3 {
		tfContext.PushState("flowStateReceiver", tfContext.NewFlowStateUpdate("1", tfContext.GetFlowSyncMode()))
	}
}

func (tfContext *TrcFlowContext) WaitFlowLoaded() {
	// Wait for broadcast signal
	<-tfContext.FlowLoadedNotifyChan.Ch
	// Must call BcastAck after receiving
	tfContext.FlowLoadedNotifyChan.BcastAck()
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
			syncMatchModeLen := len(syncMode)
			if syncMatchModeLen > len(tfContext.FlowState.SyncMode) {
				syncMatchModeLen = len(tfContext.FlowState.SyncMode)
			}
			if syncMode == tfContext.FlowState.SyncMode[0:syncMatchModeLen] {
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

func (tfContext *TrcFlowContext) GetLastRefreshedTime() string {
	tfContext.FlowStateLock.RLock()
	defer tfContext.FlowStateLock.RUnlock()
	return tfContext.LastRefreshed
}

func (tfContext *TrcFlowContext) SetLastRefreshedTime(lastRefreshed string) {
	tfContext.FlowStateLock.Lock()
	defer tfContext.FlowStateLock.Unlock()
	tfContext.LastRefreshed = lastRefreshed
}

func (tfContext *TrcFlowContext) GetFlowSource() string {
	tfContext.FlowStateLock.RLock()
	defer tfContext.FlowStateLock.RUnlock()
	return tfContext.FlowHeader.Source
}

func (tfContext *TrcFlowContext) GetFlowSourceAlias() string {
	tfContext.FlowStateLock.RLock()
	defer tfContext.FlowStateLock.RUnlock()
	return tfContext.FlowHeader.SourceAlias
}

func (tfContext *TrcFlowContext) SetFlowSourceAlias(flowSourceAlias string) {
	tfContext.FlowStateLock.Lock()
	defer tfContext.FlowStateLock.Unlock()
	tfContext.FlowHeader.SourceAlias = flowSourceAlias
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
	new := flowState.(flowcorehelper.CurrentFlowState)
	tfContext.FlowState = new
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
	// return syncFilters
}

func (tfContext *TrcFlowContext) GetFlowHeader() *flowcore.FlowHeaderType {
	return tfContext.FlowHeader
}

func (tfContext *TrcFlowContext) NewFlowStateUpdate(state string, syncMode string) flowcore.FlowStateUpdate {
	tfContext.FlowStateLock.RLock()
	defer tfContext.FlowStateLock.RUnlock()
	return flowcorehelper.FlowStateUpdate{
		FlowName:    tfContext.GetFlowHeader().FlowName(),
		StateUpdate: state,
		SyncFilter:  tfContext.FlowState.SyncFilter,
		SyncMode:    syncMode,
		FlowAlias:   tfContext.FlowState.FlowAlias,
	}
}

func (tfContext *TrcFlowContext) GetCurrentFlowStateUpdateByDataSource(dataSource string) any {
	if tfContext.RemoteDataSource == nil {
		return nil
	}
	if flowStateController, ok := tfContext.RemoteDataSource[dataSource]; ok {
		return flowStateController
	}
	return nil
}

func (tfContext *TrcFlowContext) UpdateFlowStateByDataSource(dataSource string) {
	if tfContext.RemoteDataSource != nil {
		if flowStateController, ok := tfContext.RemoteDataSource[dataSource]; ok {
			newState := <-flowStateController.(chan flowcore.CurrentFlowState)
			tfContext.SetFlowState(newState)
		}
	}
}

func (tfContext *TrcFlowContext) PushState(dataSource string, flowStateUpdate flowcore.FlowStateUpdate) {
	if tfContext.RemoteDataSource != nil {
		if flowStateReceiver, ok := tfContext.RemoteDataSource[dataSource]; ok {
			flowStateReceiver.(chan flowcore.FlowStateUpdate) <- flowStateUpdate
		}
	}
}

func (tfContext *TrcFlowContext) GetUpdatePermission() flowcore.PermissionUpdate {
	return PermissionUpdate{
		TableName:    tfContext.GetFlowHeader().FlowName(),
		CurrentState: tfContext.GetFlowStateState(),
	}
}

func (tfContext *TrcFlowContext) GetFlowUpdate(currentFlowState flowcore.CurrentFlowState) flowcore.FlowStateUpdate {
	cfs := currentFlowState.(flowcorehelper.CurrentFlowState)
	return flowcorehelper.FlowStateUpdate{
		FlowName:    tfContext.GetFlowHeader().FlowName(),
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

func (tfContext *TrcFlowContext) GetRemoteDataSourceAttribute(dataSourceAttribute string, regions ...string) any {
	if tfContext.RemoteDataSource == nil {
		return nil
	}
	if len(regions) > 0 {
		if regionSource, ok := tfContext.RemoteDataSource[regions[0]].(map[string]any); ok {
			if remoteDataSourceAttribute, ok := regionSource[dataSourceAttribute]; ok {
				return remoteDataSourceAttribute
			} else {
				return nil
			}
		}
	}

	if remoteDataSourceAttribute, ok := tfContext.RemoteDataSource[dataSourceAttribute]; ok {
		return remoteDataSourceAttribute
	} else {
		lookup := fmt.Sprintf("region-%s", dataSourceAttribute)
		if remoteDataSourceAttribute, ok := tfContext.RemoteDataSource[lookup]; ok {
			return remoteDataSourceAttribute
		}

	}
	return nil
}

func (tfContext *TrcFlowContext) GetFlowChatMsgReceiverChan() *chan *tccore.ChatMsg {
	return tfContext.FlowChatMsgReceiverChan
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
	return tfContext.Logger
}

func (tfContext *TrcFlowContext) Log(msg string, err error) {
	if err != nil {
		tfContext.Logger.Printf("%s %s\n", msg, err.Error())
	} else {
		tfContext.Logger.Printf("%s\n", msg)
	}
}

func (tfContext *TrcFlowContext) TransitionState(syncMode string) {
	tfContext.UpdateFlowStateByDataSource("flowStateController")
	if syncMode != "" {
		tfContext.SetFlowSyncMode(syncMode)
	}
	stateUpdateChannel := tfContext.GetCurrentFlowStateUpdateByDataSource("flowStateReceiver")

	go func(tfCtx *TrcFlowContext, sPC any) {
		tfCtx.SetPreviousFlowState(tfCtx.GetFlowState()) // does get need locking...
		for {
			previousState := tfCtx.GetPreviousFlowState().(flowcorehelper.CurrentFlowState)
			stateUpdateI := <-tfCtx.GetCurrentFlowStateUpdateByDataSource("flowStateController").(chan flowcore.CurrentFlowState)
			stateUpdate := stateUpdateI.(flowcorehelper.CurrentFlowState)
			if syncMode != "" {
				stateUpdate.SyncMode = syncMode
				syncMode = ""
			}
			if previousState.State == stateUpdate.State && previousState.SyncMode == stateUpdate.SyncMode && previousState.SyncFilter == stateUpdate.SyncFilter && previousState.FlowAlias == stateUpdate.FlowAlias {
				continue
			} else if previousState.SyncMode == "refreshingDaily" && stateUpdate.SyncMode != "refreshEnd" && stateUpdate.State == 2 && int(previousState.State) != coreopts.BuildOptions.PreviousStateCheck(int(stateUpdate.State)) {
				sPC.(chan flowcore.FlowStateUpdate) <- flowcorehelper.FlowStateUpdate{FlowName: tfCtx.FlowHeader.TableName(), StateUpdate: strconv.Itoa(int(stateUpdate.State)), SyncFilter: stateUpdate.SyncFilter, SyncMode: previousState.SyncMode, FlowAlias: tfCtx.GetFlowStateAlias()}
				break
			} else if int(previousState.State) != previousStateCheck(int(stateUpdate.State)) && stateUpdate.State != previousState.State {
				// Invalid state transition, send previous state
				sPC.(chan flowcore.FlowStateUpdate) <- tfCtx.NewFlowStateUpdate(strconv.Itoa(int(previousState.State)), tfCtx.GetFlowSyncMode())
				continue
			}
			tfCtx.SetPreviousFlowState(stateUpdate)
			tfCtx.SetFlowState(stateUpdate)
		}
	}(tfContext, stateUpdateChannel)
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

package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"VaultConfig.TenantConfig/lib"
	tclibc "VaultConfig.TenantConfig/lib/libsqlc"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/flowcoreopts"
	"github.com/trimble-oss/tierceron/trcflow/core/flowcorehelper"

	trcvutils "github.com/trimble-oss/tierceron/trcvault/util"
	trcdb "github.com/trimble-oss/tierceron/trcx/db"
	trcengine "github.com/trimble-oss/tierceron/trcx/engine"
	"github.com/trimble-oss/tierceron/trcx/extract"
	sys "github.com/trimble-oss/tierceron/vaulthelper/system"

	dfssql "github.com/trimble-oss/tierceron/trcflow/flows/flowsql"
	eUtils "github.com/trimble-oss/tierceron/utils"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"

	"VaultConfig.TenantConfig/util/core"
	utilcore "VaultConfig.TenantConfig/util/core"

	sqle "github.com/dolthub/go-mysql-server/sql"
	sqlee "github.com/dolthub/go-mysql-server/sql/expression"
	"github.com/dolthub/vitess/go/sqltypes"
	"github.com/mrjrieke/nute/mashupsdk"

	sqlememory "github.com/dolthub/go-mysql-server/memory"
	sqles "github.com/dolthub/go-mysql-server/sql"
)

type FlowType int64
type FlowNameType string

var signalChannel chan os.Signal
var sourceDatabaseConnectionsMap map[string]map[string]interface{}
var tfmContextMap = make(map[string]*TrcFlowMachineContext, 5)

const (
	TableSyncFlow FlowType = iota
	TableEnrichFlow
	TableTestFlow
)

func getUpdateTrigger(databaseName string, tableName string, idColumnName string) string {
	return `CREATE TRIGGER tcUpdateTrigger AFTER UPDATE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + idColumnName + `, current_timestamp());` +
		` END;`
}

func getInsertTrigger(databaseName string, tableName string, idColumnName string) string {
	return `CREATE TRIGGER tcInsertTrigger AFTER INSERT ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
		` BEGIN` +
		` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + idColumnName + `, current_timestamp());` +
		` END;`
}

func (fnt FlowNameType) TableName() string {
	return string(fnt)
}

func (fnt FlowNameType) ServiceName() string {
	return string(fnt)
}

func TriggerAllChangeChannel(table string, changeIds map[string]string) {
	for _, tfmContext := range tfmContextMap {

		// If changIds identified, manually trigger a change.
		if table != "" {
			for changeIdKey, changeIdValue := range changeIds {
				if tfContext, tfContextOk := tfmContext.FlowMap[FlowNameType(table)]; tfContextOk {
					if tfContext.ChangeIdKey == changeIdKey {
						changeQuery := fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES (:id, current_timestamp())", tfContext.FlowSourceAlias, tfContext.ChangeFlowName)
						bindings := map[string]sqle.Expression{
							"id": sqlee.NewLiteral(changeIdValue, sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
						}
						_, _, _, _ = trcdb.QueryWithBindings(tfmContext.TierceronEngine, changeQuery, bindings, tfContext.FlowLock)
						break
					}
				}
			}
			if notificationFlowChannel, notificationChannelOk := tfmContext.ChannelMap[FlowNameType(table)]; notificationChannelOk {
				if len(notificationFlowChannel) == 5 {
				emptied:
					for i := 0; i < 3; i++ {
						select {
						case <-notificationFlowChannel:
						default:
							break emptied
						}
					}
				}
				go func(nfc chan bool) {
					nfc <- true
				}(notificationFlowChannel)
				return
			}
		}

		for _, notificationFlowChannel := range tfmContext.ChannelMap {
			if len(notificationFlowChannel) == 5 {
			emptied1:
				for i := 0; i < 3; i++ {
					select {
					case <-notificationFlowChannel:
					default:
						break emptied1
					}
				}
			}
			if len(notificationFlowChannel) < 3 {
				go func(nfc chan bool) {
					nfc <- true
				}(notificationFlowChannel)
			}
		}
	}
}

type TrcFlowMachineContext struct {
	InitConfigWG       *sync.WaitGroup
	FlowControllerLock sync.Mutex

	Region                    string
	Env                       string
	FlowControllerInit        bool
	FlowControllerUpdateLock  sync.Mutex
	FlowControllerUpdateAlert chan string
	Config                    *eUtils.DriverConfig
	Vault                     *sys.Vault
	TierceronEngine           *trcengine.TierceronEngine
	ExtensionAuthData         map[string]interface{}
	ExtensionAuthDataReloader map[string]interface{}
	GetAdditionalFlowsByState func(teststate string) []FlowNameType
	ChannelMap                map[FlowNameType]chan bool
	FlowMap                   map[FlowNameType]*TrcFlowContext // Map of all running flows for engine
	PermissionChan            chan PermissionUpdate            // This channel is used to alert for dynamic permissions when tables are loaded
}

type PermissionUpdate struct {
	TableName    string
	CurrentState int64
}

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
	FlowLock          *sync.Mutex //This is for sync concurrent changes to FlowState
	Restart           bool
	Init              bool
	ReadOnly          bool
	Inserter          sqle.RowInserter
	DataFlowStatistic FakeDFStat
	Log               *log.Logger
}

type FakeDFStat struct {
	mashupsdk.MashupDetailedElement
	ChildNodes []FakeDFStat
}

var tableModifierLock sync.Mutex

func (tfmContext *TrcFlowMachineContext) GetTableModifierLock() *sync.Mutex {
	return &tableModifierLock
}

func TableCollationIdGen(tableName string) sqle.CollationID {
	return sqle.CollationID(sqle.Collation_utf8mb4_unicode_ci)
}

func (tfmContext *TrcFlowMachineContext) Init(
	sdbConnMap map[string]map[string]interface{},
	tableNames []string,
	additionalFlowNames []FlowNameType,
	testFlowNames []FlowNameType,
) error {
	sourceDatabaseConnectionsMap = sdbConnMap

	// Set up global signal capture.
	signalChannel = make(chan os.Signal, 1)
	signal.Notify(signalChannel,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	tfmContext.GetTableModifierLock().Lock()
	for _, tableName := range tableNames {
		changeTableName := tableName + "_Changes"
		if _, ok, _ := tfmContext.TierceronEngine.Database.GetTableInsensitive(tfmContext.TierceronEngine.Context, changeTableName); !ok {
			eUtils.LogInfo(tfmContext.Config, "Creating tierceron sql table: "+changeTableName)
			err := tfmContext.TierceronEngine.Database.CreateTable(tfmContext.TierceronEngine.Context, changeTableName,
				sqle.NewPrimaryKeySchema(sqle.Schema{
					{Name: "id", Type: flowcoreopts.GetIdColumnType(tableName), Source: changeTableName, PrimaryKey: true},
					{Name: "updateTime", Type: sqle.Timestamp, Source: changeTableName},
				}),
				TableCollationIdGen(tableName),
			)
			if err != nil {
				tfmContext.GetTableModifierLock().Unlock()
				eUtils.LogErrorObject(tfmContext.Config, err, false)
				return err
			}
		}
	}
	tfmContext.GetTableModifierLock().Unlock()
	eUtils.LogInfo(tfmContext.Config, "Tables creation completed.")

	tfmContext.ChannelMap = make(map[FlowNameType]chan bool)

	for _, table := range tableNames {
		tfmContext.ChannelMap[FlowNameType(table)] = make(chan bool, 5)
	}

	for _, f := range additionalFlowNames {
		tfmContext.ChannelMap[f] = make(chan bool, 5)
	}

	for _, f := range testFlowNames {
		tfmContext.ChannelMap[f] = make(chan bool, 5)
	}

	tfmContext.PermissionChan = make(chan PermissionUpdate, 10)
	tfmContextMap[tfmContext.TierceronEngine.Database.Name()+"_"+tfmContext.Env] = tfmContext
	return nil
}

func (tfmContext *TrcFlowMachineContext) AddTableSchema(tableSchema sqle.PrimaryKeySchema, tfContext *TrcFlowContext) {
	tableName := tfContext.Flow.TableName()
	// Create table if necessary.
	tfmContext.GetTableModifierLock().Lock()
	if _, ok, _ := tfmContext.TierceronEngine.Database.GetTableInsensitive(tfmContext.TierceronEngine.Context, tableName); !ok {
		//	ii. Init database and tables in local mysql engine instance.
		err := tfmContext.TierceronEngine.Database.CreateTable(tfmContext.TierceronEngine.Context, tableName, tableSchema, TableCollationIdGen(tableName))
		tfmContext.GetTableModifierLock().Unlock()

		if err != nil {
			tfContext.FlowState = flowcorehelper.CurrentFlowState{State: -1, SyncMode: "Could not create table.", SyncFilter: ""}
			tfmContext.Log("Could not create table.", err)
		} else {
			if tfContext.Flow.TableName() == "TierceronFlow" {
				tfContext.FlowState = flowcorehelper.CurrentFlowState{State: 2, SyncMode: "nosync", SyncFilter: ""}
			} else {
				select {
				case tfContext.FlowState = <-tfContext.RemoteDataSource["flowStateController"].(chan flowcorehelper.CurrentFlowState):
					tfmContext.Log("Flow ready for use: "+tfContext.Flow.TableName(), nil)
					tfContext.FlowLock.Lock()
					if tfContext.FlowState.State != 2 {
						tfContext.FlowLock.Unlock()
					} else {
						tfContext.FlowLock.Unlock()
					}
				case <-time.After(7 * time.Second):
					{
						tfContext.FlowState = flowcorehelper.CurrentFlowState{State: 0, SyncMode: "nosync", SyncFilter: ""}
						tfmContext.Log("Flow ready for use (but inactive due to invalid setup): "+tfContext.Flow.TableName(), nil)
					}
				}
			}
		}
	} else {
		tfmContext.GetTableModifierLock().Unlock()
		tfmContext.Log("Unrecognized table: "+tfContext.Flow.TableName(), nil)
	}
}

// Set up call back to enable a trigger to track
// whenever a row in a table changes...
func (tfmContext *TrcFlowMachineContext) CreateTableTriggers(trcfc *TrcFlowContext, identityColumnName string) {
	tfmContext.GetTableModifierLock().Lock()

	// Workaround triggers not firing: 9/30/2022
	trcfc.ChangeIdKey = identityColumnName

	//Create triggers
	var updTrigger sqle.TriggerDefinition
	var insTrigger sqle.TriggerDefinition
	insTrigger.Name = "tcInsertTrigger_" + trcfc.Flow.TableName()
	updTrigger.Name = "tcUpdateTrigger_" + trcfc.Flow.TableName()
	//Prevent duplicate triggers from existing
	existingTriggers, err := tfmContext.TierceronEngine.Database.GetTriggers(tfmContext.TierceronEngine.Context)
	if err != nil {
		tfmContext.GetTableModifierLock().Unlock()
		eUtils.CheckError(tfmContext.Config, err, false)
	}

	triggerExist := false
	for _, trigger := range existingTriggers {
		if trigger.Name == insTrigger.Name || trigger.Name == updTrigger.Name {
			triggerExist = true
		}
	}
	if !triggerExist {
		updTrigger.CreateStatement = getUpdateTrigger(tfmContext.TierceronEngine.Database.Name(), trcfc.Flow.TableName(), identityColumnName)
		insTrigger.CreateStatement = getInsertTrigger(tfmContext.TierceronEngine.Database.Name(), trcfc.Flow.TableName(), identityColumnName)
		tfmContext.TierceronEngine.Database.CreateTrigger(tfmContext.TierceronEngine.Context, updTrigger)
		tfmContext.TierceronEngine.Database.CreateTrigger(tfmContext.TierceronEngine.Context, insTrigger)
	}
	tfmContext.GetTableModifierLock().Unlock()
}

// Set up call back to enable a trigger to track
// whenever a row in a table changes...
func (tfmContext *TrcFlowMachineContext) CreateCompositeTableTriggers(trcfc *TrcFlowContext, iden1 string, iden2 string, insertT func(string, string, string, string) string, updateT func(string, string, string, string) string) {
	//Create triggers
	var updTrigger sqle.TriggerDefinition
	var insTrigger sqle.TriggerDefinition
	insTrigger.Name = "tcInsertTrigger_" + trcfc.Flow.TableName()
	updTrigger.Name = "tcUpdateTrigger_" + trcfc.Flow.TableName()
	//Prevent duplicate triggers from existing
	existingTriggers, err := tfmContext.TierceronEngine.Database.GetTriggers(tfmContext.TierceronEngine.Context)
	if err != nil {
		tfmContext.GetTableModifierLock().Unlock()
		eUtils.CheckError(tfmContext.Config, err, false)
	}

	triggerExist := false
	for _, trigger := range existingTriggers {
		if trigger.Name == insTrigger.Name || trigger.Name == updTrigger.Name {
			triggerExist = true
		}
	}
	if !triggerExist {
		updTrigger.CreateStatement = updateT(tfmContext.TierceronEngine.Database.Name(), trcfc.Flow.TableName(), iden1, iden2)
		insTrigger.CreateStatement = insertT(tfmContext.TierceronEngine.Database.Name(), trcfc.Flow.TableName(), iden1, iden2)
		tfmContext.TierceronEngine.Database.CreateTrigger(tfmContext.TierceronEngine.Context, updTrigger)
		tfmContext.TierceronEngine.Database.CreateTrigger(tfmContext.TierceronEngine.Context, insTrigger)
	}
}

// Set up call back to enable a trigger to track
// whenever a row in a table changes...
func (tfmContext *TrcFlowMachineContext) CreateDataFlowTableTriggers(trcfc *TrcFlowContext, iden1 string, iden2 string, iden3 string, insertT func(string, string, string, string, string) string, updateT func(string, string, string, string, string) string) {
	//Create triggers
	var updTrigger sqle.TriggerDefinition
	var insTrigger sqle.TriggerDefinition
	insTrigger.Name = "tcInsertTrigger_" + trcfc.Flow.TableName()
	updTrigger.Name = "tcUpdateTrigger_" + trcfc.Flow.TableName()
	//Prevent duplicate triggers from existing
	existingTriggers, err := tfmContext.TierceronEngine.Database.GetTriggers(tfmContext.TierceronEngine.Context)
	if err != nil {
		tfmContext.GetTableModifierLock().Unlock()
		eUtils.CheckError(tfmContext.Config, err, false)
	}

	triggerExist := false
	for _, trigger := range existingTriggers {
		if trigger.Name == insTrigger.Name || trigger.Name == updTrigger.Name {
			triggerExist = true
		}
	}
	if !triggerExist {
		updTrigger.CreateStatement = updateT(tfmContext.TierceronEngine.Database.Name(), trcfc.Flow.TableName(), iden1, iden2, iden3)
		insTrigger.CreateStatement = insertT(tfmContext.TierceronEngine.Database.Name(), trcfc.Flow.TableName(), iden1, iden2, iden3)
		tfmContext.TierceronEngine.Database.CreateTrigger(tfmContext.TierceronEngine.Context, updTrigger)
		tfmContext.TierceronEngine.Database.CreateTrigger(tfmContext.TierceronEngine.Context, insTrigger)
	}
}

func (tfmContext *TrcFlowMachineContext) GetFlowConfiguration(trcfc *TrcFlowContext, flowTemplatePath string) (map[string]interface{}, bool) {
	flowProject, flowService, flowConfigTemplatePath := eUtils.GetProjectService(flowTemplatePath)
	flowConfigTemplateName := eUtils.GetTemplateFileName(flowConfigTemplatePath, flowService)
	trcfc.GoMod.SectionKey = "/Restricted/"
	trcfc.GoMod.SectionName = flowService
	if refreshErr := trcfc.Vault.RefreshClient(); refreshErr != nil {
		// Panic situation...  Can't connect to vault... Wait until next cycle to try again.
		eUtils.LogErrorMessage(tfmContext.Config, "Failure to connect to vault.  It may be down...", false)
		eUtils.LogErrorObject(tfmContext.Config, refreshErr, false)
		return nil, false
	}
	properties, err := trcvutils.NewProperties(tfmContext.Config, trcfc.Vault, trcfc.GoMod, tfmContext.Env, flowProject, flowProject)
	if err != nil {
		return nil, false
	}
	return properties.GetConfigValues(flowService, flowConfigTemplateName)
}

// seedVaultCycle - looks for changes in TrcDb and seeds vault with changes and pushes them also to remote
//
//	data sources.
func (tfmContext *TrcFlowMachineContext) seedVaultCycle(tfContext *TrcFlowContext,
	identityColumnName string,
	indexColumnNames interface{},
	getIndexedPathExt func(engine interface{}, rowDataMap map[string]interface{}, indexColumnNames interface{}, databaseName string, tableName string, dbCallBack func(interface{}, map[string]interface{}) (string, []string, [][]interface{}, error)) (string, error),
	flowPushRemote func(*TrcFlowContext, map[string]interface{}, map[string]interface{}) error,
	sqlState bool) {

	mysqlPushEnabled := sqlState
	flowChangedChannel := tfmContext.ChannelMap[tfContext.Flow]
	go func(fcc chan bool) {
		fcc <- true
	}(flowChangedChannel)

	for {
		select {
		case <-signalChannel:
			eUtils.LogErrorMessage(tfmContext.Config, "Receiving shutdown presumably from vault.", true)
			os.Exit(0)
		case <-flowChangedChannel:
			if len(flowChangedChannel) == 5 {
			emptied:
				for i := 0; i < 3; i++ {
					select {
					case <-flowChangedChannel:
					default:
						break emptied
					}
				}
			}
			tfmContext.vaultPersistPushRemoteChanges(
				tfContext,
				identityColumnName,
				indexColumnNames,
				mysqlPushEnabled,
				getIndexedPathExt,
				flowPushRemote)
		case <-tfContext.Context.Done():
			tfmContext.Log(fmt.Sprintf("Flow shutdown: %s", tfContext.Flow), nil)
			tfmContext.vaultPersistPushRemoteChanges(
				tfContext,
				identityColumnName,
				indexColumnNames,
				mysqlPushEnabled,
				getIndexedPathExt,
				flowPushRemote)
			if tfContext.Restart {
				tfmContext.Log(fmt.Sprintf("Restarting flow: %s", tfContext.Flow), nil)
				// Reload table from vault...
				go tfmContext.SyncTableCycle(tfContext,
					identityColumnName,
					indexColumnNames,
					getIndexedPathExt,
					flowPushRemote,
					sqlState)
				tfContext.Restart = false
			}
			return
		case <-tfContext.ContextNotifyChan:
		}
	}
}

// Seeds TrcDb from vault...  useful during init.
func (tfmContext *TrcFlowMachineContext) seedTrcDbCycle(tfContext *TrcFlowContext,
	identityColumnName string,
	indexColumnNames interface{},
	getIndexedPathExt func(engine interface{}, rowDataMap map[string]interface{}, indexColumnNames interface{}, databaseName string, tableName string, dbCallBack func(interface{}, map[string]interface{}) (string, []string, [][]interface{}, error)) (string, error),
	flowPushRemote func(*TrcFlowContext, map[string]interface{}, map[string]interface{}) error,
	bootStrap bool,
	seedInitCompleteChan chan bool) {

	if bootStrap {
		removedTriggers := []sqle.TriggerDefinition{}
		tfmContext.GetTableModifierLock().Lock()
		triggers, err := tfmContext.TierceronEngine.Database.GetTriggers(tfmContext.TierceronEngine.Context)
		if err == nil {
			for _, trigger := range triggers {
				if strings.HasSuffix(trigger.Name, "_"+string(tfContext.Flow)) {
					err := tfmContext.TierceronEngine.Database.DropTrigger(tfmContext.TierceronEngine.Context, trigger.Name)
					if err == nil {
						removedTriggers = append(removedTriggers, trigger)
					}
				}
			}
		}
		tfmContext.GetTableModifierLock().Unlock()

		/*
			tfmContext.seedTrcDbFromChanges(			//Old implementation
				tfContext,								//Templatized approach
				identityColumnName,
				vaultIndexColumnName,
				true,
				getIndexedPathExt,
				flowPushRemote,
				tfmContext.GetTableModifierLock(),
			)
		*/
		tfmContext.seedTrcDbFromVault(tfContext) //New implementation - direct approach

		tfmContext.GetTableModifierLock().Lock()
		for _, trigger := range removedTriggers {
			tfmContext.TierceronEngine.Database.CreateTrigger(tfmContext.TierceronEngine.Context, trigger)
		}
		tfmContext.GetTableModifierLock().Unlock()
		seedInitCompleteChan <- true
		if importChan, ok := tfContext.RemoteDataSource["vaultImportChannel"].(chan bool); ok {
			importChan <- true
		}
	}

	// Check vault hourly for changes to sync with mysql
	/* TODO: Seed mysql from Vault currently only work on insert level, not update...
		         Before this can be uncommented, the Insert/Update must be implemented.

		afterTime := time.Duration(time.Hour * 1) // More expensive to test vault for changes.
	                                              // Only check once an hour for changes in vault.
		flowChangedChannel := tfmContext.ChannelMap[tfContext.Flow]

		for {
			select {
			case <-signalChannel:
				eUtils.LogErrorMessage(tfmContext.Config, "Receiving shutdown presumably from vault.", true)
				os.Exit(0)
			case <-flowChangedChannel:
				tfmContext.seedTrcDbFromChanges(
					tfContext,
					identityColumnName,
					vaultIndexColumnName,
					false,
					getIndexedPathExt,
					flowPushRemote)
			case <-time.After(afterTime):
				afterTime = time.Minute * 3
				eUtils.LogInfo(tfmContext.Config, "3 minutes... checking local mysql for changes for sync with remote and vault.")
				tfmContext.seedTrcDbFromChanges(
					tfContext,
					identityColumnName,
					vaultIndexColumnName,
					false,
					getIndexedPathExt,
					flowPushRemote)
			}
		}
	*/
}

func (tfmContext *TrcFlowMachineContext) SyncTableCycle(tfContext *TrcFlowContext,
	identityColumnName string,
	indexColumnNames interface{},
	getIndexedPathExt func(engine interface{}, rowDataMap map[string]interface{}, indexColumnNames interface{}, databaseName string, tableName string, dbCallBack func(interface{}, map[string]interface{}) (string, []string, [][]interface{}, error)) (string, error),
	flowPushRemote func(*TrcFlowContext, map[string]interface{}, map[string]interface{}) error,
	sqlState bool) {

	// 2 rows (on startup always):
	//    1. Flow state... 0,1,2,3   0 - flow stopped, 1 starting up, 2 running, 3 shutting down.
	// 1st row:
	// flowName : tfContext.Flow.TableName()
	// argosid: system
	// flowGroup: System (hardcoded)
	// mode: 2 if flow is stopped, 1 if flow is running.
	//
	// Rows 3-4 is reserved for push or pull external activity.
	// Calls: Init, Update, Finish
	tfContext.Context, tfContext.CancelContext = context.WithCancel(context.Background())
	go func() {
		tfContext.ContextNotifyChan <- true
	}()
	//First row here:

	// tfContext.DataFlowStatistic["FlowState"] = ""
	// tfContext.DataFlowStatistic["flowName"] = ""
	// tfContext.DataFlowStatistic["flume"] = "" //Used to be argosid
	// tfContext.DataFlowStatistic["Flows"] = "" //Used to be flowGroup
	// tfContext.DataFlowStatistic["mode"] = ""
	df := InitDataFlow(nil, tfContext.Flow.TableName(), true) //Initializing dataflow
	if tfContext.FlowState.FlowAlias != "" {
		df.UpdateDataFlowStatistic("Flows", tfContext.FlowState.FlowAlias, "Loading", "1", 1, tfmContext.Log)
	} else {
		df.UpdateDataFlowStatistic("Flows", tfContext.Flow.TableName(), "Loading", "1", 1, tfmContext.Log)
	}
	//Copy ReportStatistics from process_registerenterprise.go if !buildopts.IsTenantAutoRegReady(tenant)
	// Do we need to account for that here?
	var seedInitComplete chan bool = make(chan bool, 1)
	tfContext.FlowLock.Lock()
	// if it's in sync complete on startup, reset the mode to pullcomplete.
	if tfContext.FlowState.SyncMode == "pullsynccomplete" {
		tfContext.FlowState.SyncMode = "pullcomplete"
		stateUpdateChannel := tfContext.RemoteDataSource["flowStateReceiver"].(chan flowcorehelper.FlowStateUpdate)
		go func(ftn string, sf string) {
			stateUpdateChannel <- flowcorehelper.FlowStateUpdate{FlowName: ftn, StateUpdate: "2", SyncFilter: sf, SyncMode: "pullcomplete"}
		}(tfContext.Flow.TableName(), tfContext.FlowState.SyncFilter)
	}
	tfContext.FlowLock.Unlock()

	if _, ok := tfContext.RemoteDataSource["connection"].(*sql.DB); !ok {
		flowPushRemote = nil
	}
	if !tfContext.Restart {
		go tfmContext.seedTrcDbCycle(tfContext, identityColumnName, indexColumnNames, getIndexedPathExt, flowPushRemote, true, seedInitComplete)
	} else {
		seedInitComplete <- true
	}
	<-seedInitComplete
	if tfContext.FlowState.FlowAlias != "" {
		df.UpdateDataFlowStatistic("Flows", tfContext.FlowState.FlowAlias, "Load complete", "2", 1, tfmContext.Log)
	} else {
		df.UpdateDataFlowStatistic("Flows", tfContext.Flow.TableName(), "Load complete", "2", 1, tfmContext.Log)
	}

	// Second row here
	// Not sure if necessary to copy entire ReportStatistics method
	tenantIndexPath, tenantDFSIdPath := core.GetDFSPathName()
	df.FinishStatistic(tfmContext, tfContext, tfContext.GoMod, "flume", tenantIndexPath, tenantDFSIdPath, tfmContext.Config.Log, false)

	//df.FinishStatistic(tfmContext, tfContext, tfContext.GoMod, ...)
	tfmContext.FlowControllerLock.Lock()
	if tfmContext.InitConfigWG != nil {
		tfmContext.InitConfigWG.Done()
	}
	tfmContext.FlowControllerLock.Unlock()
	tfContext.FlowLock.Lock()
	if tfContext.Init { //Alert interface that the table is ready for permissions
		tfContext.FlowLock.Unlock()
		tfmContext.PermissionChan <- PermissionUpdate{tfContext.Flow.TableName(), tfContext.FlowState.State}
		tfContext.Init = false
	} else if tfContext.FlowState.State == 2 {
		tfContext.FlowLock.Unlock()
		tfmContext.Log("Flow ready for use: "+tfContext.Flow.TableName(), nil)
	} else {
		tfContext.FlowLock.Unlock()
		tfmContext.Log("Unexpected flow state: "+tfContext.Flow.TableName(), nil)
	}

	go tfmContext.seedVaultCycle(tfContext, identityColumnName, indexColumnNames, getIndexedPathExt, flowPushRemote, sqlState)
}

func (tfmContext *TrcFlowMachineContext) SelectFlowChannel(tfContext *TrcFlowContext) <-chan bool {
	if notificationFlowChannel, ok := tfmContext.ChannelMap[tfContext.Flow]; ok {
		return notificationFlowChannel
	}
	tfmContext.Log("Could not find channel for flow.", nil)

	return nil
}

// Make a call on Call back to insert or update using the provided query.
// If this is expected to result in a change to an existing table, thern trigger
// something to the changed channel.
func (tfmContext *TrcFlowMachineContext) CallDBQuery(tfContext *TrcFlowContext,
	queryMap map[string]interface{},
	bindings map[string]sqle.Expression, // Optional param
	changed bool,
	operation string,
	flowNotifications []FlowNameType, // On successful completion, which flows to notify.
	flowtestState string) [][]interface{} {

	if queryMap["TrcQuery"].(string) == "" {
		return nil
	}
	if operation == "INSERT" {
		var matrix [][]interface{}
		var err error
		if bindings == nil {
			_, _, matrix, err = trcdb.Query(tfmContext.TierceronEngine, queryMap["TrcQuery"].(string), tfContext.FlowLock)
			if len(matrix) == 0 {
				changed = false
			}
		} else {
			tableName, _, _, err := trcdb.QueryWithBindings(tfmContext.TierceronEngine, queryMap["TrcQuery"].(string), bindings, tfContext.FlowLock)

			if err == nil && tableName == "ok" {
				changed = true
				matrix = append(matrix, []interface{}{})
			}
		}
		if err != nil {
			eUtils.LogErrorObject(tfmContext.Config, err, false)
		}
		if changed && len(matrix) > 0 {

			// If triggers are ever fixed, this can be removed.
			if changeIdValue, changeIdValueOk := queryMap["TrcChangeId"].(string); changeIdValueOk {
				changeQuery := fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES (:id, current_timestamp())", tfContext.FlowSourceAlias, tfContext.ChangeFlowName)
				bindings := map[string]sqle.Expression{
					"id": sqlee.NewLiteral(changeIdValue, sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
				}
				_, _, matrix, err = trcdb.QueryWithBindings(tfmContext.TierceronEngine, changeQuery, bindings, tfContext.FlowLock)
				if err != nil {
					tfmContext.Log("Failed to insert changes for INSERT.", err)
				}
			} else if changeIdValues, changeIdValueOk := queryMap["TrcChangeId"].([]string); changeIdValueOk && len(changeIdValues) == 2 {
				if changeIdCols, changeIdColOk := queryMap["TrcChangeCol"].([]string); changeIdColOk && len(changeIdCols) == 2 {
					changeQuery := fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES (:"+changeIdCols[0]+", :"+changeIdCols[1]+", current_timestamp())", utilcore.GetDatabaseName(), tfContext.ChangeFlowName)
					bindings := map[string]sqle.Expression{
						changeIdCols[0]: sqlee.NewLiteral(changeIdValues[0], sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
						changeIdCols[1]: sqlee.NewLiteral(changeIdValues[1], sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
					}
					_, _, matrix, err = trcdb.QueryWithBindings(tfmContext.TierceronEngine, changeQuery, bindings, tfContext.FlowLock)
					if err != nil {
						tfmContext.Log("Failed to insert changes for INSERT - 2A.", err)
					}
				} else {
					tfmContext.Log("Failed to find changed column Ids for INSERT - 2A", err)
				}
			} else if changeIdValues, changeIdValueOk := queryMap["TrcChangeId"].([]string); changeIdValueOk && len(changeIdValues) == 3 {
				changeQuery := fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES (:"+dfssql.DataflowTestNameColumn+", :"+dfssql.DataflowTestIdColumn+", :"+dfssql.DataflowTestStateCodeColumn+", current_timestamp())", utilcore.GetDatabaseName(), "DataFlowStatistics_Changes")
				bindings := map[string]sqle.Expression{
					dfssql.DataflowTestNameColumn:      sqlee.NewLiteral(changeIdValues[0], sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
					dfssql.DataflowTestIdColumn:        sqlee.NewLiteral(changeIdValues[1], sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
					dfssql.DataflowTestStateCodeColumn: sqlee.NewLiteral(changeIdValues[2], sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
				}
				_, _, matrix, err = trcdb.QueryWithBindings(tfmContext.TierceronEngine, changeQuery, bindings, tfContext.FlowLock)
				if err != nil {
					tfmContext.Log("Failed to insert dfs changes for INSERT.", err)
				}
			}

			if len(flowNotifications) > 0 {
				// look up channels and notify them too.
				for _, flowNotification := range flowNotifications {
					if notificationFlowChannel, ok := tfmContext.ChannelMap[flowNotification]; ok {
						if len(notificationFlowChannel) < 5 {
							go func(nfc chan bool) {
								nfc <- true
							}(notificationFlowChannel)
						}
					}
				}
			}
			// If this is a test...  Also inject notifications appropriately.
			if flowtestState != "" {
				additionalTestFlows := tfmContext.GetAdditionalFlowsByState(flowtestState)
				for _, flowNotification := range additionalTestFlows {
					if notificationFlowChannel, ok := tfmContext.ChannelMap[flowNotification]; ok {
						if len(notificationFlowChannel) < 5 {
							go func(nfc chan bool) {
								nfc <- true
							}(notificationFlowChannel)
						}
					}
				}
			}
		}
	} else if operation == "UPDATE" || operation == "DELETE" {
		var tableName string
		var matrix [][]interface{}
		var err error

		if bindings == nil {
			tableName, _, matrix, err = trcdb.Query(tfmContext.TierceronEngine, queryMap["TrcQuery"].(string), tfContext.FlowLock)
			if err == nil && tableName == "ok" {
				changed = true
				matrix = append(matrix, []interface{}{})
			} else if len(matrix) == 0 {
				changed = false
			}
		} else {
			tableName, _, _, err = trcdb.QueryWithBindings(tfmContext.TierceronEngine, queryMap["TrcQuery"].(string), bindings, tfContext.FlowLock)

			if err == nil && tableName == "ok" {
				changed = true
				matrix = append(matrix, []interface{}{})
				tfmContext.Log("UPDATE successful.", nil)
			} else {
				tfmContext.Log("UPDATE failed.", nil)
			}
		}

		if err != nil {
			eUtils.LogErrorObject(tfmContext.Config, err, false)
		}
		if changed && (len(matrix) > 0 || tableName != "") {
			// If triggers are ever fixed, this can be removed.
			if changeIdValue, changeIdValueOk := queryMap["TrcChangeId"].(string); changeIdValueOk {
				changeQuery := fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES (:id, current_timestamp())", tfContext.FlowSourceAlias, tfContext.ChangeFlowName)
				bindings := map[string]sqle.Expression{
					"id": sqlee.NewLiteral(changeIdValue, sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
				}
				_, _, matrix, err = trcdb.QueryWithBindings(tfmContext.TierceronEngine, changeQuery, bindings, tfContext.FlowLock)
				if err != nil {
					tfmContext.Log("Failed to insert changes for UPDATE.", err)
				}
			} else if changeIdValues, changeIdValueOk := queryMap["TrcChangeId"].([]string); changeIdValueOk && len(changeIdValues) == 2 {
				if changeIdCols, changeIdColOk := queryMap["TrcChangeCol"].([]string); changeIdColOk && len(changeIdCols) == 2 {
					changeQuery := fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES (:"+changeIdCols[0]+", :"+changeIdCols[1]+", current_timestamp())", utilcore.GetDatabaseName(), tfContext.ChangeFlowName)
					bindings := map[string]sqle.Expression{
						changeIdCols[0]: sqlee.NewLiteral(changeIdValues[0], sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
						changeIdCols[1]: sqlee.NewLiteral(changeIdValues[1], sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
					}
					_, _, matrix, err = trcdb.QueryWithBindings(tfmContext.TierceronEngine, changeQuery, bindings, tfContext.FlowLock)
					if err != nil {
						tfmContext.Log("Failed to insert changes for UPDATE - 2A.", err)
					}
				} else {
					tfmContext.Log("Failed to find changed column Ids for UPDATE - 2A", err)
				}
			} else if changeIdValues, changeIdValueOk := queryMap["TrcChangeId"].([]string); changeIdValueOk && len(changeIdValues) == 3 {
				changeQuery := fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES (:"+dfssql.DataflowTestNameColumn+", :"+dfssql.DataflowTestIdColumn+", :"+dfssql.DataflowTestStateCodeColumn+", current_timestamp())", utilcore.GetDatabaseName(), "DataFlowStatistics_Changes")
				bindings := map[string]sqle.Expression{
					dfssql.DataflowTestNameColumn:      sqlee.NewLiteral(changeIdValues[0], sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
					dfssql.DataflowTestIdColumn:        sqlee.NewLiteral(changeIdValues[1], sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
					dfssql.DataflowTestStateCodeColumn: sqlee.NewLiteral(changeIdValues[2], sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
				}
				_, _, matrix, err = trcdb.QueryWithBindings(tfmContext.TierceronEngine, changeQuery, bindings, tfContext.FlowLock)
				if err != nil {
					tfmContext.Log("Failed to insert dfs changes for UPDATE.", err)
				}
			}

			if len(flowNotifications) > 0 {
				// look up channels and notify them too.
				for _, flowNotification := range flowNotifications {
					if notificationFlowChannel, ok := tfmContext.ChannelMap[flowNotification]; ok {
						if len(notificationFlowChannel) < 5 {
							go func(nfc chan bool) {
								nfc <- true
							}(notificationFlowChannel)
						}
					}
				}
			}
			// If this is a test...  Also inject notifications appropriately.
			if flowtestState != "" {
				additionalTestFlows := tfmContext.GetAdditionalFlowsByState(flowtestState)
				for _, flowNotification := range additionalTestFlows {
					if notificationFlowChannel, ok := tfmContext.ChannelMap[flowNotification]; ok {
						if len(notificationFlowChannel) < 5 {
							go func(nfc chan bool) {
								nfc <- true
							}(notificationFlowChannel)
						}
					}
				}
			}
		}
	} else if operation == "SELECT" {
		_, _, matrixChangedEntries, err := trcdb.Query(tfmContext.TierceronEngine, queryMap["TrcQuery"].(string), tfContext.FlowLock)
		if err != nil {
			eUtils.LogErrorObject(tfmContext.Config, err, false)
		}
		return matrixChangedEntries
	}
	return nil
}

// Open a database connection to the provided source using provided
// source configurations.
func (tfmContext *TrcFlowMachineContext) GetDbConn(tfContext *TrcFlowContext, dbUrl string, username string, sourceDBConfig map[string]interface{}) (*sql.DB, error) {
	return trcvutils.OpenDirectConnection(tfmContext.Config, dbUrl,
		username,
		coreopts.DecryptSecretConfig(sourceDBConfig, sourceDatabaseConnectionsMap[tfContext.RemoteDataSource["dbsourceregion"].(string)]))
}

// Utilizing provided api auth headers, endpoint, and body data
// this CB makes a call on behalf of the caller and returns a map
// representation of json data provided by the endpoint.
func (tfmContext *TrcFlowMachineContext) CallAPI(apiAuthHeaders map[string]string, host string, apiEndpoint string, bodyData io.Reader, getOrPost bool) (map[string]interface{}, int, error) {
	httpClient, err := helperkv.CreateHTTPClient(false, host, tfmContext.Env, false)
	if httpClient != nil {
		defer httpClient.CloseIdleConnections()
	}

	if err != nil {
		return nil, -1, err
	}
	if getOrPost {
		return trcvutils.GetJSONFromClientByGet(tfmContext.Config, httpClient, apiAuthHeaders, apiEndpoint, bodyData)
	}
	return trcvutils.GetJSONFromClientByPost(tfmContext.Config, httpClient, apiAuthHeaders, apiEndpoint, bodyData)
}

func (tfmContext *TrcFlowMachineContext) Log(msg string, err error) {
	if err != nil {
		eUtils.LogMessageErrorObject(tfmContext.Config, msg, err, false)
	} else {
		eUtils.LogInfo(tfmContext.Config, msg)
	}
}

func (tfmContext *TrcFlowMachineContext) ProcessFlow(
	config *eUtils.DriverConfig,
	tfContext *TrcFlowContext,
	processFlowController func(tfmContext *TrcFlowMachineContext, tfContext *TrcFlowContext) error,
	vaultDatabaseConfig map[string]interface{}, // TODO: actually use this to set up a mysql facade.
	sourceDatabaseConnectionMap map[string]interface{},
	flow FlowNameType,
	flowType FlowType) error {

	// 	i. Init engine
	//     a. Get project, service, and table config template name.
	if flowType == TableSyncFlow {
		tfContext.ChangeFlowName = tfContext.Flow.TableName() + "_Changes"
		tfContext.FlowLock = &sync.Mutex{}
		// 3. Create a base seed template for use in vault seed process.
		var baseTableTemplate extract.TemplateResultData
		trcvutils.LoadBaseTemplate(config, &baseTableTemplate, tfContext.GoMod, tfContext.FlowSource, tfContext.Flow.ServiceName(), tfContext.FlowPath)
		tfContext.FlowData = &baseTableTemplate
	} else {
		// Use the flow name directly.
		tfContext.FlowSource = flow.ServiceName()
	}

	tfContext.RemoteDataSource["dbsourceregion"] = sourceDatabaseConnectionMap["dbsourceregion"]
	tfContext.RemoteDataSource["dbingestinterval"] = sourceDatabaseConnectionMap["dbingestinterval"] //60000
	//if mysql.IsMysqlPullEnabled() || mysql.IsMysqlPushEnabled() { //Flag is now replaced by syncMode in controller
	// Create remote data source with only what is needed.
	if flow.ServiceName() != flowcorehelper.TierceronFlowConfigurationTableName {
		if _, ok := sourceDatabaseConnectionMap["dbsourceurl"].(string); ok {

			eUtils.LogInfo(config, "Obtaining resource connections for : "+flow.ServiceName())
			dbsourceConn, err := trcvutils.OpenDirectConnection(config, sourceDatabaseConnectionMap["dbsourceurl"].(string), sourceDatabaseConnectionMap["dbsourceuser"].(string), sourceDatabaseConnectionMap["dbsourcepassword"].(string))

			if err != nil {
				eUtils.LogErrorMessage(config, "Couldn't get dedicated database connection.  Sync modes will fail.", false)
			} else {
				defer dbsourceConn.Close()
			}

			tfContext.RemoteDataSource["connection"] = dbsourceConn
		}
	}

	if initConfigWG, ok := tfContext.RemoteDataSource["controllerInitWG"].(*sync.WaitGroup); ok {
		tfmContext.FlowControllerUpdateLock.Lock()
		if initConfigWG != nil {
			initConfigWG.Done()
		}
		tfmContext.FlowControllerUpdateLock.Unlock()
	}
	//}
	//
	// Hand processing off to process flow implementor.
	//
	flowError := processFlowController(tfmContext, tfContext)
	if flowError != nil {
		eUtils.LogErrorObject(config, flowError, true)
	}

	return nil
}

func (tfmContext *TrcFlowMachineContext) PathToTableRowHelper(tfContext *TrcFlowContext) ([]interface{}, error) {
	dataMap, readErr := tfContext.GoMod.ReadData(tfContext.GoMod.SectionPath)
	if readErr != nil {
		return nil, readErr
	}

	rowDataMap := make(map[string]string, 1)
	for columnName, columnData := range dataMap {
		if dataString, ok := columnData.(string); ok {
			rowDataMap[columnName] = dataString
		} else {
			return nil, errors.New("Found data that was not a string - unable to write columnName: " + columnName + " to " + tfContext.Flow.TableName())
		}
	}
	row := tfmContext.writeToTableHelper(tfContext, nil, rowDataMap)

	if row != nil {
		return row, nil
	}
	return nil, nil
}

func (tfmContext *TrcFlowMachineContext) writeToTableHelper(tfContext *TrcFlowContext, valueColumns map[string]string, secretColumns map[string]string) []interface{} {

	tableSql, tableOk, _ := tfmContext.TierceronEngine.Database.GetTableInsensitive(nil, tfContext.Flow.TableName())
	var table *sqlememory.Table

	// TODO: Do we want back lookup by enterpriseId on all rows?
	// if enterpriseId, ok := secretColumns["EnterpriseId"]; ok {
	// 	valueColumns["_EnterpriseId_"] = enterpriseId
	// }
	// valueColumns["_Version_"] = version

	if !tableOk {
		// This is cacheable...
		tableSchema := sqles.NewPrimaryKeySchema([]*sqles.Column{})

		columnKeys := []string{}

		for valueKeyColumn := range valueColumns {
			columnKeys = append(columnKeys, valueKeyColumn)
		}

		for secretKeyColumn := range secretColumns {
			columnKeys = append(columnKeys, secretKeyColumn)
		}

		// Alpha sort -- yay...?
		sort.Strings(columnKeys)

		for _, columnKey := range columnKeys {
			column := sqles.Column{Name: columnKey, Type: sqles.Text, Source: tfContext.Flow.TableName()}
			tableSchema.Schema = append(tableSchema.Schema, &column)
		}

		table = sqlememory.NewTable(tfContext.Flow.TableName(), tableSchema, nil)
		m.Lock()
		tfmContext.TierceronEngine.Database.AddTable(tfContext.Flow.TableName(), table)
		m.Unlock()
	} else {
		table = tableSql.(*sqlememory.Table)
	}

	row := []interface{}{}

	// TODO: Add Enterprise, Environment, and Version....
	allDefaults := true
	for _, column := range table.Schema() {
		if value, ok := valueColumns[column.Name]; ok {
			var iVar interface{}
			var cErr error
			if value == "<Enter Secret Here>" || value == "" || value == "0" {
				iVar, cErr = column.Type.Convert("")
				if cErr != nil {
					iVar = nil
				}
			} else {
				iVar, _ = column.Type.Convert(stringClone(value))
				allDefaults = false
			}
			row = append(row, iVar)
		} else if secretValue, svOk := secretColumns[column.Name]; svOk {
			var iVar interface{}
			var cErr error
			if tclibc.CheckIncomingColumnName(column.Name) && secretValue != "<Enter Secret Here>" && secretValue != "" {
				decodedValue, secretValue, lmQuery, lm, incomingValErr := tclibc.CheckMysqlFileIncoming(secretColumns, secretValue, tfContext.FlowSourceAlias, tfContext.Flow.TableName())
				if incomingValErr != nil {
					eUtils.LogErrorObject(tfmContext.Config, incomingValErr, false)
					continue
				}
				if lmQuery != "" {
					rows := tfmContext.CallDBQuery(tfContext, map[string]interface{}{"TrcQuery": lmQuery}, nil, true, "SELECT", nil, "") //Query to alert change channel
					if len(rows) > 0 {
						if WhichLastModified(rows[0][0], lm) { //True if table is more recent
							continue
						}
					}
				}
				if secretValue == "" {
					iVar = []uint8(decodedValue)
				} else {
					iVar, _ = column.Type.Convert(stringClone(secretValue))
				}
				allDefaults = false
			} else if secretValue == "<Enter Secret Here>" || secretValue == "" {
				iVar, cErr = column.Type.Convert("")
				if cErr != nil {
					iVar = nil
				}
			} else {
				iVar, _ = column.Type.Convert(stringClone(secretValue))
				allDefaults = false
			}
			row = append(row, iVar)
		} else if _, svOk := secretColumns[column.Name]; !svOk {
			var iVar interface{}
			if tclibc.CheckIncomingAliasColumnName(column.Name) { //Specific case for controller
				iVar, _ = column.Type.Convert(row[0].(string))
			} else {
				iVar, _ = column.Type.Convert(column.Default.String())
			}
			row = append(row, iVar)
			//
		}
	}

	if !allDefaults {
		return row
	}
	return nil
}

// True if a time was most recent, false if b time was most recent.
func WhichLastModified(a interface{}, b interface{}) bool {
	//Check if a & b are time.time
	//Check if they match.
	var lastModifiedA time.Time
	var lastModifiedB time.Time
	var timeErr error
	if lastMA, ok := a.(time.Time); !ok {
		if lmA, ok := a.(string); ok {
			lastModifiedA, timeErr = time.Parse(lib.RFC_ISO_8601, lmA)
			if timeErr != nil {
				return false
			}
		}
	} else {
		lastModifiedA = lastMA
	}

	if lastMB, ok := b.(time.Time); !ok {
		if lmB, ok := b.(string); ok {
			lastModifiedB, timeErr = time.Parse(lib.RFC_ISO_8601, lmB)
			if timeErr != nil {
				return false
			}
		}
	} else {
		lastModifiedB = lastMB
	}

	if lastModifiedA != lastModifiedB {
		return lastModifiedA.After(lastModifiedB)
	} else {
		return true
	}
}

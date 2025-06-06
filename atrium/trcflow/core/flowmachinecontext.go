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

	sqlememory "github.com/dolthub/go-mysql-server/memory"
	sqle "github.com/dolthub/go-mysql-server/sql"
	sqlee "github.com/dolthub/go-mysql-server/sql/expression"
	"github.com/dolthub/vitess/go/sqltypes"
	"github.com/glycerine/bchan"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	coreutil "github.com/trimble-oss/tierceron-core/v2/util"

	"github.com/trimble-oss/tierceron/atrium/buildopts/flowcoreopts"
	trcdb "github.com/trimble-oss/tierceron/atrium/trcdb"
	trcengine "github.com/trimble-oss/tierceron/atrium/trcdb/engine"
	"github.com/trimble-oss/tierceron/atrium/trcflow/core/flowcorehelper"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	tcopts "github.com/trimble-oss/tierceron/buildopts/tcopts"
	trcdbutil "github.com/trimble-oss/tierceron/pkg/core/dbutil"
	"github.com/trimble-oss/tierceron/pkg/core/util"
	trcvutils "github.com/trimble-oss/tierceron/pkg/core/util"
	"github.com/trimble-oss/tierceron/pkg/trcx/extract"
	xencrypt "github.com/trimble-oss/tierceron/pkg/trcx/xencrypt"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	sys "github.com/trimble-oss/tierceron/pkg/vaulthelper/system"
)

type TrcFlowMachineContext struct {
	InitConfigWG       *sync.WaitGroup
	FlowControllerLock sync.Mutex

	Region                    string
	Env                       string
	FlowControllerInit        bool
	FlowControllerUpdateLock  sync.Mutex
	FlowControllerUpdateAlert chan string
	DriverConfig              *config.DriverConfig
	Vault                     *sys.Vault
	TierceronEngine           *trcengine.TierceronEngine
	ExtensionAuthData         map[string]interface{}
	ExtensionAuthDataReloader map[string]interface{}
	GetAdditionalFlowsByState func(teststate string) []flowcore.FlowNameType
	ChannelMap                map[flowcore.FlowNameType]*bchan.Bchan
	FlowMap                   map[flowcore.FlowNameType]*TrcFlowContext // Map of all running flows for engine
	FlowMapLock               sync.RWMutex
	PreloadChan               chan PermissionUpdate
	PermissionChan            chan PermissionUpdate // This channel is used to alert for dynamic permissions when tables are loaded
}

var _ flowcore.FlowMachineContext = (*TrcFlowMachineContext)(nil)

func (tfmContext *TrcFlowMachineContext) GetEnv() string {
	return tfmContext.Env
}
func (tfmContext *TrcFlowMachineContext) GetFlowContext(flowName flowcore.FlowNameType) flowcore.FlowContext {
	tfmContext.FlowMapLock.RLock()
	defer tfmContext.FlowMapLock.RUnlock()
	if flowContext, refOk := tfmContext.FlowMap[flowName]; refOk {
		return flowContext
	} else {
		return nil
	}

}

func (tfmContext *TrcFlowMachineContext) GetTrcFlowContext(flowName flowcore.FlowNameType) *TrcFlowContext {
	tfmContext.FlowMapLock.RLock()
	defer tfmContext.FlowMapLock.RUnlock()
	if flowContext, refOk := tfmContext.FlowMap[flowName]; refOk {
		return flowContext
	} else {
		return nil
	}
}

func (tfmContext *TrcFlowMachineContext) GetDatabaseName() string {
	return tfmContext.TierceronEngine.Database.Name()
}

func (tfmContext *TrcFlowMachineContext) GetTableModifierLock() *sync.Mutex {
	return &tableModifierLock
}

func (tfmContext *TrcFlowMachineContext) TableCollationIdGen(tableName string) interface{} {
	return TableCollationIdGen(tableName)
}

func TableCollationIdGen(tableName string) sqle.CollationID {
	return sqle.CollationID(sqle.Collation_utf8mb4_unicode_ci)
}

func (tfmContext *TrcFlowMachineContext) Init(
	sdbConnMap map[string]map[string]interface{},
	tableNames []string,
	additionalFlowNames []flowcore.FlowNameType,
	testFlowNames []flowcore.FlowNameType,
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
			tfmContext.LogInfo("Creating tierceron sql table: " + changeTableName)
			err := tfmContext.TierceronEngine.Database.CreateTable(tfmContext.TierceronEngine.Context, changeTableName,
				sqle.NewPrimaryKeySchema(sqle.Schema{
					{Name: "id", Type: flowcoreopts.BuildOptions.GetIdColumnType(tableName), Source: changeTableName, PrimaryKey: true},
					{Name: "updateTime", Type: sqle.Timestamp, Source: changeTableName},
				}),
				TableCollationIdGen(tableName),
			)
			if err != nil {
				tfmContext.GetTableModifierLock().Unlock()
				tfmContext.Log("Could not create table.", err)
				return err
			}
		}
	}
	tfmContext.GetTableModifierLock().Unlock()
	tfmContext.LogInfo("Tables creation completed.")

	tfmContext.ChannelMap = make(map[flowcore.FlowNameType]*bchan.Bchan)

	for _, table := range tableNames {
		tfmContext.ChannelMap[flowcore.FlowNameType(table)] = bchan.New(1)
	}

	for _, f := range additionalFlowNames {
		tfmContext.ChannelMap[flowcore.FlowNameType(f)] = bchan.New(1)
	}

	for _, f := range testFlowNames {
		tfmContext.ChannelMap[flowcore.FlowNameType(f)] = bchan.New(1)
	}

	tfmContext.PermissionChan = make(chan PermissionUpdate, 10)
	tfmContextMap[tfmContext.TierceronEngine.Database.Name()+"_"+tfmContext.Env] = tfmContext
	return nil
}

func ColumnTypeConverter(fnt flowcore.FlowColumnType) sqle.Type {
	switch fnt {
	case flowcore.TinyText:
		return sqle.TinyText
	case flowcore.Text:
		return sqle.Text
	case flowcore.MediumText:
		return sqle.MediumText
	case flowcore.LongText:
		return sqle.LongText
	case flowcore.TinyBlob:
		return sqle.TinyBlob
	case flowcore.Blob:
		return sqle.MediumText
	case flowcore.MediumBlob:
		return sqle.MediumBlob
	case flowcore.LongBlob:
		return sqle.LongBlob
	case flowcore.Int8:
		return sqle.Int8
	case flowcore.Uint8:
		return sqle.Uint8
	case flowcore.Int16:
		return sqle.Int16
	case flowcore.Uint16:
		return sqle.Uint16
	case flowcore.Int24:
		return sqle.Int24
	case flowcore.Uint24:
		return sqle.Uint24
	case flowcore.Int32:
		return sqle.Int32
	case flowcore.Uint32:
		return sqle.Uint32
	case flowcore.Int64:
		return sqle.Int64
	case flowcore.Uint64:
		return sqle.Uint64
	case flowcore.Float32:
		return sqle.Float32
	case flowcore.Float64:
		return sqle.Float64
	case flowcore.Timestamp:
		return sqle.Timestamp
	}
	return sqle.Text
}

func (tfmContext *TrcFlowMachineContext) AddTableSchema(tableSchemaI interface{}, tcflowContext flowcore.FlowContext) {
	var tableSchema sqle.PrimaryKeySchema
	if metaSchema, ok := tableSchemaI.([]flowcore.FlowColumn); ok {
		schema := sqle.Schema{}
		for _, col := range metaSchema {
			schema = append(schema, &sqle.Column{
				Name:       col.Name,
				Type:       ColumnTypeConverter(col.Type),
				Source:     col.Source,
				PrimaryKey: col.PrimaryKey})
		}
		tableSchema = sqle.NewPrimaryKeySchema(schema)
	} else if metaSchema, ok := tableSchemaI.(sqle.PrimaryKeySchema); ok {
		tableSchema = metaSchema
	} else {
		tfmContext.Log("Could not add table schema.  Invalid type: "+fmt.Sprintf("%T", tableSchemaI), nil)
		return
	}

	tfContext := tcflowContext.(*TrcFlowContext)
	tableName := tfContext.Flow.TableName()
	// Create table if necessary.
	tfmContext.GetTableModifierLock().Lock()
	if _, ok, _ := tfmContext.TierceronEngine.Database.GetTableInsensitive(tfmContext.TierceronEngine.Context, tableName); !ok {
		//	ii. Init database and tables in local mysql engine instance.
		err := tfmContext.TierceronEngine.Database.CreateTable(tfmContext.TierceronEngine.Context, tableName, tableSchema, TableCollationIdGen(tableName))
		tfmContext.GetTableModifierLock().Unlock()

		if err != nil {
			tfContext.SetFlowState(flowcorehelper.CurrentFlowState{State: -1, SyncMode: "Could not create table.", SyncFilter: ""})
			tfmContext.Log("Could not create table.", err)
		} else {
			if tfContext.Flow.TableName() == flowcorehelper.TierceronFlowConfigurationTableName {
				tfContext.SetFlowState(flowcorehelper.CurrentFlowState{State: 2, SyncMode: "nosync", SyncFilter: ""})
			} else {
				select {
				case newFlowState := <-tfContext.RemoteDataSource["flowStateController"].(chan flowcore.CurrentFlowState):
					tfContext.SetFlowState(newFlowState)
					tfmContext.Log("Flow ready for use: "+tfContext.Flow.TableName(), nil)
					if tfContext.GetFlowStateState() != 2 {
						// Chewbacca -- why?
					} else {
					}
				case <-time.After(15 * time.Second):
					{
						tfContext.SetFlowState(flowcorehelper.CurrentFlowState{State: 0, SyncMode: "nosync", SyncFilter: ""})
						tfmContext.Log("Flow ready for use (but inactive due to invalid setup): "+tfContext.Flow.TableName(), nil)
					}
				}
			}
		}
	} else {
		tfmContext.GetTableModifierLock().Unlock()
		tfmContext.Log("Recognized table: "+tfContext.Flow.TableName(), nil)
	}
}

func (tfmContext *TrcFlowMachineContext) CreateTable(name string, schemaI interface{}, collationI interface{}) error {
	schema := schemaI.(sqle.PrimaryKeySchema)
	collation := collationI.(sqle.CollationID)
	return tfmContext.TierceronEngine.Database.CreateTable(tfmContext.TierceronEngine.Context, name, schema, collation)
}

// Set up call back to enable a trigger to track
// whenever a row in a table changes...
func (tfmContext *TrcFlowMachineContext) CreateTableTriggers(tcflowContext flowcore.FlowContext,
	identityColumnNames []string) {
	tfContext := tcflowContext.(*TrcFlowContext)
	tfmContext.GetTableModifierLock().Lock()

	// Workaround triggers not firing: 9/30/2022
	tfContext.ChangeIdKeys = identityColumnNames

	//Create triggers
	var updTrigger sqle.TriggerDefinition
	var insTrigger sqle.TriggerDefinition
	var delTrigger sqle.TriggerDefinition
	insTrigger.Name = "tcInsertTrigger_" + tfContext.Flow.TableName()
	updTrigger.Name = "tcUpdateTrigger_" + tfContext.Flow.TableName()
	delTrigger.Name = "tcDeleteTrigger_" + tfContext.Flow.TableName()
	//Prevent duplicate triggers from existing
	existingTriggers, err := tfmContext.TierceronEngine.Database.GetTriggers(tfmContext.TierceronEngine.Context)
	if err != nil {
		tfmContext.GetTableModifierLock().Unlock()
		eUtils.CheckError(tfmContext.DriverConfig.CoreConfig, err, false)
	}

	triggerExist := false
	for _, trigger := range existingTriggers {
		if trigger.Name == insTrigger.Name || trigger.Name == updTrigger.Name || trigger.Name == delTrigger.Name {
			triggerExist = true
		}
	}
	if !triggerExist {
		updTrigger.CreateStatement = getUpdateTrigger(tfmContext.TierceronEngine.Database.Name(), tfContext.Flow.TableName(), identityColumnNames)
		insTrigger.CreateStatement = getInsertTrigger(tfmContext.TierceronEngine.Database.Name(), tfContext.Flow.TableName(), identityColumnNames)
		delTrigger.CreateStatement = getDeleteTrigger(tfmContext.TierceronEngine.Database.Name(), tfContext.Flow.TableName(), identityColumnNames)
		tfmContext.TierceronEngine.Database.CreateTrigger(tfmContext.TierceronEngine.Context, updTrigger)
		tfmContext.TierceronEngine.Database.CreateTrigger(tfmContext.TierceronEngine.Context, insTrigger)
		tfmContext.TierceronEngine.Database.CreateTrigger(tfmContext.TierceronEngine.Context, delTrigger)
	}
	tfmContext.GetTableModifierLock().Unlock()
}

// Set up call back to enable a trigger to track
// whenever a row in a table changes...
func (tfmContext *TrcFlowMachineContext) CreateCompositeTableTriggers(tcflowContext flowcore.FlowContext, iden1 string, iden2 string, insertT func(string, string, string, string) string, updateT func(string, string, string, string) string, deleteT func(string, string, string, string) string) {
	tfContext := tcflowContext.(*TrcFlowContext)
	//Create triggers
	var updTrigger sqle.TriggerDefinition
	var insTrigger sqle.TriggerDefinition
	var delTrigger sqle.TriggerDefinition
	insTrigger.Name = "tcInsertTrigger_" + tfContext.Flow.TableName()
	updTrigger.Name = "tcUpdateTrigger_" + tfContext.Flow.TableName()
	delTrigger.Name = "tcDeleteTrigger_" + tfContext.Flow.TableName()
	//Prevent duplicate triggers from existing
	existingTriggers, err := tfmContext.TierceronEngine.Database.GetTriggers(tfmContext.TierceronEngine.Context)
	if err != nil {
		tfmContext.GetTableModifierLock().Unlock()
		eUtils.CheckError(tfmContext.DriverConfig.CoreConfig, err, false)
	}

	triggerExist := false
	for _, trigger := range existingTriggers {
		if trigger.Name == insTrigger.Name || trigger.Name == updTrigger.Name || trigger.Name == delTrigger.Name {
			triggerExist = true
		}
	}
	if !triggerExist {
		updTrigger.CreateStatement = updateT(tfmContext.TierceronEngine.Database.Name(), tfContext.Flow.TableName(), iden1, iden2)
		insTrigger.CreateStatement = insertT(tfmContext.TierceronEngine.Database.Name(), tfContext.Flow.TableName(), iden1, iden2)
		delTrigger.CreateStatement = deleteT(tfmContext.TierceronEngine.Database.Name(), tfContext.Flow.TableName(), iden1, iden2)

		tfmContext.TierceronEngine.Database.CreateTrigger(tfmContext.TierceronEngine.Context, updTrigger)
		tfmContext.TierceronEngine.Database.CreateTrigger(tfmContext.TierceronEngine.Context, insTrigger)
		tfmContext.TierceronEngine.Database.CreateTrigger(tfmContext.TierceronEngine.Context, delTrigger)
	}
}

// Set up call back to enable a trigger to track
// whenever a row in a table changes...
func (tfmContext *TrcFlowMachineContext) CreateDataFlowTableTriggers(tcflowContext flowcore.FlowContext,
	iden1 string,
	iden2 string,
	iden3 string,
	insertT func(string, string, string, string, string) string,
	updateT func(string, string, string, string, string) string,
	deleteT func(string, string, string, string, string) string) {
	tfContext := tcflowContext.(*TrcFlowContext)

	//Create triggers
	var updTrigger sqle.TriggerDefinition
	var insTrigger sqle.TriggerDefinition
	var delTrigger sqle.TriggerDefinition
	insTrigger.Name = "tcInsertTrigger_" + tfContext.Flow.TableName()
	updTrigger.Name = "tcUpdateTrigger_" + tfContext.Flow.TableName()
	delTrigger.Name = "tcDeleteTrigger_" + tfContext.Flow.TableName()
	//Prevent duplicate triggers from existing
	existingTriggers, err := tfmContext.TierceronEngine.Database.GetTriggers(tfmContext.TierceronEngine.Context)
	if err != nil {
		tfmContext.GetTableModifierLock().Unlock()
		eUtils.CheckError(tfmContext.DriverConfig.CoreConfig, err, false)
	}

	triggerExist := false
	for _, trigger := range existingTriggers {
		if trigger.Name == insTrigger.Name || trigger.Name == updTrigger.Name || trigger.Name == delTrigger.Name {
			triggerExist = true
		}
	}
	if !triggerExist {
		updTrigger.CreateStatement = updateT(tfmContext.TierceronEngine.Database.Name(), tfContext.Flow.TableName(), iden1, iden2, iden3)
		insTrigger.CreateStatement = insertT(tfmContext.TierceronEngine.Database.Name(), tfContext.Flow.TableName(), iden1, iden2, iden3)
		delTrigger.CreateStatement = deleteT(tfmContext.TierceronEngine.Database.Name(), tfContext.Flow.TableName(), iden1, iden2, iden3)
		tfmContext.TierceronEngine.Database.CreateTrigger(tfmContext.TierceronEngine.Context, updTrigger)
		tfmContext.TierceronEngine.Database.CreateTrigger(tfmContext.TierceronEngine.Context, insTrigger)
		tfmContext.TierceronEngine.Database.CreateTrigger(tfmContext.TierceronEngine.Context, delTrigger)
	}
}

func (tfmContext *TrcFlowMachineContext) GetFlowConfiguration(tcflowContext flowcore.FlowContext,
	flowTemplatePath string) (map[string]interface{}, bool) {
	tfContext := tcflowContext.(*TrcFlowContext)
	// Get the flow configuration from vault.
	flowProject, flowService, _, flowConfigTemplatePath := coreutil.GetProjectService("", "trc_templates", flowTemplatePath)
	flowConfigTemplateName := coreutil.GetTemplateFileName(flowConfigTemplatePath, flowService)
	tfContext.GoMod.SectionKey = "/Restricted/"
	tfContext.GoMod.SectionName = flowService
	if refreshErr := tfContext.Vault.RefreshClient(); refreshErr != nil {
		// Panic situation...  Can't connect to vault... Wait until next cycle to try again.
		tfmContext.Log("Failure to connect to vault.  It may be down...", refreshErr)
		return nil, false
	}
	properties, err := trcvutils.NewProperties(tfmContext.DriverConfig.CoreConfig, tfContext.Vault, tfContext.GoMod, tfmContext.Env, flowProject, flowProject)
	if err != nil {
		return nil, false
	}
	return properties.GetConfigValues(flowService, flowConfigTemplateName)
}

// seedVaultCycle - looks for changes in TrcDb and seeds vault with changes and pushes them also to remote
//
//	data sources.
func (tfmContext *TrcFlowMachineContext) seedVaultCycle(tcflowContext flowcore.FlowContext,
	identityColumnNames []string,
	indexColumnNames interface{},
	getIndexedPathExt func(engine interface{}, rowDataMap map[string]interface{}, indexColumnNames interface{}, databaseName string, tableName string, dbCallBack func(interface{}, map[string]interface{}) (string, []string, [][]interface{}, error)) (string, error),
	flowPushRemote func(flowcore.FlowContext, map[string]interface{}) error,
	sqlState bool) {

	tfContext := tcflowContext.(*TrcFlowContext)

	mysqlPushEnabled := sqlState
	flowChangedChannel := tfmContext.ChannelMap[tfContext.Flow]
	//	flowChangedChannel.Bcast(true)

	for {
		select {
		case <-signalChannel:
			tfmContext.Log("Receiving shutdown presumably from vault.", nil)
			os.Exit(0)
		case <-flowChangedChannel.Ch:
			tfmContext.vaultPersistPushRemoteChanges(
				tfContext,
				identityColumnNames,
				indexColumnNames,
				mysqlPushEnabled,
				getIndexedPathExt,
				flowPushRemote)
			flowChangedChannel.Clear()
		case <-tfContext.Context.Done():
			tfmContext.Log(fmt.Sprintf("Flow shutdown: %s", tfContext.Flow), nil)
			tfmContext.vaultPersistPushRemoteChanges(
				tfContext,
				identityColumnNames,
				indexColumnNames,
				mysqlPushEnabled,
				getIndexedPathExt,
				flowPushRemote)
			if tfContext.Restart {
				tfmContext.Log(fmt.Sprintf("Restarting flow: %s", tfContext.Flow), nil)
				// Reload table from vault...
				go tfmContext.SyncTableCycle(tfContext,
					identityColumnNames,
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
	identityColumnName []string,
	indexColumnNames interface{},
	getIndexedPathExt func(engine interface{}, rowDataMap map[string]interface{}, indexColumnNames interface{}, databaseName string, tableName string, dbCallBack func(interface{}, map[string]interface{}) (string, []string, [][]interface{}, error)) (string, error),
	flowPushRemote func(flowcore.FlowContext, map[string]interface{}) error,
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

func (tfmContext *TrcFlowMachineContext) SyncTableCycle(tcflowContext flowcore.FlowContext,
	identityColumnNames []string,
	indexColumnNames interface{},
	getIndexedPathExt func(engine interface{}, rowDataMap map[string]interface{}, indexColumnNames interface{}, databaseName string, tableName string, dbCallBack func(interface{}, map[string]interface{}) (string, []string, [][]interface{}, error)) (string, error),
	flowPushRemote func(flowcore.FlowContext, map[string]interface{}) error,
	sqlState bool) {

	tfContext := tcflowContext.(*TrcFlowContext)

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
	var df *tccore.TTDINode = nil
	if tfContext.Init && tfContext.Flow.TableName() != flowcorehelper.TierceronFlowConfigurationTableName {
		df = tccore.InitDataFlow(nil, tfContext.Flow.TableName(), true) //Initializing dataflow
		if tfContext.GetFlowStateAlias() != "" {
			df.UpdateDataFlowStatistic("Flows", tfContext.GetFlowStateAlias(), "Loading", "1", 1, tfmContext.Log)
		} else {
			df.UpdateDataFlowStatistic("Flows", tfContext.Flow.TableName(), "Loading", "1", 1, tfmContext.Log)
		}
	}
	//Copy ReportStatistics from process_registerenterprise.go if !buildopts.BuildOptions.IsTenantAutoRegReady(tenant)
	// Do we need to account for that here?
	var seedInitComplete chan bool = make(chan bool, 1)
	// if it's in sync complete on startup, reset the mode to pullcomplete.
	if tfContext.GetFlowSyncMode() == "pullsynccomplete" {
		tfContext.SetFlowSyncMode("pullcomplete")
		if stateUpdateChannel, ok := tfContext.GetRemoteDataSourceAttribute("flowStateReceiver").(chan flowcorehelper.FlowStateUpdate); ok {
			go func(suc chan flowcorehelper.FlowStateUpdate, tfc *TrcFlowContext) {
				suc <- flowcorehelper.FlowStateUpdate{FlowName: tfc.Flow.TableName(), StateUpdate: "2", SyncFilter: tfc.GetFlowStateSyncFilterRaw(), SyncMode: "pullcomplete"}
			}(stateUpdateChannel, tfContext)
		}
	}

	if _, ok := tfContext.RemoteDataSource["connection"].(*sql.DB); !ok {
		flowPushRemote = nil
	}
	if !tfContext.Restart {
		go tfmContext.seedTrcDbCycle(tfContext, identityColumnNames, indexColumnNames, getIndexedPathExt, flowPushRemote, true, seedInitComplete)
	} else {
		seedInitComplete <- true
	}
	<-seedInitComplete
	if tfContext.Init && tfContext.Flow.TableName() != flowcorehelper.TierceronFlowConfigurationTableName {
		if tfContext.GetFlowStateAlias() != "" {
			df.UpdateDataFlowStatistic("Flows", tfContext.GetFlowStateAlias(), "Load complete", "2", 1, tfmContext.Log)
		} else {
			df.UpdateDataFlowStatistic("Flows", tfContext.Flow.TableName(), "Load complete", "2", 1, tfmContext.Log)
		}
	}

	// Second row here
	// Not sure if necessary to copy entire ReportStatistics method
	if tfContext.Init && tfContext.Flow.TableName() != flowcorehelper.TierceronFlowConfigurationTableName {
		tenantIndexPath, tenantDFSIdPath := coreopts.BuildOptions.GetDFSPathName()
		dsc, _, err := df.GetDeliverStatCtx()
		if err == nil {
			df.FinishStatistic("flume", tenantIndexPath, tenantDFSIdPath, tfmContext.DriverConfig.CoreConfig.Log, false, dsc)
		} else {
			tfmContext.Log("deliver stat ctx extraction error: "+tfContext.Flow.TableName(), err)
		}
	}

	//df.FinishStatistic(tfmContext, tfContext, tfContext.GoMod, ...)
	tfmContext.FlowControllerLock.Lock()
	if tfmContext.InitConfigWG != nil {
		tfmContext.InitConfigWG.Done()
	}
	tfmContext.FlowControllerLock.Unlock()
	if tfContext.Init { //Alert interface that the table is ready for permissions
		go func() {
			tfmContext.PreloadChan <- PermissionUpdate{tfContext.Flow.TableName(), tfContext.GetFlowStateState()}
		}()
		go func() {
			tfmContext.PermissionChan <- PermissionUpdate{tfContext.Flow.TableName(), tfContext.GetFlowStateState()}
		}()
		tfContext.Init = false
	} else if tfContext.GetFlowStateState() == 2 {
		tfmContext.Log("Flow ready for use: "+tfContext.Flow.TableName(), nil)
	} else {
		tfmContext.Log("Unexpected flow state: "+tfContext.Flow.TableName(), nil)
	}

	go tfmContext.seedVaultCycle(tfContext, identityColumnNames, indexColumnNames, getIndexedPathExt, flowPushRemote, sqlState)
}

func (tfmContext *TrcFlowMachineContext) SelectFlowChannel(tcflowContext flowcore.FlowContext) <-chan interface{} {
	tfContext := tcflowContext.(*TrcFlowContext)
	if notificationFlowChannel, ok := tfmContext.ChannelMap[tfContext.Flow]; ok {
		return notificationFlowChannel.Ch
	}
	tfmContext.Log("Could not find channel for flow.", nil)

	return nil
}

func (tfmContext *TrcFlowMachineContext) GetAuthExtended(getExtensionAuthComponents func(config map[string]interface{}) map[string]interface{}, refresh bool) (map[string]interface{}, error) {
	if tfmContext.ExtensionAuthData != nil && !refresh {
		return tfmContext.ExtensionAuthData, nil
	}
	var authErr error
	driverConfig := tfmContext.ExtensionAuthDataReloader["config"].(*config.DriverConfig)
	extensionAuthComponents := getExtensionAuthComponents(tfmContext.ExtensionAuthDataReloader["identityConfig"].(map[string]interface{}))
	httpClient, err := helperkv.CreateHTTPClient(false, extensionAuthComponents["authDomain"].(string), driverConfig.CoreConfig.Env, false)
	if httpClient != nil {
		defer httpClient.CloseIdleConnections()
	}
	if err != nil {
		eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
		return nil, err
	}
	tfmContext.ExtensionAuthData, _, authErr = util.GetJSONFromClientByPost(driverConfig.CoreConfig, httpClient, extensionAuthComponents["authHeaders"].(map[string]string), extensionAuthComponents["authUrl"].(string), extensionAuthComponents["bodyData"].(io.Reader))

	return tfmContext.ExtensionAuthData, authErr
}

// Make a call on Call back to insert or update using the provided query.
// If this is expected to result in a change to an existing table, thern trigger
// something to the changed channel.
func (tfmContext *TrcFlowMachineContext) CallDBQuery(tcflowContext flowcore.FlowContext,
	queryMap map[string]interface{},
	bindingsI map[string]interface{}, // Optional param
	changed bool,
	operation string,
	flowNotifications []flowcore.FlowNameType, // On successful completion, which flows to notify.
	flowtestState string) ([][]interface{}, bool) {

	tfContext := tcflowContext.(*TrcFlowContext)

	if queryMap["TrcQuery"].(string) == "" {
		return nil, false
	}
	switch operation {
	case "INSERT":
		var matrix [][]interface{}
		var err error
		if bindingsI == nil {
			_, _, matrix, err = trcdb.Query(tfmContext.TierceronEngine, queryMap["TrcQuery"].(string), tfContext.QueryLock)
			if len(matrix) == 0 {
				changed = false
			}
		} else {
			bindings := convertUntypedExpressionMap(bindingsI)
			if bindings == nil {
				return nil, false
			}

			tableName, _, _, err := trcdb.QueryWithBindings(tfmContext.TierceronEngine, queryMap["TrcQuery"].(string), bindings, tfContext.QueryLock)

			if err == nil && tableName == "ok" {
				changed = true
				matrix = append(matrix, []interface{}{})
			}
		}
		if err != nil {
			tfmContext.Log("query error", err)
		}
		if changed && len(matrix) > 0 {

			// If triggers are ever fixed, this can be removed.
			if changeIdValue, changeIdValueOk := queryMap["TrcChangeId"].(string); changeIdValueOk {
				changeQuery := fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES (:id, current_timestamp())", coreopts.BuildOptions.GetDatabaseName(), tfContext.ChangeFlowName)
				bindings := map[string]sqle.Expression{
					"id": sqlee.NewLiteral(changeIdValue, sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
				}
				_, _, _, err = trcdb.QueryWithBindings(tfmContext.TierceronEngine, changeQuery, bindings, tfContext.QueryLock)
				if err != nil {
					tfmContext.Log("Failed to insert changes for INSERT.", err)
				}
			} else if changeIdValues, changeIdValueOk := queryMap["TrcChangeId"].([]string); changeIdValueOk && len(changeIdValues) == 2 {
				if changeIdCols, changeIdColOk := queryMap["TrcChangeCol"].([]string); changeIdColOk && len(changeIdCols) == 2 {
					changeQuery := fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES (:"+changeIdCols[0]+", :"+changeIdCols[1]+", current_timestamp())", coreopts.BuildOptions.GetDatabaseName(), tfContext.ChangeFlowName)
					bindings := map[string]sqle.Expression{
						changeIdCols[0]: sqlee.NewLiteral(changeIdValues[0], sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
						changeIdCols[1]: sqlee.NewLiteral(changeIdValues[1], sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
					}
					_, _, _, err = trcdb.QueryWithBindings(tfmContext.TierceronEngine, changeQuery, bindings, tfContext.QueryLock)
					if err != nil {
						tfmContext.Log("Failed to insert changes for INSERT - 2A.", err)
					}
				} else {
					tfmContext.Log("Failed to find changed column Ids for INSERT - 2A", err)
				}
			} else if changeIdValues, changeIdValueOk := queryMap["TrcChangeId"].([]string); changeIdValueOk && len(changeIdValues) == 3 {
				changeQuery := fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES (:"+flowcoreopts.DataflowTestNameColumn+", :"+flowcoreopts.DataflowTestIdColumn+", :"+flowcoreopts.DataflowTestStateCodeColumn+", current_timestamp())", coreopts.BuildOptions.GetDatabaseName(), "DataFlowStatistics_Changes")
				bindings := map[string]sqle.Expression{
					flowcoreopts.DataflowTestNameColumn:      sqlee.NewLiteral(changeIdValues[0], sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
					flowcoreopts.DataflowTestIdColumn:        sqlee.NewLiteral(changeIdValues[1], sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
					flowcoreopts.DataflowTestStateCodeColumn: sqlee.NewLiteral(changeIdValues[2], sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
				}
				_, _, _, err = trcdb.QueryWithBindings(tfmContext.TierceronEngine, changeQuery, bindings, tfContext.QueryLock)
				if err != nil {
					tfmContext.Log("Failed to insert dfs changes for INSERT.", err)
				}
			}

			if len(flowNotifications) > 0 {
				// look up channels and notify them too.
				for _, flowNotification := range flowNotifications {
					if notificationFlowChannel, ok := tfmContext.ChannelMap[flowcore.FlowNameType(flowNotification)]; ok {
						notificationFlowChannel.Bcast(true)
					}
				}
			}
			// If this is a test...  Also inject notifications appropriately.
			if flowtestState != "" {
				additionalTestFlows := tfmContext.GetAdditionalFlowsByState(flowtestState)
				for _, flowNotification := range additionalTestFlows {
					if notificationFlowChannel, ok := tfmContext.ChannelMap[flowNotification]; ok {
						notificationFlowChannel.Bcast(true)
					}
				}
			}
		}
	case "DELETE":
		fallthrough
	case "UPDATE":
		var tableName string
		var matrix [][]interface{}
		var err error

		if bindingsI == nil {
			tableName, _, matrix, err = trcdb.Query(tfmContext.TierceronEngine, queryMap["TrcQuery"].(string), tfContext.QueryLock)
			if err == nil && tableName == "ok" {
				changed = true
				matrix = append(matrix, []interface{}{})
			} else if len(matrix) == 0 {
				changed = false
			}
		} else {
			bindings := convertUntypedExpressionMap(bindingsI)
			if bindings == nil {
				return nil, false
			}

			tableName, _, _, err = trcdb.QueryWithBindings(tfmContext.TierceronEngine, queryMap["TrcQuery"].(string), bindings, tfContext.QueryLock)

			if err == nil && tableName == "ok" {
				changed = true
				matrix = append(matrix, []interface{}{})
				tfmContext.Log("UPDATE successful.", nil)
			} else {
				tfmContext.Log("UPDATE failed.", nil)
			}
		}

		if err != nil {
			tfmContext.Log("query update error", err)
		}
		if changed && (len(matrix) > 0 || tableName != "") {
			// If triggers are ever fixed, this can be removed.
			if changeIdValue, changeIdValueOk := queryMap["TrcChangeId"].(string); changeIdValueOk {
				var changeQuery string
				if strings.Contains(tfContext.ChangeFlowName, flowcorehelper.TierceronFlowConfigurationTableName) {
					changeQuery = fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES (:id, current_timestamp())", "FlumeDatabase", tfContext.ChangeFlowName)
				} else {
					changeQuery = fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES (:id, current_timestamp())", coreopts.BuildOptions.GetDatabaseName(), tfContext.ChangeFlowName)
				}
				bindings := map[string]sqle.Expression{
					"id": sqlee.NewLiteral(changeIdValue, sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
				}
				_, _, _, err = trcdb.QueryWithBindings(tfmContext.TierceronEngine, changeQuery, bindings, tfContext.QueryLock)
				if err != nil {
					tfmContext.Log("Failed to insert changes for UPDATE.", err)
				}
			} else if changeIdValues, changeIdValueOk := queryMap["TrcChangeId"].([]string); changeIdValueOk && len(changeIdValues) == 2 {
				if changeIdCols, changeIdColOk := queryMap["TrcChangeCol"].([]string); changeIdColOk && len(changeIdCols) == 2 {
					changeQuery := fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES (:"+changeIdCols[0]+", :"+changeIdCols[1]+", current_timestamp())", coreopts.BuildOptions.GetDatabaseName(), tfContext.ChangeFlowName)
					bindings := map[string]sqle.Expression{
						changeIdCols[0]: sqlee.NewLiteral(changeIdValues[0], sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
						changeIdCols[1]: sqlee.NewLiteral(changeIdValues[1], sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
					}
					_, _, _, err = trcdb.QueryWithBindings(tfmContext.TierceronEngine, changeQuery, bindings, tfContext.QueryLock)
					if err != nil {
						tfmContext.Log("Failed to insert changes for UPDATE - 2A.", err)
					}
				} else {
					tfmContext.Log("Failed to find changed column Ids for UPDATE - 2A", err)
				}
			} else if changeIdValues, changeIdValueOk := queryMap["TrcChangeId"].([]string); changeIdValueOk && len(changeIdValues) == 3 {
				changeQuery := fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES (:"+flowcoreopts.DataflowTestNameColumn+", :"+flowcoreopts.DataflowTestIdColumn+", :"+flowcoreopts.DataflowTestStateCodeColumn+", current_timestamp())", coreopts.BuildOptions.GetDatabaseName(), "DataFlowStatistics_Changes")
				bindings := map[string]sqle.Expression{
					flowcoreopts.DataflowTestNameColumn:      sqlee.NewLiteral(changeIdValues[0], sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
					flowcoreopts.DataflowTestIdColumn:        sqlee.NewLiteral(changeIdValues[1], sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
					flowcoreopts.DataflowTestStateCodeColumn: sqlee.NewLiteral(changeIdValues[2], sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
				}
				_, _, _, err = trcdb.QueryWithBindings(tfmContext.TierceronEngine, changeQuery, bindings, tfContext.QueryLock)
				if err != nil {
					tfmContext.Log("Failed to insert dfs changes for UPDATE.", err)
				}
			}

			if len(flowNotifications) > 0 {
				// look up channels and notify them too.
				for _, flowNotification := range flowNotifications {
					if notificationFlowChannel, ok := tfmContext.ChannelMap[flowcore.FlowNameType(flowNotification)]; ok {
						notificationFlowChannel.Bcast(true)
					}
				}
			}
			// If this is a test...  Also inject notifications appropriately.
			if flowtestState != "" {
				additionalTestFlows := tfmContext.GetAdditionalFlowsByState(flowtestState)
				for _, flowNotification := range additionalTestFlows {
					if notificationFlowChannel, ok := tfmContext.ChannelMap[flowNotification]; ok {
						notificationFlowChannel.Bcast(true)
					}
				}
			}
		}
	case "SELECT":
		_, _, matrixChangedEntries, err := trcdb.Query(tfmContext.TierceronEngine, queryMap["TrcQuery"].(string), tfContext.QueryLock)
		if err != nil {
			tfmContext.Log("query select error", err)
		}
		return matrixChangedEntries, changed
	}
	return nil, changed
}

func convertUntypedExpressionMap(bindingsI map[string]interface{}) map[string]sqle.Expression {
	bindings := make(map[string]sqle.Expression, len(bindingsI))
	for k, v := range bindingsI {
		// Assert to MyType
		if typedVal, ok := v.(sqle.Expression); ok {
			bindings[k] = typedVal
		} else {
			return nil
		}
	}
	return bindings
}

// Open a database connection to the provided source using provided
// source configurations.
func (tfmContext *TrcFlowMachineContext) GetDbConn(tcflowContext flowcore.FlowContext,
	dbUrl string,
	username string,
	sourceDBConfig map[string]interface{}) (interface{}, error) {
	tfContext := tcflowContext.(*TrcFlowContext)
	return trcdbutil.OpenDirectConnection(tfmContext.DriverConfig, tfContext.GoMod, dbUrl,
		username,
		func() (string, error) {
			return coreopts.BuildOptions.DecryptSecretConfig(sourceDBConfig, sourceDatabaseConnectionsMap[tfContext.RemoteDataSource["dbsourceregion"].(string)])
		})
}

func (tfmContext *TrcFlowMachineContext) GetCacheRefreshSqlConn(tcflowContext flowcore.FlowContext, region string) (interface{}, error) {
	tfContext := tcflowContext.(*TrcFlowContext)
	sqlConn := tfContext.RemoteDataSource["connection"].(*sql.DB)
	if sqlConn == nil {
		// dbsourceConn, err := trcdbutil.OpenDirectConnection(tfmContext.DriverConfig, tfContext.GoMod, regionSource["dbsourceurl"].(string), regionSource["dbsourceuser"].(string),
		// 	func() (string, error) {
		// 		if _, ok := regionSource["dbsourcepassword"].(string); ok {
		// 			return regionSource["dbsourcepassword"].(string), nil
		// 		} else {
		// 			return "", errors.New("missing password")
		// 		}
		// 	})
		// if err != nil {
		// 	return nil, err
		// }
		// regionSource["connection"] = dbsourceConn
	}
	return sqlConn, nil
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
		return trcvutils.GetJSONFromClientByGet(tfmContext.DriverConfig.CoreConfig, httpClient, apiAuthHeaders, apiEndpoint, bodyData)
	}
	return trcvutils.GetJSONFromClientByPost(tfmContext.DriverConfig.CoreConfig, httpClient, apiAuthHeaders, apiEndpoint, bodyData)
}

func (tfmContext *TrcFlowMachineContext) SetEncryptionSecret() {
	xencrypt.SetEncryptionSecret(tfmContext.DriverConfig)
}

func (tfmContext *TrcFlowMachineContext) ProcessFlow(
	tcflowContext flowcore.FlowContext,
	processFlowController func(tfmContext flowcore.FlowMachineContext, tfContext flowcore.FlowContext) error,
	vaultDatabaseConfig map[string]interface{}, // TODO: actually use this to set up a mysql facade.
	sourceDatabaseConnectionsMap map[string]map[string]interface{},
	flow flowcore.FlowNameType,
	flowType flowcore.FlowType) error {

	tfContext := tcflowContext.(*TrcFlowContext)

	// 	i. Init engine
	//     a. Get project, service, and table config template name.
	if flowType == TableSyncFlow {
		tfContext.ChangeFlowName = tfContext.Flow.TableName() + "_Changes"
		// 3. Create a base seed template for use in vault seed process.
		var baseTableTemplate extract.TemplateResultData
		trcvutils.LoadBaseTemplate(tfmContext.DriverConfig, &baseTableTemplate, tfContext.GoMod, tfContext.FlowSource, tfContext.Flow.ServiceName(), tfContext.FlowPath)
		tfContext.FlowData = &baseTableTemplate
	} else {
		// Use the flow name directly.
		tfContext.FlowSource = flow.TableName()
	}

	for _, sDC := range sourceDatabaseConnectionsMap {
		if _, ok := sDC["dbingestinterval"]; ok {
			tfContext.RemoteDataSource["dbingestinterval"] = sDC["dbingestinterval"]
		} else {
			var d time.Duration = 60000
			tfContext.RemoteDataSource["dbingestinterval"] = d
		}
		//if mysql.IsMysqlPullEnabled() || mysql.IsMysqlPushEnabled() { //Flag is now replaced by syncMode in controller
		// Create remote data source with only what is needed.
		if flow.TableName() != flowcorehelper.TierceronFlowConfigurationTableName {
			if region, ok := sDC["dbsourceregion"].(string); ok {
				tfContext.RemoteDataSource["region-"+region] = sDC
				if _, ok := sDC["dbsourceurl"].(string); ok {
					retryCount := 0
					tfmContext.LogInfo("Obtaining resource connections for : " + flow.TableName() + "-" + region)
				retryConnectionAccess:
					dbsourceConn, err := trcdbutil.OpenDirectConnection(tfmContext.DriverConfig, tfContext.GoMod, sDC["dbsourceurl"].(string), sDC["dbsourceuser"].(string), func() (string, error) { return sDC["dbsourcepassword"].(string), nil })
					if err != nil && err.Error() != "incorrect URL format" {
						if retryCount < 3 && err != nil && dbsourceConn == nil {
							retryCount = retryCount + 1
							goto retryConnectionAccess
						}
					}

					if err != nil {
						tfmContext.Log("Couldn't get dedicated database connection.  Sync modes will fail for "+sDC["dbsourceregion"].(string)+".", err)
						tfmContext.Log("Couldn't get dedicated database connection: "+err.Error(), err)
					} else {
						defer dbsourceConn.Close()
					}
					tfmContext.LogInfo("Obtained resource connection for : " + flow.TableName() + "-" + region)
					tfContext.RemoteDataSource["region-"+region].(map[string]interface{})["connection"] = dbsourceConn
					tfContext.DataSourceRegions = append(tfContext.DataSourceRegions, region)

					if region == "west" { //Sets west as default connection for non-region controlled flows.
						tfContext.RemoteDataSource["connection"] = dbsourceConn
						tfContext.RemoteDataSource["dbsourceregion"] = region
					}
				}
			}
		}
	}

	if initConfigWG, ok := tfContext.RemoteDataSource["controllerInitWG"].(*sync.WaitGroup); ok {
		tfmContext.FlowControllerUpdateLock.Lock()
		if initConfigWG != nil {
			initConfigWG.Done()
		}
		tfmContext.FlowControllerUpdateLock.Unlock()
	}
	//
	//
	// Hand processing off to process flow implementor.
	//
	flowError := processFlowController(tfmContext, tfContext)
	if flowError != nil {
		tfmContext.Log(flowError.Error(), flowError)
	}

	return nil
}

func (tfmContext *TrcFlowMachineContext) SetPermissionUpdate(tcFlowContext flowcore.FlowContext) {
	tfContext := tcFlowContext.(*TrcFlowContext)
	if tfmContext.PermissionChan != nil {
		tfmContext.PermissionChan <- PermissionUpdate{tfContext.Flow.TableName(), tfContext.GetFlowStateState()}
	}
}

func (tfmContext *TrcFlowMachineContext) PathToTableRowHelper(tcflowContext flowcore.FlowContext) ([]interface{}, error) {
	tfContext := tcflowContext.(*TrcFlowContext)
	dataMap, readErr := tfContext.GoMod.ReadData(tfContext.GoMod.SectionPath)
	if readErr != nil {
		return nil, readErr
	}

	rowDataMap := make(map[string]string, 1)
	for columnName, columnData := range dataMap {
		if dataString, ok := columnData.(string); ok {
			rowDataMap[columnName] = dataString
		} else {
			if columnData != nil { //Cover non strings if possible.
				rowDataMap[columnName] = fmt.Sprintf("%v", columnData)
				continue
			}
			return nil, errors.New("Found data that was not a string - unable to write columnName: " + columnName + " to " + tfContext.Flow.TableName())
		}
	}
	row := tfmContext.writeToTableHelper(tfContext, nil, rowDataMap)

	if row != nil {
		return row, nil
	}
	return nil, nil
}

func (tfmContext *TrcFlowMachineContext) DeliverTheStatistic(
	tcflowContext flowcore.FlowContext,
	dfs *tccore.TTDINode,
	id string,
	indexPath string,
	idName string,
	vaultWriteBack bool) {
	tfContext := tcflowContext.(*TrcFlowContext)
	DeliverStatistic(tfmContext, tfContext, tfContext.GoMod, dfs, id, indexPath, idName, tfContext.Logger, vaultWriteBack)
}

func (tfmContext *TrcFlowMachineContext) LoadBaseTemplate(
	tcflowContext flowcore.FlowContext,
) (flowcore.TemplateData, error) {
	tfContext := tcflowContext.(*TrcFlowContext)
	var baseTableTemplate extract.TemplateResultData
	loadTemplateError := trcvutils.LoadBaseTemplate(tfmContext.DriverConfig, &baseTableTemplate, tfContext.GoMod, tfContext.FlowSource, tfContext.Flow.ServiceName(), tfContext.FlowPath)
	return &baseTableTemplate, loadTemplateError
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
		tableSchema := sqle.NewPrimaryKeySchema([]*sqle.Column{})

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
			column := sqle.Column{Name: columnKey, Type: sqle.Text, Source: tfContext.Flow.TableName()}
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
			if tcopts.BuildOptions.CheckIncomingColumnName(column.Name) && secretValue != "<Enter Secret Here>" && secretValue != "" {
				flowSource := tfContext.FlowSourceAlias
				if flowSource == "" {
					flowSource = tfContext.FlowSource
				}
				decodedValue, secretValue, lmQuery, lm, incomingValErr := tcopts.BuildOptions.CheckFlowDataIncoming(secretColumns, secretValue, flowSource, tfContext.Flow.TableName())
				if incomingValErr != nil {
					tfmContext.Log("error checking incoming data flow", incomingValErr)
					continue
				}
				if lmQuery != "" {
					rows, _ := tfmContext.CallDBQuery(tfContext, map[string]interface{}{"TrcQuery": lmQuery}, nil, true, "SELECT", nil, "") //Query to alert change channel
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
			if tcopts.BuildOptions.CheckIncomingAliasColumnName(column.Name) { //Specific case for controller
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

func (tfmContext *TrcFlowMachineContext) Log(msg string, err error) {
	if err != nil {
		eUtils.LogMessageErrorObject(tfmContext.DriverConfig.CoreConfig, msg, err, false)
	} else {
		eUtils.LogInfo(tfmContext.DriverConfig.CoreConfig, msg)
	}
}

func (tfmContext *TrcFlowMachineContext) LogInfo(msg string) {
	eUtils.LogInfo(tfmContext.DriverConfig.CoreConfig, msg)
}

func (tfmContext *TrcFlowMachineContext) GetLogger() *log.Logger {
	if tfmContext.DriverConfig != nil {
		return tfmContext.DriverConfig.CoreConfig.Log
	}
	return nil
}

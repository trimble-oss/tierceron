package core

import (
	"database/sql"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"tierceron/trcvault/util"
	"tierceron/trcx/db"
	"tierceron/trcx/extract"
	sys "tierceron/vaulthelper/system"

	eUtils "tierceron/utils"
	helperkv "tierceron/vaulthelper/kv"

	configcore "VaultConfig.Bootstrap/configcore"
	sqle "github.com/dolthub/go-mysql-server/sql"
)

type FlowType int64
type FlowNameType string

var channelMap map[FlowNameType]chan bool
var signalChannel chan os.Signal
var sourceDatabaseConnectionsMap map[string]map[string]interface{}

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

type TrcFlowMachineContext struct {
	Region            string
	Env               string
	Config            *eUtils.DriverConfig
	Vault             *sys.Vault
	TierceronEngine   *db.TierceronEngine
	ExtensionAuthData map[string]interface{}
}

type TrcFlowContext struct {
	RemoteDataSource map[string]interface{}
	GoMod            *helperkv.Modifier
	Vault            *sys.Vault
	FlowSource       string       // The name of the flow source identified by project.
	FlowSourceAlias  string       // May be a database name
	Flow             FlowNameType // May be a table name.
	FlowPath         string
	FlowData         interface{}
	ChangeFlowName   string // Change flow table name.
}

var tableCreationLock sync.Mutex

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

	for _, tableName := range tableNames {
		changeTableName := tableName + "_Changes"

		if _, ok, _ := tfmContext.TierceronEngine.Database.GetTableInsensitive(tfmContext.TierceronEngine.Context, changeTableName); !ok {
			eUtils.LogInfo(tfmContext.Config, "Creating tierceron sql table: "+changeTableName)
			tableCreationLock.Lock()
			err := tfmContext.TierceronEngine.Database.CreateTable(tfmContext.TierceronEngine.Context, changeTableName, sqle.NewPrimaryKeySchema(sqle.Schema{
				{Name: "id", Type: sqle.Text, Source: changeTableName, PrimaryKey: true},
				{Name: "updateTime", Type: sqle.Timestamp, Source: changeTableName},
			}))
			tableCreationLock.Unlock()
			if err != nil {
				eUtils.LogErrorObject(tfmContext.Config, err, false)
				return err
			}
		}
	}
	eUtils.LogInfo(tfmContext.Config, "Tables creation completed.")

	channelMap = make(map[FlowNameType]chan bool)

	for _, table := range tableNames {
		channelMap[FlowNameType(table)] = make(chan bool, 5)
	}

	for _, f := range additionalFlowNames {
		channelMap[f] = make(chan bool, 5)
	}

	for _, f := range testFlowNames {
		channelMap[f] = make(chan bool, 5)
	}
	return nil
}

func (tfmContext *TrcFlowMachineContext) AddTableSchema(tableSchema sqle.PrimaryKeySchema, tableName string) {
	// Create table if necessary.
	tableCreationLock.Lock()
	if _, ok, _ := tfmContext.TierceronEngine.Database.GetTableInsensitive(tfmContext.TierceronEngine.Context, tableName); !ok {
		//	ii. Init database and tables in local mysql engine instance.
		err := tfmContext.TierceronEngine.Database.CreateTable(tfmContext.TierceronEngine.Context, tableName, tableSchema)
		if err != nil {
			tfmContext.Log("Could not create table.", err)
		}
	}
	tableCreationLock.Unlock()
}

// Set up call back to enable a trigger to track
// whenever a row in a table changes...
func (tfmContext *TrcFlowMachineContext) CreateTableTriggers(trcfc *TrcFlowContext, identityColumnName string) {
	//Create triggers
	var updTrigger sqle.TriggerDefinition
	var insTrigger sqle.TriggerDefinition
	insTrigger.Name = "tcInsertTrigger_" + trcfc.Flow.TableName()
	updTrigger.Name = "tcUpdateTrigger_" + trcfc.Flow.TableName()
	//Prevent duplicate triggers from existing
	existingTriggers, err := tfmContext.TierceronEngine.Database.GetTriggers(tfmContext.TierceronEngine.Context)
	if err != nil {
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
}

func (tfmContext *TrcFlowMachineContext) GetFlowConfiguration(trcfc *TrcFlowContext, flowTemplatePath string) (map[string]interface{}, bool) {
	flowProject, flowService, flowConfigTemplatePath := eUtils.GetProjectService(flowTemplatePath)
	flowConfigTemplateName := eUtils.GetTemplateFileName(flowConfigTemplatePath, flowService)
	trcfc.GoMod.SectionKey = "/Restricted/"
	trcfc.GoMod.SectionName = flowService
	trcfc.GoMod.SubSectionValue = flowService
	properties, err := util.NewProperties(tfmContext.Config, tfmContext.Vault, trcfc.GoMod, tfmContext.Env, flowProject, flowService)
	if err != nil {
		return nil, false
	}
	return properties.GetConfigValues(flowService, flowConfigTemplateName)
}

func (tfmContext *TrcFlowMachineContext) seedVaultCycle(tfContext *TrcFlowContext,
	identityColumnName string,
	vaultIndexColumnName string,
	getIndexedPathExt func(engine interface{}, rowDataMap map[string]interface{}, vaultIndexColumnName string, databaseName string, tableName string, dbCallBack func(interface{}, string) (string, []string, [][]string, error)) (string, error),
	flowPushRemote func(map[string]interface{}, map[string]interface{}) error) {

	afterTime := time.Duration(time.Second * 20)
	isInit := true
	flowChangedChannel := channelMap[tfContext.Flow]

	for {
		select {
		case <-signalChannel:
			eUtils.LogErrorMessage(tfmContext.Config, "Receiving shutdown presumably from vault.", true)
			os.Exit(0)
		case <-flowChangedChannel:
			tfmContext.seedVaultFromChanges(
				tfContext,
				identityColumnName,
				vaultIndexColumnName,
				false,
				getIndexedPathExt,
				flowPushRemote)
		case <-time.After(afterTime):
			afterTime = time.Minute * 3
			eUtils.LogInfo(tfmContext.Config, "3 minutes... checking local mysql for changes.")
			tfmContext.seedVaultFromChanges(
				tfContext,
				identityColumnName,
				vaultIndexColumnName,
				isInit,
				getIndexedPathExt,
				flowPushRemote)
			isInit = false
		}
	}
}

func (tfmContext *TrcFlowMachineContext) seedTrcDbCycle(tfContext *TrcFlowContext,
	identityColumnName string,
	vaultIndexColumnName string,
	getIndexedPathExt func(engine interface{}, rowDataMap map[string]interface{}, vaultIndexColumnName string, databaseName string, tableName string, dbCallBack func(interface{}, string) (string, []string, [][]string, error)) (string, error),
	flowPushRemote func(map[string]interface{}, map[string]interface{}) error) {

	afterTime := time.Duration(time.Second * 20)
	isInit := true
	flowChangedChannel := channelMap[tfContext.Flow]

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
			eUtils.LogInfo(tfmContext.Config, "3 minutes... checking local mysql for changes.")
			tfmContext.seedTrcDbFromChanges(
				tfContext,
				identityColumnName,
				vaultIndexColumnName,
				isInit,
				getIndexedPathExt,
				flowPushRemote)
			isInit = false
		}
	}
}

func (tfmContext *TrcFlowMachineContext) SyncTableCycle(tfContext *TrcFlowContext,
	identityColumnName string,
	vaultIndexColumnName string,
	getIndexedPathExt func(engine interface{}, rowDataMap map[string]interface{}, vaultIndexColumnName string, databaseName string, tableName string, dbCallBack func(interface{}, string) (string, []string, [][]string, error)) (string, error),
	flowPushRemote func(map[string]interface{}, map[string]interface{}) error) {

	go tfmContext.seedTrcDbCycle(tfContext, identityColumnName, vaultIndexColumnName, getIndexedPathExt, flowPushRemote)

	go tfmContext.seedVaultCycle(tfContext, identityColumnName, vaultIndexColumnName, getIndexedPathExt, flowPushRemote)
}

func (tfmContext *TrcFlowMachineContext) SelectFlowChannel(tfContext *TrcFlowContext) <-chan bool {
	if notificationFlowChannel, ok := channelMap[tfContext.Flow]; ok {
		return notificationFlowChannel
	}
	tfmContext.Log("Could not find channel for flow.", nil)

	return nil
}

// Make a call on Call back to insert or update using the provided query.
// If this is expected to result in a change to an existing table, thern trigger
// something to the changed channel.
func (tfmContext *TrcFlowMachineContext) CallDBQuery(tfContext *TrcFlowContext, query string, changed bool, operation string, flowNotifications []FlowNameType) [][]string {
	var changedChannel chan bool

	if changed {
		changedChannel = channelMap[FlowNameType(tfContext.Flow.TableName())]
	}

	if operation == "INSERT" {
		_, _, matrix, err := db.Query(tfmContext.TierceronEngine, query)
		if err != nil {
			eUtils.LogErrorObject(tfmContext.Config, err, false)
		}
		if changed && len(matrix) > 0 {
			if changedChannel != nil {
				changedChannel <- true
			}
			if len(flowNotifications) > 0 {
				// look up channels and notify them too.
				for _, flowNotification := range flowNotifications {
					if notificationFlowChannel, ok := channelMap[flowNotification]; ok {
						notificationFlowChannel <- true
					}
				}
			}
		}
	} else if operation == "UPDATE" {
		tableName, _, matrix, err := db.Query(tfmContext.TierceronEngine, query)
		if err != nil {
			eUtils.LogErrorObject(tfmContext.Config, err, false)
		}
		if changed && (len(matrix) > 0 || tableName != "") {
			if changedChannel != nil {
				changedChannel <- true
			}
			if len(flowNotifications) > 0 {
				// look up channels and notify them too.
				for _, flowNotification := range flowNotifications {
					if notificationFlowChannel, ok := channelMap[flowNotification]; ok {
						notificationFlowChannel <- true
					}
				}
			}
		}
	} else if operation == "SELECT" {
		_, _, matrixChangedEntries, err := db.Query(tfmContext.TierceronEngine, query)
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
	return util.OpenDirectConnection(tfmContext.Config, dbUrl,
		username,
		configcore.DecryptSecretConfig(sourceDBConfig, sourceDatabaseConnectionsMap[tfContext.RemoteDataSource["dbsourceregion"].(string)]))
}

// Utilizing provided api auth headers, endpoint, and body data
// this CB makes a call on behalf of the caller and returns a map
// representation of json data provided by the endpoint.
func (tfmContext *TrcFlowMachineContext) CallAPI(apiAuthHeaders map[string]string, host string, apiEndpoint string, bodyData io.Reader, getOrPost bool) (map[string]interface{}, error) {
	httpClient, err := helperkv.CreateHTTPClient(false, host, tfmContext.Env, false)
	if err != nil {
		return nil, err
	}
	if getOrPost {
		return util.GetJSONFromClientByGet(tfmContext.Config, httpClient, apiAuthHeaders, apiEndpoint, bodyData)
	}
	return util.GetJSONFromClientByPost(tfmContext.Config, httpClient, apiAuthHeaders, apiEndpoint, bodyData)
}

func (tfmContext *TrcFlowMachineContext) Log(msg string, err error) {
	if err != nil {
		eUtils.LogErrorObject(tfmContext.Config, err, false)
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

		// 3. Create a base seed template for use in vault seed process.
		var baseTableTemplate extract.TemplateResultData
		util.LoadBaseTemplate(config, &baseTableTemplate, tfContext.GoMod, tfContext.FlowSource, tfContext.Flow.ServiceName(), tfContext.FlowPath)
		tfContext.FlowData = &baseTableTemplate
	} else {
		// Use the flow name directly.
		tfContext.FlowSource = flow.ServiceName()
	}

	// Create remote data source with only what is needed.
	tfContext.RemoteDataSource["dbsourceregion"] = sourceDatabaseConnectionMap["dbsourceregion"]
	tfContext.RemoteDataSource["dbingestinterval"] = sourceDatabaseConnectionMap["dbingestinterval"]

	dbsourceConn, err := util.OpenDirectConnection(config, sourceDatabaseConnectionMap["dbsourceurl"].(string), sourceDatabaseConnectionMap["dbsourceuser"].(string), sourceDatabaseConnectionMap["dbsourcepassword"].(string))

	if err != nil {
		eUtils.LogErrorMessage(config, "Couldn't get dedicated database connection.", false)
		return err
	}
	defer dbsourceConn.Close()

	tfContext.RemoteDataSource["connection"] = dbsourceConn

	//
	// Hand processing off to process flow implementor.
	//
	flowError := processFlowController(tfmContext, tfContext)
	if flowError != nil {
		eUtils.LogErrorObject(config, flowError, true)
	}

	return nil
}

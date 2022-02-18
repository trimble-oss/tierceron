package core

import (
	"database/sql"
	"io"

	"tierceron/trcx/db"

	helperkv "tierceron/vaulthelper/kv"

	sqle "github.com/dolthub/go-mysql-server/sql"
)

type FlowType int64

const (
	TableSyncFlow FlowType = iota
	TableEnrichFlow
)

type TrcFlowMachineContext struct {
	Env                      string
	TierceronEngine          *db.TierceronEngine
	ExtensionAuthData        map[string]interface{}
	CallGetFlowConfiguration func(trcFlowContext *TrcFlowContext, templatePath string) (map[string]interface{}, bool)
	CallAPI                  func(apiAuthHeaders map[string]string, host string, apiEndpoint string, bodyData io.Reader, getOrPost bool) (map[string]interface{}, error)
	CallGetDbConn            func(dbUrl string, username string, sourceDBConfig map[string]interface{}) (*sql.DB, error)
	CallAddTableSchema       func(schema sqle.PrimaryKeySchema, tableName string)
	CallCreateTableTriggers  func(trcFlowContext *TrcFlowContext, identityColumnName string)
	CallDBQuery              func(trcFlowContext *TrcFlowContext, insertQuery string, changed bool, operation string, flowNotifications []string) [][]string
	CallSyncTableCycle       func(trcFlowContext *TrcFlowContext, identityColumnName string, vaultIndexColumnName string, flowPushRemote func(map[string]interface{}, map[string]interface{}) error)
	CallLog                  func(msg string, err error)
}

type TrcFlowContext struct {
	RemoteDataSource map[string]interface{}
	GoMod            *helperkv.Modifier
	FlowSource       string // The name of the flow source identified by project.
	FlowService      string // The name of the flow source identified by service.
	FlowSourceAlias  string // May be a database name
	FlowName         string // May be a table name.
	FlowPath         string
	FlowData         interface{}
	ChangeFlowName   string // Change flow table name.
}

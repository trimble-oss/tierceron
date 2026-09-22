package core

import (
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	trcdb "github.com/trimble-oss/tierceron/atrium/trcdb"

	sqle "github.com/dolthub/go-mysql-server/sql"
	sqlee "github.com/dolthub/go-mysql-server/sql/expression"
	"github.com/dolthub/vitess/go/sqltypes"
	"github.com/trimble-oss/tierceron-nute-core/mashupsdk"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
)

var AskFlumeFlow flowcore.FlowDefinition = flowcore.FlowDefinition{
	FlowHeader: flowcore.FlowHeaderType{Name: "AskFlumeFlow", Instances: "*"},
}

var (
	signalChannel                chan os.Signal
	sourceDatabaseConnectionsMap map[string]map[string]any
	tfmContextMap                = make(map[string]*TrcFlowMachineContext, 5)
)

const (
	TableSyncFlow flowcore.FlowType = iota
	TableEnrichFlow
	TableTestFlow

	ChangeTypeColumnName = "changeType"
	ChangeTypeInsert     = "insert"
	ChangeTypeUpdate     = "update"
	ChangeTypeDelete     = "delete"
)

func getChangeTrigger(databaseName string, tableName string, idColumnNames []string, rowPrefix string, changeType string) string {
	changeColumnNames := idColumnNames
	if len(idColumnNames) == 1 {
		changeColumnNames = []string{"id"}
	}
	values := make([]string, len(idColumnNames))
	for index, columnName := range idColumnNames {
		values[index] = rowPrefix + "." + columnName
	}
	return ` INSERT INTO ` + databaseName + `.` + tableName + `_Changes (` + strings.Join(changeColumnNames, ",") + `,` + ChangeTypeColumnName + `,updateTime) VALUES (` + strings.Join(values, ",") + `,'` + changeType + `',current_timestamp())` +
		` ON DUPLICATE KEY UPDATE ` + ChangeTypeColumnName + `=VALUES(` + ChangeTypeColumnName + `),updateTime=VALUES(updateTime);`
}

func getUpdateTrigger(databaseName string, tableName string, idColumnNames []string) string {
	if len(idColumnNames) < 1 || len(idColumnNames) > 3 {
		return ""
	}
	return `CREATE TRIGGER tcUpdateTrigger_` + tableName + ` AFTER UPDATE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW BEGIN` +
		getChangeTrigger(databaseName, tableName, idColumnNames, "new", ChangeTypeUpdate) + ` END;`
}

func getInsertTrigger(databaseName string, tableName string, idColumnNames []string) string {
	if len(idColumnNames) < 1 || len(idColumnNames) > 3 {
		return ""
	}
	return `CREATE TRIGGER tcInsertTrigger_` + tableName + ` AFTER INSERT ON ` + databaseName + `.` + tableName + ` FOR EACH ROW BEGIN` +
		getChangeTrigger(databaseName, tableName, idColumnNames, "new", ChangeTypeInsert) + ` END;`
}

func getDeleteTrigger(databaseName string, tableName string, idColumnNames []string) string {
	if len(idColumnNames) < 1 || len(idColumnNames) > 3 {
		return ""
	}
	return `CREATE TRIGGER tcDeleteTrigger_` + tableName + ` AFTER DELETE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW BEGIN` +
		getChangeTrigger(databaseName, tableName, idColumnNames, "old", ChangeTypeDelete) + ` END;`
}

func TriggerChangeChannel(table string) {
	for _, tfmContext := range tfmContextMap {
		if notificationFlowChannel, notificationChannelOk := tfmContext.ChannelMap[flowcore.FlowNameType(table)]; notificationChannelOk {
			notificationFlowChannel.Bcast(true)
			return
		}
	}
}

func TriggerAllChangeChannel(tfmContext *TrcFlowMachineContext, table string, changeIds map[string]string) {
	// If changIds identified, manually trigger a change.
	if table != "" {
		for changeIdKey, changeIDValue := range changeIds {
			tfmContext.FlowMapLock.RLock()
			if tfContext, tfContextOk := tfmContext.FlowMap[flowcore.FlowNameType(table)]; tfContextOk {
				tfmContext.FlowMapLock.RUnlock()
				if slices.Contains(tfContext.ChangeIdKeys, changeIdKey) {
					changeQuery := getManualChangeUpsertQuery(tfContext.FlowHeader.SourceAlias, tfContext.ChangeFlowName, []string{"id"}, ChangeTypeUpdate)
					bindings := map[string]sqle.Expression{
						"id": sqlee.NewLiteral(changeIDValue, sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
					}
					_, _, _, _ = trcdb.QueryWithBindings(tfmContext.TierceronEngine, changeQuery, bindings, tfContext.QueryLock)
					break
				}
			} else {
				tfmContext.FlowMapLock.RUnlock()
			}
		}
		if notificationFlowChannel, notificationChannelOk := tfmContext.ChannelMap[flowcore.FlowNameType(table)]; notificationChannelOk {
			// Notify the affected flow that a change has occured.
			notificationFlowChannel.Bcast(true)
			return
		}
	}
}

type PermissionUpdate struct {
	TableName    string
	CurrentState int64
}

type FakeDFStat struct {
	mashupsdk.MashupDetailedElement
	ChildNodes []FakeDFStat
}

var tableModifierLock sync.Mutex

// WhichLastModified - true if a time was most recent, false if b time was most recent.
func WhichLastModified(a any, b any) bool {
	// Check if a & b are time.time
	// Check if they match.
	var lastModifiedA time.Time
	var lastModifiedB time.Time
	var timeErr error
	if lastMA, ok := a.(time.Time); !ok {
		if lmA, ok := a.(string); ok {
			lastModifiedA, timeErr = time.Parse(tccore.RFC_ISO_8601, lmA)
			if timeErr != nil {
				return false
			}
		}
	} else {
		lastModifiedA = lastMA
	}

	if lastMB, ok := b.(time.Time); !ok {
		if lmB, ok := b.(string); ok {
			lastModifiedB, timeErr = time.Parse(tccore.RFC_ISO_8601, lmB)
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

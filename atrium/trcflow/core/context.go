package core

import (
	"fmt"
	"os"
	"slices"
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

var DataFlowStatConfigurationsFlow flowcore.FlowNameType = "DataFlowStatistics"
var ArgosSociiFlow flowcore.FlowNameType = "ArgosSocii"
var AskFlumeFlow flowcore.FlowNameType = "AskFlumeFlow"

var signalChannel chan os.Signal
var sourceDatabaseConnectionsMap map[string]map[string]interface{}
var tfmContextMap = make(map[string]*TrcFlowMachineContext, 5)

const (
	TableSyncFlow flowcore.FlowType = iota
	TableEnrichFlow
	TableTestFlow
)

func getUpdateTrigger(databaseName string, tableName string, idColumnNames []string) string {
	if len(idColumnNames) == 1 {
		return `CREATE TRIGGER tcUpdateTrigger_` + tableName + `  AFTER UPDATE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
			` BEGIN` +
			` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + idColumnNames[0] + `, current_timestamp());` +
			` END;`
	} else if len(idColumnNames) == 2 {
		return `CREATE TRIGGER tcUpdateTrigger_` + tableName + `  AFTER UPDATE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
			` BEGIN` +
			` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + idColumnNames[0] + `, new.` + idColumnNames[1] + `, current_timestamp());` +
			` END;`
	} else if len(idColumnNames) == 3 {
		return `CREATE TRIGGER tcUpdateTrigger_` + tableName + `  AFTER UPDATE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
			` BEGIN` +
			` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + idColumnNames[0] + `, new.` + idColumnNames[1] + `, new.` + idColumnNames[2] + `, current_timestamp());` +
			` END;`
	} else {
		return ""
	}
}

func getInsertTrigger(databaseName string, tableName string, idColumnNames []string) string {
	if len(idColumnNames) == 1 {
		return `CREATE TRIGGER tcInsertTrigger_` + tableName + ` AFTER INSERT ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
			` BEGIN` +
			` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + idColumnNames[0] + `, current_timestamp());` +
			` END;`
	} else if len(idColumnNames) == 2 {
		return `CREATE TRIGGER tcInsertTrigger_` + tableName + ` AFTER INSERT ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
			` BEGIN` +
			` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + idColumnNames[0] + `, new.` + idColumnNames[1] + `, current_timestamp());` +
			` END;`
	} else if len(idColumnNames) == 3 {
		return `CREATE TRIGGER tcInsertTrigger_` + tableName + `  AFTER INSERT ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
			` BEGIN` +
			` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (new.` + idColumnNames[0] + `, new.` + idColumnNames[1] + `, new.` + idColumnNames[2] + `, current_timestamp());` +
			` END;`
	} else {
		return ""
	}
}

func getDeleteTrigger(databaseName string, tableName string, idColumnNames []string) string {
	if len(idColumnNames) == 1 {
		return `CREATE TRIGGER tcDeleteTrigger_` + tableName + `  AFTER DELETE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
			` BEGIN` +
			` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (old.` + idColumnNames[0] + `, current_timestamp());` +
			` END;`
	} else if len(idColumnNames) == 2 {
		return `CREATE TRIGGER tcDeleteTrigger_` + tableName + `  AFTER DELETE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
			` BEGIN` +
			` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (old.` + idColumnNames[0] + `, old.` + idColumnNames[1] + `, current_timestamp());` +
			` END;`
	} else if len(idColumnNames) == 3 {
		return `CREATE TRIGGER tcDeleteTrigger_` + tableName + `  AFTER DELETE ON ` + databaseName + `.` + tableName + ` FOR EACH ROW` +
			` BEGIN` +
			` INSERT IGNORE INTO ` + databaseName + `.` + tableName + `_Changes VALUES (old.` + idColumnNames[0] + `, old.` + idColumnNames[1] + `, old.` + idColumnNames[2] + `, current_timestamp());` +
			` END;`
	} else {
		return ""
	}
}

func TriggerChangeChannel(table string) {
	for _, tfmContext := range tfmContextMap {
		if notificationFlowChannel, notificationChannelOk := tfmContext.ChannelMap[flowcore.FlowNameType(table)]; notificationChannelOk {
			notificationFlowChannel.Bcast(true)
			return
		}
	}
}

func TriggerAllChangeChannel(table string, changeIds map[string]string) {
	for _, tfmContext := range tfmContextMap {
		// If changIds identified, manually trigger a change.
		if table != "" {
			for changeIdKey, changeIdValue := range changeIds {
				if tfContext, tfContextOk := tfmContext.FlowMap[flowcore.FlowNameType(table)]; tfContextOk {
					if slices.Contains(tfContext.ChangeIdKeys, changeIdKey) {
						changeQuery := fmt.Sprintf("INSERT IGNORE INTO %s.%s VALUES (:id, current_timestamp())", tfContext.FlowSourceAlias, tfContext.ChangeFlowName)
						bindings := map[string]sqle.Expression{
							"id": sqlee.NewLiteral(changeIdValue, sqle.MustCreateStringWithDefaults(sqltypes.VarChar, 200)),
						}
						_, _, _, _ = trcdb.QueryWithBindings(tfmContext.TierceronEngine, changeQuery, bindings, tfContext.QueryLock)
						break
					}
				}
			}
			if notificationFlowChannel, notificationChannelOk := tfmContext.ChannelMap[flowcore.FlowNameType(table)]; notificationChannelOk {
				notificationFlowChannel.Bcast(true)
				continue
			}
		}

		for _, notificationFlowChannel := range tfmContext.ChannelMap {
			notificationFlowChannel.Bcast(true)
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

// True if a time was most recent, false if b time was most recent.
func WhichLastModified(a interface{}, b interface{}) bool {
	//Check if a & b are time.time
	//Check if they match.
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

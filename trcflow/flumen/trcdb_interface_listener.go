package flumen

import (
	"log"
	"strings"
	"sync"

	flowcore "github.com/trimble-oss/tierceron/trcflow/core"

	"github.com/dolthub/vitess/go/vt/sqlparser"
	ast "github.com/dolthub/vitess/go/vt/sqlparser"

	"time"
)

var changeLock sync.Mutex

type TrcDBServerEventListener struct {
	Log *log.Logger
}

func (t *TrcDBServerEventListener) ClientConnected() {}

func (tl *TrcDBServerEventListener) ClientDisconnected() {}

func (tl *TrcDBServerEventListener) QueryStarted() {}

func (tl *TrcDBServerEventListener) QueryCompleted(query string, success bool, duration time.Duration) {
	if success && (strings.HasPrefix(strings.ToLower(query), "replace") || strings.HasPrefix(strings.ToLower(query), "insert") || strings.HasPrefix(strings.ToLower(query), "update")) {
		// TODO: one could implement exactly which flows to notify based on the query.
		//
		// Workaround: Vitess to the rescue.
		// Workaround triggers not firing: 9/30/2022
		//
		tableName := ""
		changeIds := map[string]string{}
		stmt, err := ast.Parse(query)
		if err == nil {
			if sqlInsert, sqlInsertOk := stmt.(*sqlparser.Insert); sqlInsertOk {
				tableName = sqlInsert.Table.Name.String()
				tl.Log.Printf("Query completed: %s %v\n", query, success)
				// Query with bindings may not be deadlock safe.
				// Disable this for now and hope the triggers work.
				// if sqlValues, sqlValuesOk := sqlInsert.Rows.(sqlparser.Values); sqlValuesOk {
				// 	for _, sqlValue := range sqlValues {
				// 		for sqlExprIndex, sqlExpr := range sqlValue {
				// 			if sqlValueIdentity, sqlValueIdentityOk := sqlExpr.(*sqlparser.SQLVal); sqlValueIdentityOk {
				// 				if sqlValueIdentity.Type == sqlparser.StrVal {
				// 					columnName := sqlInsert.Columns[sqlExprIndex].String()
				// 					changeIds[columnName] = string(sqlValueIdentity.Val)
				// 				}
				// 			}
				// 		}
				// 	}
				// }
			} else if sqlUpdate, sqlUpdateOk := stmt.(*sqlparser.Update); sqlUpdateOk {
				for _, tableExpr := range sqlUpdate.TableExprs {
					if aliasTableExpr, aliasTableExprOk := tableExpr.(*sqlparser.AliasedTableExpr); aliasTableExprOk {
						if tableNameType, tableNameTypeOk := aliasTableExpr.Expr.(sqlparser.TableName); tableNameTypeOk {
							tableName = tableNameType.Name.String()
							tl.Log.Printf("Query completed: %s %v\n", query, success)
							break
						}
					}
				}
				// TODO: grab changeId for updates as well.
			}
		}

		changeLock.Lock()
		flowcore.TriggerAllChangeChannel(tableName, changeIds)
		changeLock.Unlock()
	}
}

package flumen

import (
	"log"
	"strings"
	"sync"

	flowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"

	"github.com/dolthub/go-mysql-server/server"
	"github.com/dolthub/vitess/go/vt/sqlparser"
	ast "github.com/dolthub/vitess/go/vt/sqlparser"

	"time"
)

var changeLock sync.Mutex

type TrcDBServerEventListener struct {
	Log *log.Logger
}

var _ server.ServerEventListener = (*TrcDBServerEventListener)(nil)

func (t *TrcDBServerEventListener) ClientConnected() {}

func (tl *TrcDBServerEventListener) ClientDisconnected() {}

func (tl *TrcDBServerEventListener) QueryStarted( /* query string */ ) {
	//	if query contains "FOR UPDATE" {
	//		sync.Lock()
	//	}
}

func (tl *TrcDBServerEventListener) QueryCompleted(query string, success bool, duration time.Duration) {
	if success && (strings.HasPrefix(strings.ToLower(query), "replace") || strings.HasPrefix(strings.ToLower(query), "insert") || strings.HasPrefix(strings.ToLower(query), "update") || strings.HasPrefix(strings.ToLower(query), "delete")) {
		// TODO: one could implement exactly which flows to notify based on the query.
		//
		// Workaround: Vitess to the rescue.
		// Workaround triggers not firing: 9/30/2022
		//
		tableName := ""
		changeIds := map[string]string{}
		stmt, err := ast.Parse(query)
		if err == nil {
			if _, isSelect := stmt.(*sqlparser.Select); isSelect {
				//				if query contains "FOR UPDATE" {
				//					sync.Release()
				//				}

				tl.Log.Printf("Query completed: %s %v\n", tableName, success)
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
			} else if sqlInsert, isInsertQuery := stmt.(*sqlparser.Insert); isInsertQuery {
				tableName = sqlInsert.Table.Name.String()
				tl.Log.Printf("Query completed: %s %v\n", tableName, success)
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
			} else if sqlUpdate, isUpdateQuery := stmt.(*sqlparser.Update); isUpdateQuery {
				for _, tableExpr := range sqlUpdate.TableExprs {
					if aliasTableExpr, aliasTableExprOk := tableExpr.(*sqlparser.AliasedTableExpr); aliasTableExprOk {
						if tableNameType, tableNameTypeOk := aliasTableExpr.Expr.(sqlparser.TableName); tableNameTypeOk {
							tableName = tableNameType.Name.String()
							tl.Log.Printf("Query completed: %s %v\n", tableName, success)
							break
						}
					}
				}
				// TODO: grab changeId for updates as well.
			} else if sqlDelete, isDeleteQuery := stmt.(*sqlparser.Delete); isDeleteQuery {
				//Grabbing change Ids for writeback
				//Prevents anything but a single delete for writing back.
				/*
					subQuery := strings.Split(strings.ToLower(query), "where")
					if len(subQuery) == 2 {
						queryParts, parseErr := url.ParseQuery(subQuery[len(subQuery)-1])
						if parseErr != nil {
							tl.Log.Printf("Delete query parser failed: %s %v\n", query, parseErr.Error())
						} else {
							for qKey, qVal := range queryParts {
								if len(qVal) == 1 {
									changeIds[strings.TrimPrefix(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSuffix(qKey, "\""), "\""), "'"), "'")] = strings.TrimPrefix(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSuffix(qVal[0], "\""), "\""), "'"), "'")
								}
							}

						}
					}*/
				for _, tableExpr := range sqlDelete.TableExprs {
					if aliasTableExpr, aliasTableExprOk := tableExpr.(*sqlparser.AliasedTableExpr); aliasTableExprOk {
						if tableNameType, tableNameTypeOk := aliasTableExpr.Expr.(sqlparser.TableName); tableNameTypeOk {
							tableName = tableNameType.Name.String()
							tl.Log.Printf("Query completed: %s %v\n", tableName, success)
							changeLock.Lock()
							flowcore.TriggerChangeChannel(tableName)
							changeLock.Unlock()
							return
						}
					}
				}

			}
		}

		changeLock.Lock()
		flowcore.TriggerAllChangeChannel(tableName, changeIds)
		changeLock.Unlock()
	}
}

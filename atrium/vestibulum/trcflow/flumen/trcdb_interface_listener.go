package flumen

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	trcflowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"

	"github.com/dolthub/go-mysql-server/server"
	"github.com/dolthub/vitess/go/vt/sqlparser"
	ast "github.com/dolthub/vitess/go/vt/sqlparser"
)

var changeLock sync.Mutex

type TrcDBServerEventListener struct {
	TfmContext *trcflowcore.TrcFlowMachineContext
}

var _ server.ServerEventListener = (*TrcDBServerEventListener)(nil)

func (t *TrcDBServerEventListener) ClientConnected() {}

func (tl *TrcDBServerEventListener) ClientDisconnected() {}

func (tl *TrcDBServerEventListener) QueryStarted(query string) {
	if tl == nil || tl.TfmContext == nil {
		return
	}
	if strings.HasPrefix(strings.ToLower(query), "replace") || strings.HasPrefix(strings.ToLower(query), "insert") || strings.HasPrefix(strings.ToLower(query), "update") || strings.HasPrefix(strings.ToLower(query), "delete") {
		// TODO: one could implement exactly which flows to notify based on the query.
		//
		// Workaround: Vitess to the rescue.
		// Workaround triggers not firing: 9/30/2022
		//
		flowName := ""
		stmt, err := ast.Parse(query)
		if err == nil {
			if _, isSelect := stmt.(*sqlparser.Select); isSelect {
				//				if query contains "FOR UPDATE" {
				//					sync.Release()
				//				}

				tl.TfmContext.DriverConfig.CoreConfig.Log.Printf("Query completed: %s %v\n", flowName)
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
				if sqlInsert != nil {
					flowName = sqlInsert.Table.Name.String()
					var queryMask uint64 = 0
					flowID := tl.TfmContext.GetFlowID(flowcore.FlowNameType(flowName))
					if flowID != nil {
						queryMask = queryMask ^ *flowID
					} else {
						flowNameStr := fmt.Sprintf("%v", flowName)
						tl.TfmContext.Log("Could not find flow ID for flow: "+flowNameStr, errors.New("could not find flow ID for flow"))
					}
					tl.TfmContext.BitLock.Lock(queryMask)
				}
			} else if sqlUpdate, isUpdateQuery := stmt.(*sqlparser.Update); isUpdateQuery {
				if sqlUpdate != nil && sqlUpdate.TableExprs != nil {
					var queryMask uint64 = 0
					var flows []string // List of flows used in query
					for _, tableExpr := range sqlUpdate.TableExprs {
						if aliasTableExpr, aliasTableExprOk := tableExpr.(*sqlparser.AliasedTableExpr); aliasTableExprOk {
							if tableNameType, tableNameTypeOk := aliasTableExpr.Expr.(sqlparser.TableName); tableNameTypeOk {
								flowName = tableNameType.Name.String()
								flows = append(flows, flowName)
							}
						}
					}

					for _, flowName := range flows {
						flowID := tl.TfmContext.GetFlowID(flowcore.FlowNameType(flowName))
						if flowID != nil {
							queryMask = queryMask ^ *flowID
						} else {
							flowNameStr := fmt.Sprintf("%v", flowName)
							tl.TfmContext.Log("Could not find flow ID for flow: "+flowNameStr, errors.New("could not find flow ID for flow"))
						}
					}
					tl.TfmContext.BitLock.Lock(queryMask)
				}
				// TODO: grab changeId for updates as well.
			} else if sqlDelete, isDeleteQuery := stmt.(*sqlparser.Delete); isDeleteQuery {
				var queryMask uint64 = 0
				var flows []string // List of flows used in query
				for _, tableExpr := range sqlDelete.TableExprs {
					if aliasTableExpr, aliasTableExprOk := tableExpr.(*sqlparser.AliasedTableExpr); aliasTableExprOk {
						if tableNameType, tableNameTypeOk := aliasTableExpr.Expr.(sqlparser.TableName); tableNameTypeOk {
							flowName = tableNameType.Name.String()
							flows = append(flows, flowName)
						}
					}
				}

				for _, flowName := range flows {
					flowID := tl.TfmContext.GetFlowID(flowcore.FlowNameType(flowName))
					if flowID != nil {
						queryMask = queryMask ^ *flowID
					} else {
						flowNameStr := fmt.Sprintf("%v", flowName)
						tl.TfmContext.Log("Could not find flow ID for flow: "+flowNameStr, errors.New("could not find flow ID for flow"))
					}
				}
				tl.TfmContext.BitLock.Lock(queryMask)
			}
		} else {
			// Log the error but don't panic
			if tl.TfmContext != nil && tl.TfmContext.DriverConfig != nil {
				tl.TfmContext.DriverConfig.CoreConfig.Log.Printf("QueryStarted Failed to parse query: %s, error: %v\n", query, err)
			}
		}
	}
}

func (tl *TrcDBServerEventListener) QueryCompleted(query string, success bool, duration time.Duration) {
	if tl == nil || tl.TfmContext == nil {
		return
	}
	if strings.HasPrefix(strings.ToLower(query), "replace") || strings.HasPrefix(strings.ToLower(query), "insert") || strings.HasPrefix(strings.ToLower(query), "update") || strings.HasPrefix(strings.ToLower(query), "delete") {
		// TODO: one could implement exactly which flows to notify based on the query.
		//
		// Workaround: Vitess to the rescue.
		// Workaround triggers not firing: 9/30/2022
		//
		tableName := ""
		var flows []string // List of flows used in query
		changeIds := map[string]string{}
		stmt, err := ast.Parse(query)
		if err == nil {
			if _, isSelect := stmt.(*sqlparser.Select); isSelect {
				//				if query contains "FOR UPDATE" {
				//					sync.Release()
				//				}

				tl.TfmContext.DriverConfig.CoreConfig.Log.Printf("Query completed: %s %v\n", tableName, success)
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
				if sqlInsert != nil {
					tableName = sqlInsert.Table.Name.String()
					tl.TfmContext.DriverConfig.CoreConfig.Log.Printf("Query completed: %s %v\n", tableName, success)
					flows = append(flows, tableName)
				}
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
				if sqlUpdate != nil && sqlUpdate.TableExprs != nil {
					for _, tableExpr := range sqlUpdate.TableExprs {
						if aliasTableExpr, aliasTableExprOk := tableExpr.(*sqlparser.AliasedTableExpr); aliasTableExprOk {
							if tableNameType, tableNameTypeOk := aliasTableExpr.Expr.(sqlparser.TableName); tableNameTypeOk {
								tableName = tableNameType.Name.String()
								flows = append(flows, tableName)
							}
						}
					}
					tl.TfmContext.DriverConfig.CoreConfig.Log.Printf("Query completed: %v %v\n", flows, success)
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
							tl.TfmContext.DriverConfig.CoreConfig.Log.Printf("Delete query parser failed: %s %v\n", query, parseErr.Error())
						} else {
							for qKey, qVal := range queryParts {
								if len(qVal) == 1 {
									changeIds[strings.TrimPrefix(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSuffix(qKey, "\""), "\""), "'"), "'")] = strings.TrimPrefix(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSuffix(qVal[0], "\""), "\""), "'"), "'")
								}
							}

						}
					}*/
				if success {
					for _, tableExpr := range sqlDelete.TableExprs {
						if aliasTableExpr, aliasTableExprOk := tableExpr.(*sqlparser.AliasedTableExpr); aliasTableExprOk {
							if tableNameType, tableNameTypeOk := aliasTableExpr.Expr.(sqlparser.TableName); tableNameTypeOk {
								tableName = tableNameType.Name.String()
								flows = append(flows, tableName)
							}
						}
					}
				}
				tl.TfmContext.DriverConfig.CoreConfig.Log.Printf("Query completed: %v %v\n", flows, success)

			}
		} else {
			// Log the error but don't panic
			if tl.TfmContext != nil && tl.TfmContext.DriverConfig != nil {
				tl.TfmContext.DriverConfig.CoreConfig.Log.Printf("QueryCompleted Failed to parse query: %s, error: %v\n", query, err)
			}
		}

		if len(flows) > 0 {
			var queryMask uint64 = 0
			for _, flowName := range flows {
				flowID := tl.TfmContext.GetFlowID(flowcore.FlowNameType(flowName))
				if flowID != nil {
					queryMask = queryMask ^ *flowID
				} else {
					flowNameStr := fmt.Sprintf("%v", flowName)
					tl.TfmContext.Log("Could not find flow ID for flow: "+flowNameStr, errors.New("could not find flow ID for flow"))
				}
			}
			tl.TfmContext.BitLock.Unlock(queryMask)
		}

		if success && tableName != "" {
			changeLock.Lock()
			// Main query entry point for changes to any tables... notification follows.
			trcflowcore.TriggerAllChangeChannel(tl.TfmContext, tableName, changeIds)
			changeLock.Unlock()
		}
	}
}

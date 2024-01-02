package flowcoreopts

import (
	sqle "github.com/dolthub/go-mysql-server/sql"
)

const DataflowTestNameColumn = "flowName"
const DataflowTestIdColumn = "argosId"
const DataflowTestStateCodeColumn = "stateCode"

func GetIdColumnType(table string) sqle.Type {
	return sqle.Text
}

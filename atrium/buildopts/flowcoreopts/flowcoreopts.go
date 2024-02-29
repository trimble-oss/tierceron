package flowcoreopts

import (
	sqle "github.com/dolthub/go-mysql-server/sql"
)

const DataflowTestNameColumn = "flowName"
const DataflowTestIdColumn = "argosId"
const DataflowTestStateCodeColumn = "stateCode"

// GetIdColumnType is provided as a custom override to allow users of the TrcDb
// to specify the type of the index column for a given table. Examples of this are
// sqles.Text, sqles.Int64, etc...
func GetIdColumnType(table string) sqle.Type {
	return sqle.Text
}

package flowcoreopts

import (
	sqle "github.com/dolthub/go-mysql-server/sql"
)

const (
	DataflowTestNameColumn      = "flowName"
	DataflowGroupTestNameColumn = "flowGroup"
	DataflowTestIdColumn        = "argosId"
	DataflowTestStateCodeColumn = "stateCode"
)

// GetIdColumnType is provided as a custom override to allow users of the TrcDb
// to specify the type of the index column for a given table. Examples of this are
// sqles.Text, sqles.Int64, etc...
func GetIdColumnType(table string) sqle.Type {
	return sqle.Text
}

// IsCreateTableEnabled - Default implementation returns false
// This can be overridden by setting BuildOptions.IsCreateTableEnabled to a custom function
func IsCreateTableEnabled() bool {
	return false
}

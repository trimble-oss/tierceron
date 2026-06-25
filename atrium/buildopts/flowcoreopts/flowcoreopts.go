package flowcoreopts

import (
	sqle "github.com/dolthub/go-mysql-server/sql"
	coreflowcoreopts "github.com/trimble-oss/tierceron-core/v2/atrium/buildopts/flowcoreopts"
)

const (
	DataflowTestNameColumn      = coreflowcoreopts.DataflowTestNameColumn
	DataflowGroupTestNameColumn = coreflowcoreopts.DataflowGroupTestNameColumn
	DataflowTestIdColumn        = coreflowcoreopts.DataflowTestIdColumn
	DataflowTestStateCodeColumn = coreflowcoreopts.DataflowTestStateCodeColumn
)

// GetIdColumnType is provided as a custom override to allow users of the TrcDb
// to specify the type of the index column for a given table. Examples of this are
// sqles.Text, sqles.Int64, etc...
func GetIdColumnType(table string) any {
	return sqle.Text
}

func IsCreateTableEnabled() bool {
	return coreflowcoreopts.IsCreateTableEnabled()
}

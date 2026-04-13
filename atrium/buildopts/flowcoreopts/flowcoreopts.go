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

func GetIdColumnType(table string) sqle.Type {
	return sqle.Text
}

func IsCreateTableEnabled() bool {
	return coreflowcoreopts.IsCreateTableEnabled()
}

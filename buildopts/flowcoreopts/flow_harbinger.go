//go:build harbinger
// +build harbinger

package flowcoreopts

import (
	harbingerflow "VaultConfig.TenantConfig/util/buildopts/flow"

	sqle "github.com/dolthub/go-mysql-server/sql"
)

func GetIdColumnType(table string) sqle.Type {
	return harbingerflow.GetIdColumnType(table)
}

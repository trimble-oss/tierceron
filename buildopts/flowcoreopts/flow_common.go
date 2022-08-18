//go:build !harbinger
// +build !harbinger

package flowcoreopts

import (
	sqle "github.com/dolthub/go-mysql-server/sql"
)

func GetIdColumnType(table string) sqle.Type {
	return sqle.Text
}

package db

import (
	sqle "github.com/dolthub/go-mysql-server"
	"github.com/dolthub/go-mysql-server/memory"
	"github.com/dolthub/go-mysql-server/sql"
)

type TierceronTable struct {
	Table  *memory.Table
	Schema sql.Schema
}

type TierceronEngine struct {
	Name       string
	Engine     *sqle.Engine
	Context    *sql.Context
	TableCache map[string]*TierceronTable
}

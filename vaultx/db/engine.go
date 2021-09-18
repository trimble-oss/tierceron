package db

import (
	sqle "github.com/dolthub/go-mysql-server"
)

type TierceronEngine struct {
	Name   string
	Engine *sqle.Engine
}

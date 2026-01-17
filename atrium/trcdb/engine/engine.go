package engine

import (
	"github.com/trimble-oss/tierceron/pkg/utils/config"

	sqle "github.com/dolthub/go-mysql-server"
	"github.com/dolthub/go-mysql-server/memory"
	"github.com/dolthub/go-mysql-server/sql"
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
)

type TierceronTable struct {
	Table  *memory.Table
	Schema sql.PrimaryKeySchema
}

type TierceronEngine struct {
	Config     config.DriverConfig
	Database   *memory.Database
	Engine     *sqle.Engine
	Context    *sql.Context
	TableCache map[string]*TierceronTable
	TfmContext flowcore.FlowMachineContext
}

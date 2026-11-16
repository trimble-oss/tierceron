package mysqldbutil

import (
	"database/sql"
	"sync"

	"github.com/trimble-oss/tierceron-core/v2/core"
)

// Map of chatId -> response channel for concurrent requests
var TenantConfigurationChans sync.Map

func OpenIndirectConnection(configContext *core.ConfigContext, tenantID string) (string, string, *sql.DB, error) {
	return "", "", nil, nil
}

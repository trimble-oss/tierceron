package testsuite

import (
	"sync"

	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/kafkatesting"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/mysqldbutil"
)

// Pool - it's a pool
type Pool struct{}

var connectionLock sync.Mutex

func CommonInit() {
	if kafkatesting.IndirectDbFunc == nil {
		connectionLock.Lock()
		kafkatesting.IndirectDbFunc = mysqldbutil.OpenIndirectConnection
		connectionLock.Unlock()
	}
}

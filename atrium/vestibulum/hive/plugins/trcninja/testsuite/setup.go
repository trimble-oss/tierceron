package testsuite

import (
	"sync"

	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/kafka"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/mysqldbutil"
)

// Pool - it's a pool
type Pool struct{}

var connectionLock sync.Mutex

func CommonInit() {
	if kafka.IndirectDbFunc == nil {
		connectionLock.Lock()
		kafka.IndirectDbFunc = mysqldbutil.OpenIndirectConnection
		connectionLock.Unlock()
	}
}

package flumen

import (
	"sync"
	flowcore "tierceron/trcflow/core"
	"time"
)

var changeLock sync.Mutex

type TrcDBServerEventListener struct{}

func (t *TrcDBServerEventListener) ClientConnected() {}

func (tl *TrcDBServerEventListener) ClientDisconnected() {}

func (tl *TrcDBServerEventListener) QueryStarted() {}

func (tl *TrcDBServerEventListener) QueryCompleted(success bool, duration time.Duration) {
	changeLock.Lock()
	flowcore.TriggerAllChangeChannel()
	changeLock.Unlock()
}

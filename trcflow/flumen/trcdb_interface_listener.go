package flumen

import (
	"strings"
	"sync"
	flowcore "tierceron/trcflow/core"
	"time"
)

var changeLock sync.Mutex

type TrcDBServerEventListener struct{}

func (t *TrcDBServerEventListener) ClientConnected() {}

func (tl *TrcDBServerEventListener) ClientDisconnected() {}

func (tl *TrcDBServerEventListener) QueryStarted() {}

func (tl *TrcDBServerEventListener) QueryCompleted(query string, success bool, duration time.Duration) {
	if success && (strings.HasPrefix(strings.ToLower(query), "insert") || strings.HasPrefix(strings.ToLower(query), "update")) {
		changeLock.Lock()
		flowcore.TriggerAllChangeChannel()
		changeLock.Unlock()
	}
}

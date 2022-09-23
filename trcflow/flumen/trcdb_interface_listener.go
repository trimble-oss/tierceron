package flumen

import (
	"log"
	"strings"
	"sync"
	flowcore "tierceron/trcflow/core"
	"time"
)

var changeLock sync.Mutex

type TrcDBServerEventListener struct {
	Log *log.Logger
}

func (t *TrcDBServerEventListener) ClientConnected() {}

func (tl *TrcDBServerEventListener) ClientDisconnected() {}

func (tl *TrcDBServerEventListener) QueryStarted() {}

func (tl *TrcDBServerEventListener) QueryCompleted(query string, success bool, duration time.Duration) {
	if success && (strings.HasPrefix(strings.ToLower(query), "insert") || strings.HasPrefix(strings.ToLower(query), "update")) {
		// TODO: one could implement exactly which flows to notify based on the query.
		changeLock.Lock()
		flowcore.TriggerAllChangeChannel()
		changeLock.Unlock()
	}
}

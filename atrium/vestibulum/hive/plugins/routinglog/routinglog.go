package routinglog

import (
	"log"
	"sync/atomic"
)

var routingLoggingActive atomic.Bool

func SetRoutingLoggingActive(active bool) {
	routingLoggingActive.Store(active)
}

func IsRoutingLoggingActive() bool {
	return routingLoggingActive.Load()
}

func LogRouting(logger *log.Logger, format string, args ...any) {
	if logger != nil && IsRoutingLoggingActive() {
		logger.Printf(format, args...)
	}
}

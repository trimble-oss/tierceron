package common

import (
	"context"
	"sync"

	"github.com/trimble-oss/tierceron-core/v2/core/pluginsync"
)

var localDiagnostics struct {
	sync.RWMutex
	handler func(context.Context, string, string) (string, error)
}

// SetLocalDiagnosticHandler installs the transport's wire-to-kernel adapter.
// It is independent of the transport's outbound connection mode.
func SetLocalDiagnosticHandler(handler func(context.Context, string, string) (string, error)) {
	localDiagnostics.Lock()
	defer localDiagnostics.Unlock()
	localDiagnostics.handler = handler
}

// LocalDiagnosticHandler returns a dispatcher only when every requested plugin
// is registered in this kernel. The kernel still checks each plugin's run state.
func LocalDiagnosticHandler(targets []string) func(context.Context, string, string) (string, error) {
	if len(targets) == 0 {
		return nil
	}
	for _, target := range targets {
		if !pluginsync.IsPluginDeployed(target) {
			return nil
		}
	}
	localDiagnostics.RLock()
	defer localDiagnostics.RUnlock()
	return localDiagnostics.handler
}

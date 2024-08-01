package core

import "log"

// This structure contains core properties central to Secrets engine access
type CoreConfig struct {
	IsShell           bool   // If tool running in shell.
	DynamicPathFilter string // Seeds from a specific path.
	WantCerts         bool
	ExitOnFailure     bool // Exit on a failure or try to continue
	Log               *log.Logger
}

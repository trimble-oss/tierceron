package main

import (
	flowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"
	askflume "github.com/trimble-oss/tierceron/atrium/trcflow/core/askflume"
)

// Tests the flume side of AskFlume but will not be able to query trcdb
func main() {
	tfmContext := &flowcore.TrcFlowMachineContext{}
	trcflowContext := &flowcore.TrcFlowContext{}

	askflume.ProcessAskFlumeController(tfmContext, trcflowContext)
}

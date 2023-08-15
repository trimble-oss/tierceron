package main

import (
	flowcore "github.com/trimble-oss/tierceron/trcflow/core"
	askflumeserver "github.com/trimble-oss/tierceron/trcflow/core/askflumeserver"
)

func main() {
	tfmContext := &flowcore.TrcFlowMachineContext{}
	trcflowContext := &flowcore.TrcFlowContext{}

	askflumeserver.ProcessAskFlumeController(tfmContext, trcflowContext)
}

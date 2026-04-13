package flowopts

import (
	coreflowopts "github.com/trimble-oss/tierceron-core/v2/atrium/buildopts/flowopts"
	trcflowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"
)

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	*coreflowopts.OptionsBuilder
	ProcessAskFlumeEventMapper func(askFlumeContext *trcflowcore.AskFlumeContext, query *trcflowcore.AskFlumeMessage, tfmContext *trcflowcore.TrcFlowMachineContext, tfContext *trcflowcore.TrcFlowContext) *trcflowcore.AskFlumeMessage
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.AllowTrcdbInterfaceOverride = AllowTrcdbInterfaceOverride
		optionsBuilder.GetAdditionalFlows = GetAdditionalFlows
		optionsBuilder.GetAdditionalTestFlows = GetAdditionalTestFlows
		optionsBuilder.GetAdditionalFlowsByState = GetAdditionalFlowsByState
		optionsBuilder.ProcessTestFlowController = ProcessTestFlowController
		optionsBuilder.ProcessFlowController = ProcessFlowController
		optionsBuilder.GetFlowMachineTemplates = GetFlowMachineTemplates
		optionsBuilder.ProcessAskFlumeEventMapper = ProcessAskFlumeEventMapper
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	BuildOptions = &OptionsBuilder{OptionsBuilder: &coreflowopts.OptionsBuilder{}}
	for _, opt := range opts {
		opt(BuildOptions)
	}
	coreflowopts.BuildOptions = BuildOptions.OptionsBuilder
}

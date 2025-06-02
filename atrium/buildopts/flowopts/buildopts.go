package flowopts

import (
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	trcflowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"
)

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	// Flow
	AllowTrcdbInterfaceOverride func() bool
	GetAdditionalFlows          func() []flowcore.FlowNameType
	GetAdditionalTestFlows      func() []flowcore.FlowNameType
	GetAdditionalFlowsByState   func(string) []flowcore.FlowNameType
	ProcessTestFlowController   func(tfmContext flowcore.FlowMachineContext, tfContext flowcore.FlowContext) error
	ProcessFlowController       func(tfmContext flowcore.FlowMachineContext, tfContext flowcore.FlowContext) error
	GetFlowDatabaseName         func() string
	GetFlowMachineTemplates     func() map[string]interface{}
	ProcessAskFlumeEventMapper  func(askFlumeContext *trcflowcore.AskFlumeContext, query *trcflowcore.AskFlumeMessage, tfmContext *trcflowcore.TrcFlowMachineContext, tfContext *trcflowcore.TrcFlowContext) *trcflowcore.AskFlumeMessage
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.AllowTrcdbInterfaceOverride = AllowTrcdbInterfaceOverride
		optionsBuilder.GetAdditionalFlows = GetAdditionalFlows
		optionsBuilder.GetAdditionalTestFlows = GetAdditionalTestFlows
		optionsBuilder.GetAdditionalFlowsByState = GetAdditionalFlowsByState
		optionsBuilder.ProcessTestFlowController = ProcessTestFlowController
		optionsBuilder.ProcessFlowController = ProcessFlowController
		optionsBuilder.GetFlowDatabaseName = GetFlowDatabaseName
		optionsBuilder.GetFlowMachineTemplates = GetFlowMachineTemplates
		optionsBuilder.ProcessAskFlumeEventMapper = ProcessAskFlumeEventMapper
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	BuildOptions = &OptionsBuilder{}
	for _, opt := range opts {
		opt(BuildOptions)
	}
}

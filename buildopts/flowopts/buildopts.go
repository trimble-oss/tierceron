package flowopts

import flowcore "github.com/trimble-oss/tierceron/trcflow/core"

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	// Flow
	GetAdditionalFlows         func() []flowcore.FlowNameType
	GetAdditionalTestFlows     func() []flowcore.FlowNameType
	GetAdditionalFlowsByState  func(string) []flowcore.FlowNameType
	ProcessTestFlowController  func(tfmContext *flowcore.TrcFlowMachineContext, trcFlowContext *flowcore.TrcFlowContext) error
	ProcessFlowController      func(tfmContext *flowcore.TrcFlowMachineContext, trcFlowContext *flowcore.TrcFlowContext) error
	GetFlowDatabaseName        func() string
	ProcessAskFlumeEventMapper func(askFlumeContext *flowcore.AskFlumeContext, query *flowcore.AskFlumeMessage, tfmContext *flowcore.TrcFlowMachineContext, tfContext *flowcore.TrcFlowContext) *flowcore.AskFlumeMessage
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.GetAdditionalFlows = GetAdditionalFlows
		optionsBuilder.GetAdditionalTestFlows = GetAdditionalTestFlows
		optionsBuilder.GetAdditionalFlowsByState = GetAdditionalFlowsByState
		optionsBuilder.ProcessTestFlowController = ProcessTestFlowController
		optionsBuilder.ProcessFlowController = ProcessFlowController
		optionsBuilder.GetFlowDatabaseName = GetFlowDatabaseName
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

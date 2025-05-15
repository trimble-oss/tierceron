package testopts

import (
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
)

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	GetAdditionalTestFlows    func() []flowcore.FlowNameType
	GetAdditionalFlowsByState func(teststate string) []flowcore.FlowNameType
	ProcessTestFlowController func(tfmContext flowcore.FlowMachineContext, tfContext flowcore.FlowContext) error
	GetTestConfig             func(tokenPtr *string, wantPluginPaths bool) map[string]interface{}
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.GetAdditionalTestFlows = GetAdditionalTestFlows
		optionsBuilder.GetAdditionalFlowsByState = GetAdditionalFlowsByState
		optionsBuilder.ProcessTestFlowController = ProcessTestFlowController
		optionsBuilder.GetTestConfig = GetTestConfig
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	BuildOptions = &OptionsBuilder{}
	for _, opt := range opts {
		opt(BuildOptions)
	}
}

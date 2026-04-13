package testopts

import coretestopts "github.com/trimble-oss/tierceron-core/v2/atrium/buildopts/testopts"

type Option = coretestopts.Option

type OptionsBuilder = coretestopts.OptionsBuilder

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
	coretestopts.NewOptionsBuilder(opts...)
	BuildOptions = coretestopts.BuildOptions
}

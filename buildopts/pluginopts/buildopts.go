package pluginopts

import "github.com/trimble-oss/tierceron-core/v2/flow"

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	GetPluginMessages         func(string) []string
	GetConfigPaths            func(string) []string
	GetFlowMachineInitContext func(string) *flow.FlowMachineInitContext
	Init                      func(string, *map[string]interface{})
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.GetConfigPaths = GetConfigPaths
		optionsBuilder.GetFlowMachineInitContext = GetFlowMachineInitContext
		optionsBuilder.Init = Init
		optionsBuilder.GetPluginMessages = GetPluginMessages
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	BuildOptions = &OptionsBuilder{}
	for _, opt := range opts {
		opt(BuildOptions)
	}
}

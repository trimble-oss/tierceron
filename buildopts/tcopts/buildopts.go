package tcopts

import coretcopts "github.com/trimble-oss/tierceron-core/v2/buildopts/tcopts"

type Option = coretcopts.Option

type OptionsBuilder = coretcopts.OptionsBuilder

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.CheckIncomingColumnName = CheckIncomingColumnName
		optionsBuilder.CheckFlowDataIncoming = CheckFlowDataIncoming
		optionsBuilder.CheckIncomingAliasColumnName = CheckIncomingAliasColumnName
		optionsBuilder.GetTrcDbUrl = GetTrcDbUrl
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	coretcopts.NewOptionsBuilder(opts...)
	BuildOptions = coretcopts.BuildOptions
}

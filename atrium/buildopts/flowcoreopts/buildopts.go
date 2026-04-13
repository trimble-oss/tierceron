package flowcoreopts

import coreflowcoreopts "github.com/trimble-oss/tierceron-core/v2/atrium/buildopts/flowcoreopts"

type Option = coreflowcoreopts.Option

type OptionsBuilder = coreflowcoreopts.OptionsBuilder

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.GetIdColumnType = func(table string) any {
			return GetIdColumnType(table)
		}
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	coreflowcoreopts.NewOptionsBuilder(opts...)
	BuildOptions = coreflowcoreopts.BuildOptions
}

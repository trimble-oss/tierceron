package deployopts

import coredeployopts "github.com/trimble-oss/tierceron-core/v2/buildopts/deployopts"

type Option = coredeployopts.Option

type OptionsBuilder = coredeployopts.OptionsBuilder

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.InitSupportedDeployers = InitSupportedDeployers
		optionsBuilder.GetDecodedDeployerId = GetDecodedDeployerId
		optionsBuilder.GetEncodedDeployerId = GetEncodedDeployerId
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	coredeployopts.NewOptionsBuilder(opts...)
	BuildOptions = coredeployopts.BuildOptions
}

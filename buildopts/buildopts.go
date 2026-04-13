package buildopts

import corebuildopts "github.com/trimble-oss/tierceron-core/v2/buildopts"

type Option = corebuildopts.Option

type OptionsBuilder = corebuildopts.OptionsBuilder

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.SetLogger = SetLogger
		optionsBuilder.SetErrorLogger = SetErrorLogger
		optionsBuilder.GetLocalVaultAddr = GetLocalVaultAddr
		optionsBuilder.GetSupportedSourceRegions = GetSupportedSourceRegions
		optionsBuilder.GetTestDeployConfig = GetTestDeployConfig
		optionsBuilder.ProcessPluginEnvConfig = ProcessPluginEnvConfig
		optionsBuilder.GetExtensionAuthComponents = GetExtensionAuthComponents
		optionsBuilder.GetSyncedTables = GetSyncedTables
		optionsBuilder.Authorize = Authorize
		optionsBuilder.CheckMemLock = CheckMemLock
		optionsBuilder.GetTrcDbUrl = GetTrcDbUrl
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	corebuildopts.NewOptionsBuilder(opts...)
	BuildOptions = corebuildopts.BuildOptions
}

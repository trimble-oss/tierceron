package coreopts

import corecoreopts "github.com/trimble-oss/tierceron-core/v2/buildopts/coreopts"

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	*corecoreopts.OptionsBuilder
	GetDefaultDeployments func() string
	GetConfigPaths        func(string) []string
	IsKubeRunnable        func() bool
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.GetFolderPrefix = GetFolderPrefix
		optionsBuilder.GetSupportedTemplates = GetSupportedTemplates
		optionsBuilder.IsLocalEndpoint = IsLocalEndpoint
		optionsBuilder.GetVaultInstallRoot = GetVaultInstallRoot
		optionsBuilder.GetSupportedDomains = GetSupportedDomains
		optionsBuilder.GetSupportedEndpoints = GetSupportedEndpoints
		optionsBuilder.GetLocalHost = GetLocalHost
		optionsBuilder.GetRegionByHost = GetRegionByHost
		optionsBuilder.GetDefaultRegion = GetDefaultRegion
		optionsBuilder.GetVaultHost = GetVaultHost
		optionsBuilder.GetVaultHostPort = GetVaultHostPort
		optionsBuilder.GetUserNameField = GetUserNameField
		optionsBuilder.GetUserCodeField = GetUserCodeField
		optionsBuilder.GetSyncedTables = GetSyncedTables
		optionsBuilder.ActiveSessions = ActiveSessions
		optionsBuilder.IsSupportedFlow = IsSupportedFlow
		optionsBuilder.FindIndexForService = FindIndexForService
		optionsBuilder.DecryptSecretConfig = DecryptSecretConfig
		optionsBuilder.GetDFSPathName = GetDFSPathName
		optionsBuilder.GetDatabaseName = GetDatabaseName
		optionsBuilder.CompareLastModified = CompareLastModified
		optionsBuilder.PreviousStateCheck = PreviousStateCheck
		optionsBuilder.GetMachineID = GetMachineID
		optionsBuilder.GetDefaultDeployments = GetDefaultDeployments
		optionsBuilder.InitPluginConfig = InitPluginConfig
		optionsBuilder.GetPluginRestrictedMappings = GetPluginRestrictedMappings
		optionsBuilder.GetConfigPaths = GetConfigPaths
		optionsBuilder.GetSupportedCertIssuers = GetSupportedCertIssuers
		optionsBuilder.IsKubeRunnable = IsKubeRunnable
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	BuildOptions = &OptionsBuilder{OptionsBuilder: &corecoreopts.OptionsBuilder{}}
	for _, opt := range opts {
		opt(BuildOptions)
	}
	corecoreopts.BuildOptions = BuildOptions.OptionsBuilder
}

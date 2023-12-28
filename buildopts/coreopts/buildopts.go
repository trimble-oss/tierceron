package coreopts

import (
	"database/sql"
)

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	GetFolderPrefix              func(custom []string) string
	GetSupportedTemplates        func(custom []string) []string
	GetSupportedEndpoints        func() []string
	GetLocalHost                 func() string
	GetRegion                    func(hostName string) string
	GetVaultHost                 func() string
	GetVaultHostPort             func() string
	GetUserNameField             func() string
	GetUserCodeField             func() string
	ActiveSessions               func(db *sql.DB) ([]map[string]interface{}, error)
	GetSyncedTables              func() []string
	FindIndexForService          func(project string, service string) (string, []string, string, error)
	ProcessDeployPluginEnvConfig func(pluginEnvConfig map[string]interface{}) map[string]interface{}
	DecryptSecretConfig          func(tenantConfiguration map[string]interface{}, config map[string]interface{}) string
	GetDFSPathName               func() (string, string)
	GetDatabaseName              func() string
	CompareLastModified          func(dfStatMapA map[string]interface{}, dfStatMapB map[string]interface{}) bool
	PreviousStateCheck           func(currentState int) int
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.GetFolderPrefix = GetFolderPrefix
		optionsBuilder.GetSupportedTemplates = GetSupportedTemplates
		optionsBuilder.GetSupportedEndpoints = GetSupportedEndpoints
		optionsBuilder.GetLocalHost = GetLocalHost
		optionsBuilder.GetRegion = GetRegion
		optionsBuilder.GetVaultHost = GetVaultHost
		optionsBuilder.GetVaultHostPort = GetVaultHostPort
		optionsBuilder.GetUserNameField = GetUserNameField
		optionsBuilder.GetUserCodeField = GetUserCodeField
		optionsBuilder.GetSyncedTables = GetSyncedTables
		optionsBuilder.ActiveSessions = ActiveSessions
		optionsBuilder.FindIndexForService = FindIndexForService
		optionsBuilder.ProcessDeployPluginEnvConfig = ProcessDeployPluginEnvConfig
		optionsBuilder.DecryptSecretConfig = DecryptSecretConfig
		optionsBuilder.GetDFSPathName = GetDFSPathName
		optionsBuilder.GetDatabaseName = GetDatabaseName
		optionsBuilder.CompareLastModified = CompareLastModified
		optionsBuilder.PreviousStateCheck = PreviousStateCheck
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	BuildOptions = &OptionsBuilder{}
	for _, opt := range opts {
		opt(BuildOptions)
	}
}

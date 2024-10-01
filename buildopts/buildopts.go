package buildopts

import (
	"database/sql"
)

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	SetLogger      func(logger interface{})
	SetErrorLogger func(logger interface{})

	GetLocalVaultAddr          func() string
	GetSupportedSourceRegions  func() []string
	GetTestDeployConfig        func(tokenPtr *string) map[string]interface{}
	ProcessPluginEnvConfig     func(pluginEnvConfig map[string]interface{}) map[string]interface{}
	GetExtensionAuthComponents func(config map[string]interface{}) map[string]interface{}
	GetSyncedTables            func() []string
	Authorize                  func(db *sql.DB, userIdentifier string, userPassword string) (bool, string, error)
	CheckMemLock               func(bucket string, key string) bool
	GetTrcDbUrl                func(data map[string]interface{}) string
}

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
	BuildOptions = &OptionsBuilder{}
	for _, opt := range opts {
		opt(BuildOptions)
	}
}

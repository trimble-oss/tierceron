package tcopts

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	CheckIncomingColumnName      func(col string) bool
	CheckMysqlFileIncoming       func(secretColumns map[string]string, secretValue string, dbName string, tableName string) ([]byte, string, string, string, error)
	CheckIncomingAliasColumnName func(col string) bool
	GetTrcDbUrl                  func(data map[string]interface{}) string
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.CheckIncomingColumnName = CheckIncomingColumnName
		optionsBuilder.CheckMysqlFileIncoming = CheckMysqlFileIncoming
		optionsBuilder.CheckIncomingAliasColumnName = CheckIncomingAliasColumnName
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

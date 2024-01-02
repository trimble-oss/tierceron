package flowcoreopts

import sqle "github.com/dolthub/go-mysql-server/sql"

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	// Flow Core
	GetIdColumnType func(table string) sqle.Type
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.GetIdColumnType = GetIdColumnType
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	BuildOptions = &OptionsBuilder{}
	for _, opt := range opts {
		opt(BuildOptions)
	}
}

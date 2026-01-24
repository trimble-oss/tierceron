package flowcoreopts

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	// Flow Core
	GetIdColumnType      func(table string) any
	IsCreateTableEnabled func() bool
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.GetIdColumnType = GetIdColumnType
		optionsBuilder.IsCreateTableEnabled = IsCreateTableEnabled
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	BuildOptions = &OptionsBuilder{}
	for _, opt := range opts {
		opt(BuildOptions)
	}
}

// IsCreateTableEnabled - Default implementation returns false
// This can be overridden by setting BuildOptions.IsCreateTableEnabled to a custom function
func IsCreateTableEnabled() bool {
	return false
}

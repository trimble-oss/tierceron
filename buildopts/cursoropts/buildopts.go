package cursoropts

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	TapInit             func()
	GetCapPath          func() string
	GetCapCuratorPath   func() string
	GetPluginName       func(bool) string
	GetLogPath          func() string
	GetTrusts           func() map[string][]string
	GetCursorConfigPath func() string
	GetCursorFields     func() map[string]CursorFieldAttributes
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.GetPluginName = GetPluginName
		optionsBuilder.GetCapPath = GetCapPath
		optionsBuilder.GetCapCuratorPath = GetCapCuratorPath
		optionsBuilder.GetLogPath = GetLogPath
		optionsBuilder.TapInit = TapInit
		optionsBuilder.GetTrusts = GetTrusts
		optionsBuilder.GetCursorConfigPath = GetCursorConfigPath
		optionsBuilder.GetCursorFields = GetCursorFields
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	BuildOptions = &OptionsBuilder{}
	for _, opt := range opts {
		opt(BuildOptions)
	}
}

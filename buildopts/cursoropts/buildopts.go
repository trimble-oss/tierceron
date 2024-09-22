package cursoropts

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	TapInit             func()
	GetCapPath          func() string
	GetPluginName       func() string
	GetLogPath          func() string
	GetTrcshBinPath     func() string
	GetTrcshConfigPath  func() string
	GetCursorConfigPath func() string
	GetCursorFields     func() map[string]string
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.GetPluginName = GetPluginName
		optionsBuilder.GetCapPath = GetCapPath
		optionsBuilder.GetLogPath = GetLogPath
		optionsBuilder.TapInit = TapInit
		optionsBuilder.GetTrcshBinPath = GetTrcshBinPath
		optionsBuilder.GetTrcshConfigPath = GetTrcshConfigPath
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

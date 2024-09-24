package cursoropts

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	GetCuratorConfig    func(pluginEnvConfig map[string]interface{}) map[string]interface{}
	TapInit             func()
	GetCapPath          func() string
	GetPluginName       func() string
	GetLogPath          func() string
	GetTrusts           func() map[string][]string
	GetCursorConfigPath func() string
	GetCursorFields     func() map[string]string
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.GetCuratorConfig = GetCuratorConfig
		optionsBuilder.GetPluginName = GetPluginName
		optionsBuilder.GetCapPath = GetCapPath
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

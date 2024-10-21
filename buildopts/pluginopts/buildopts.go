package pluginopts

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	IsPluginHardwired func() bool
	GetPluginMessages func(string) []string
	GetConfigPaths    func(string) []string
	Init              func(string, *map[string]interface{})
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.IsPluginHardwired = IsPluginHardwired
		optionsBuilder.GetConfigPaths = GetConfigPaths
		optionsBuilder.Init = Init
		optionsBuilder.GetPluginMessages = GetPluginMessages
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	BuildOptions = &OptionsBuilder{}
	for _, opt := range opts {
		opt(BuildOptions)
	}
}

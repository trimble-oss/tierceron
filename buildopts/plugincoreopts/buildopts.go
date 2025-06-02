package plugincoreopts

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	IsPluginHardwired func() bool
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.IsPluginHardwired = IsPluginHardwired
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	BuildOptions = &OptionsBuilder{}
	for _, opt := range opts {
		opt(BuildOptions)
	}
}

func init() {
	NewOptionsBuilder(LoadOptions())
}

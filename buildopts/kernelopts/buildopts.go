package kernelopts

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	IsKernel func() bool
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.IsKernel = IsKernel
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	BuildOptions = &OptionsBuilder{}
	for _, opt := range opts {
		opt(BuildOptions)
	}
}

package saltyopts

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	// Flow Core
	GetSaltyGuardian func() string
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.GetSaltyGuardian = GetSaltyGuardian
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	BuildOptions = &OptionsBuilder{}
	for _, opt := range opts {
		opt(BuildOptions)
	}
}

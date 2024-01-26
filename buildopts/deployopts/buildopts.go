package deployopts

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	// Deploy
	InitSupportedDeployers func(supportedDeployers []string) []string
	GetDecodedDeployerId   func(sessionId string) (string, bool)
	GetEncodedDeployerId   func(deployment string, env string) (string, bool)
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.InitSupportedDeployers = InitSupportedDeployers
		optionsBuilder.GetDecodedDeployerId = GetDecodedDeployerId
		optionsBuilder.GetEncodedDeployerId = GetEncodedDeployerId
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	BuildOptions = &OptionsBuilder{}
	for _, opt := range opts {
		opt(BuildOptions)
	}
}

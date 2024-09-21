package cursoropts

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	TapInit             func()
	IsCursor            func() bool
	GetTrcshBinPath     func() string
	GetTrcshConfigPath  func() string
	GetCursorConfigPath func() string
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.TapInit = TapInit
		optionsBuilder.IsCursor = IsCursor
		optionsBuilder.GetTrcshBinPath = GetTrcshBinPath
		optionsBuilder.GetTrcshConfigPath = GetTrcshConfigPath
		optionsBuilder.GetCursorConfigPath = GetCursorConfigPath
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	BuildOptions = &OptionsBuilder{}
	for _, opt := range opts {
		opt(BuildOptions)
	}
}

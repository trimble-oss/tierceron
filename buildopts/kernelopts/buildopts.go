package kernelopts

import "os"

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	IsKernel  func() bool
	IsKernelZ func() bool
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.IsKernel = IsKernel
		optionsBuilder.IsKernelZ = IsKernelZ
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
	// Initialize BuildOptions first
	NewOptionsBuilder(LoadOptions())

	// Redirect stderr to log file for kernelz mode to keep TUI clean
	if IsKernelZ() {
		logFile := "./trcsh.log"
		f, err := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
		if err == nil {
			os.Stderr = f
		}
	}
}

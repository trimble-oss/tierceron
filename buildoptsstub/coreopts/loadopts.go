package coreopts

import "github.com/trimble-oss/tierceron/buildopts/coreopts"

func LoadOptions() coreopts.Option {
	return func(optionsBuilder *coreopts.OptionsBuilder) {
		optionsBuilder.GetSupportedEndpoints = GetSupportedEndpoints
	}
}

package xencryptopts

import corexencryptopts "github.com/trimble-oss/tierceron-core/v2/buildopts/xencryptopts"

type Option = corexencryptopts.Option

type OptionsBuilder = corexencryptopts.OptionsBuilder

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.SetEncryptionSecret = SetEncryptionSecret
		optionsBuilder.MakeNewEncryption = MakeNewEncryption
		optionsBuilder.Encrypt = Encrypt
		optionsBuilder.Decrypt = Decrypt
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	corexencryptopts.NewOptionsBuilder(opts...)
	BuildOptions = corexencryptopts.BuildOptions
}

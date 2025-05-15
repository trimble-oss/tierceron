package xencryptopts

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	SetEncryptionSecret func(string) error
	MakeNewEncryption   func() (string, string, error)
	Encrypt             func(input string, encryption map[string]interface{}) (string, error)
	Decrypt             func(passStr string, decryption map[string]interface{}) (string, error)
}

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
	BuildOptions = &OptionsBuilder{}
	for _, opt := range opts {
		opt(BuildOptions)
	}
}

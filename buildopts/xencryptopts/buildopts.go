package xencryptopts

import (
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	LoadSecretFromSecretStore func(mod *helperkv.Modifier) (map[string]interface{}, error)
	MakeNewEncryption         func() (string, string, error)
	Encrypt                   func(input string, encryption map[string]interface{}) (string, error)
	Decrypt                   func(passStr string, decryption map[string]interface{}) (string, error)
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.LoadSecretFromSecretStore = LoadSecretFromSecretStore
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

package xencryptopts

import eUtils "github.com/trimble-oss/tierceron/pkg/utils"

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	FieldValidator         func(fields string, secSection map[string]map[string]map[string]string, valSection map[string]map[string]map[string]string) error
	SetEncryptionSecret    func(config *eUtils.DriverConfig) error
	GetEncrpytors          func(secSection map[string]map[string]map[string]string) (map[string]interface{}, error)
	CreateEncrpytedReadMap func(encrypted string) map[string]interface{}
	FieldReader            func(encryptedMap map[string]interface{}, secSection map[string]map[string]map[string]string, valSection map[string]map[string]map[string]string, decryption map[string]interface{}) error
	PromptUserForFields    func(fields string, encrypted string, encryption map[string]interface{}) (map[string]interface{}, map[string]interface{}, error)
	FieldReplacer          func(fieldMap map[string]interface{}, encryptedMap map[string]interface{}, secSection map[string]map[string]map[string]string, valSection map[string]map[string]map[string]string) error
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.FieldValidator = FieldValidator
		optionsBuilder.SetEncryptionSecret = SetEncryptionSecret
		optionsBuilder.GetEncrpytors = GetEncrpytors
		optionsBuilder.CreateEncrpytedReadMap = CreateEncrpytedReadMap
		optionsBuilder.FieldReader = FieldReader
		optionsBuilder.PromptUserForFields = PromptUserForFields
		optionsBuilder.FieldReplacer = FieldReplacer
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	BuildOptions = &OptionsBuilder{}
	for _, opt := range opts {
		opt(BuildOptions)
	}
}

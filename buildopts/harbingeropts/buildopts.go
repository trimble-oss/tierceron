package harbingeropts

import (
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	GetFolderPrefix func(custom []string) string
	GetDatabaseName func() string
	BuildInterface  func(config *eUtils.DriverConfig, goMod *kv.Modifier, tfmContext interface{}, vaultDatabaseConfig map[string]interface{}, serverListener interface{}) error
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.GetFolderPrefix = GetFolderPrefix
		optionsBuilder.GetDatabaseName = GetDatabaseName
		optionsBuilder.BuildInterface = BuildInterface
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	BuildOptions = &OptionsBuilder{}
	for _, opt := range opts {
		opt(BuildOptions)
	}
}

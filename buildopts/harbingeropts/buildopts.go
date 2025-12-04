package harbingeropts

import (
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	GetFolderPrefix    func(custom []string) string
	IsValidProjectName func(projectName string) bool
	BuildInterface     func(flowMachineInitContext *flowcore.FlowMachineInitContext, driverConfig *config.DriverConfig, goMod *kv.Modifier, tfmContext any, vaultDatabaseConfig map[string]any, serverListener any) error
	BuildTableGrant    func(tableName string) (string, error)
	TableGrantNotify   func(tfmContext flowcore.FlowMachineContext, tableName string)
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.GetFolderPrefix = GetFolderPrefix
		optionsBuilder.IsValidProjectName = IsValidProjectName
		optionsBuilder.BuildInterface = BuildInterface
		optionsBuilder.BuildTableGrant = BuildTableGrant
		optionsBuilder.TableGrantNotify = TableGrantNotify
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	BuildOptions = &OptionsBuilder{}
	for _, opt := range opts {
		opt(BuildOptions)
	}
}

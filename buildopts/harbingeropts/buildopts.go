package harbingeropts

import (
	coreharbingeropts "github.com/trimble-oss/tierceron-core/v2/buildopts/harbingeropts"
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

type Option func(*OptionsBuilder)

type OptionsBuilder struct {
	*coreharbingeropts.OptionsBuilder
	BuildInterface func(flowMachineInitContext *flowcore.FlowMachineInitContext, driverConfig *config.DriverConfig, goMod *kv.Modifier, tfmContext any, vaultDatabaseConfig map[string]any, serverListener any) error
}

func LoadOptions() Option {
	return func(optionsBuilder *OptionsBuilder) {
		optionsBuilder.GetIdColumnType = GetIdColumnType
		optionsBuilder.GetFolderPrefix = GetFolderPrefix
		optionsBuilder.IsValidProjectName = IsValidProjectName
		optionsBuilder.BuildInterface = BuildInterface
		optionsBuilder.BuildTableGrant = BuildTableGrant
		optionsBuilder.TableGrantNotify = TableGrantNotify
	}
}

var BuildOptions *OptionsBuilder

func NewOptionsBuilder(opts ...Option) {
	BuildOptions = &OptionsBuilder{OptionsBuilder: &coreharbingeropts.OptionsBuilder{}}
	for _, opt := range opts {
		opt(BuildOptions)
	}
	coreharbingeropts.BuildOptions = BuildOptions.OptionsBuilder
}

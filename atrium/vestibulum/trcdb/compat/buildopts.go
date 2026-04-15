package compat

import (
	"log"

	coreflowcoreopts "github.com/trimble-oss/tierceron-core/v2/atrium/buildopts/flowcoreopts"
	coreflowopts "github.com/trimble-oss/tierceron-core/v2/atrium/buildopts/flowopts"
	coretestopts "github.com/trimble-oss/tierceron-core/v2/atrium/buildopts/testopts"
	corebuildopts "github.com/trimble-oss/tierceron-core/v2/buildopts"
	corecoreopts "github.com/trimble-oss/tierceron-core/v2/buildopts/coreopts"
	coredeployopts "github.com/trimble-oss/tierceron-core/v2/buildopts/deployopts"
	coreharbingeropts "github.com/trimble-oss/tierceron-core/v2/buildopts/harbingeropts"
	coretcopts "github.com/trimble-oss/tierceron-core/v2/buildopts/tcopts"
	corexencryptopts "github.com/trimble-oss/tierceron-core/v2/buildopts/xencryptopts"
	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig/cache"
	"github.com/trimble-oss/tierceron/atrium/buildopts/flowcoreopts"
	"github.com/trimble-oss/tierceron/atrium/buildopts/flowopts"
	"github.com/trimble-oss/tierceron/atrium/buildopts/testopts"
	rootbuildopts "github.com/trimble-oss/tierceron/buildopts"
	rootcoreopts "github.com/trimble-oss/tierceron/buildopts/coreopts"
	rootdeployopts "github.com/trimble-oss/tierceron/buildopts/deployopts"
	rootharbingeropts "github.com/trimble-oss/tierceron/buildopts/harbingeropts"
	roottcopts "github.com/trimble-oss/tierceron/buildopts/tcopts"
	rootxencryptopts "github.com/trimble-oss/tierceron/buildopts/xencryptopts"
	tiercerontls "github.com/trimble-oss/tierceron/pkg/tls"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

type InitOptions struct {
	BuildOption     corebuildopts.Option
	CoreOption      corecoreopts.Option
	DeployOption    coredeployopts.Option
	HarbingerOption coreharbingeropts.Option
	TcOption        coretcopts.Option
	FlowOption      coreflowopts.Option
	FlowCoreOption  coreflowcoreopts.Option
	TestOption      coretestopts.Option
	XencryptOption  corexencryptopts.Option
}

func InitBuildOptions(options InitOptions) {
	if options.BuildOption == nil {
		corebuildopts.NewOptionsBuilder()
	} else {
		corebuildopts.NewOptionsBuilder(options.BuildOption)
	}
	rootbuildopts.BuildOptions = corebuildopts.BuildOptions

	if options.CoreOption == nil {
		corecoreopts.NewOptionsBuilder()
	} else {
		corecoreopts.NewOptionsBuilder(options.CoreOption)
	}
	rootcoreopts.BuildOptions = &rootcoreopts.OptionsBuilder{OptionsBuilder: corecoreopts.BuildOptions}

	if options.DeployOption == nil {
		coredeployopts.NewOptionsBuilder()
	} else {
		coredeployopts.NewOptionsBuilder(options.DeployOption)
	}
	rootdeployopts.BuildOptions = coredeployopts.BuildOptions

	if options.HarbingerOption == nil {
		coreharbingeropts.NewOptionsBuilder()
	} else {
		coreharbingeropts.NewOptionsBuilder(options.HarbingerOption)
	}
	rootharbingeropts.BuildOptions = &rootharbingeropts.OptionsBuilder{
		OptionsBuilder: coreharbingeropts.BuildOptions,
		BuildInterface: rootharbingeropts.BuildInterface,
	}

	if options.TcOption == nil {
		coretcopts.NewOptionsBuilder()
	} else {
		coretcopts.NewOptionsBuilder(options.TcOption)
	}
	roottcopts.BuildOptions = coretcopts.BuildOptions

	if options.FlowOption == nil {
		coreflowopts.NewOptionsBuilder()
	} else {
		coreflowopts.NewOptionsBuilder(options.FlowOption)
	}
	flowopts.BuildOptions = &flowopts.OptionsBuilder{
		OptionsBuilder:             coreflowopts.BuildOptions,
		ProcessAskFlumeEventMapper: flowopts.ProcessAskFlumeEventMapper,
	}

	if options.FlowCoreOption == nil {
		coreflowcoreopts.NewOptionsBuilder()
	} else {
		coreflowcoreopts.NewOptionsBuilder(options.FlowCoreOption)
	}
	flowcoreopts.BuildOptions = coreflowcoreopts.BuildOptions

	if options.TestOption == nil {
		coretestopts.NewOptionsBuilder()
	} else {
		coretestopts.NewOptionsBuilder(options.TestOption)
	}
	testopts.BuildOptions = coretestopts.BuildOptions

	if options.XencryptOption == nil {
		corexencryptopts.NewOptionsBuilder()
	} else {
		corexencryptopts.NewOptionsBuilder(options.XencryptOption)
	}
	rootxencryptopts.BuildOptions = corexencryptopts.BuildOptions
}

func InitRoot() {
	tiercerontls.InitRoot()
}

func InitDriverConfigForPlugin(pluginConfig map[string]any, tokenCache *cache.TokenCache, currentTokenName string, logger *log.Logger) (*config.DriverConfig, error) {
	return eUtils.InitDriverConfigForPlugin(pluginConfig, tokenCache, currentTokenName, logger)
}

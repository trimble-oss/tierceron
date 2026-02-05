package shellcmd

import (
	"fmt"

	"github.com/trimble-oss/tierceron-core/v2/trcshfs/trcshio"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/trcplgtoolbase"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcsh/kube/native"
	"github.com/trimble-oss/tierceron/pkg/capauth"
	"github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcinitbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcpubbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcsubbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcxbase"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

// ExecuteShellCommand executes a shell command based on the command type string from ChatMsg.Response
// Returns the MemoryFileSystem where command output is written
func ExecuteShellCommand(cmdType string, args []string, driverConfig *config.DriverConfig) trcshio.MemoryFileSystem {
	if driverConfig == nil {
		return nil
	}

	if driverConfig.CoreConfig != nil && driverConfig.CoreConfig.Log != nil {
		driverConfig.CoreConfig.Log.Printf("ExecuteShellCommand: cmdType=%s, args=%v, IsShellCommand before=%v\n", cmdType, args, driverConfig.IsShellCommand)
	}

	// Mark that this command is running from trcshcmd
	driverConfig.IsShellCommand = true

	var err error

	// Pull common values from DriverConfig like trcsh.go does
	envDefaultPtr := driverConfig.CoreConfig.EnvBasis
	tokenName := "config_token_" + driverConfig.CoreConfig.EnvBasis
	envCtx := driverConfig.CoreConfig.EnvBasis // Use same as envDefaultPtr
	region := ""
	if len(driverConfig.CoreConfig.Regions) > 0 {
		region = driverConfig.CoreConfig.Regions[0]
	}

	// Prepend command name to args as argLines[0]
	argLines := append([]string{cmdType}, args...)

	switch cmdType {
	case CmdTrcConfig:
		err = trcconfigbase.CommonMain(&envDefaultPtr, &envCtx, &tokenName, &region, nil, argLines, driverConfig)

	case CmdTrcPub:
		pubTokenName := fmt.Sprintf("vault_pub_token_%s", driverConfig.CoreConfig.EnvBasis)
		pubEnv := driverConfig.CoreConfig.Env
		trcpubbase.CommonMain(&pubEnv, &envCtx, &pubTokenName, nil, argLines, driverConfig)

	case CmdTrcSub:
		err = trcsubbase.CommonMain(&envDefaultPtr, &envCtx, &tokenName, nil, argLines, driverConfig)

	case CmdTrcX:
		trcxbase.CommonMain(nil, nil, &envDefaultPtr, nil, &envCtx, nil, nil, argLines, driverConfig)

	case CmdTrcInit:
		pubTokenName := fmt.Sprintf("vault_pub_token_%s", driverConfig.CoreConfig.EnvBasis)
		pubEnv := driverConfig.CoreConfig.Env
		uploadCert := driverConfig.CoreConfig.WantCerts
		trcinitbase.CommonMain(&pubEnv, &envCtx, &pubTokenName, &uploadCert, nil, args, driverConfig)

	case CmdTrcPlgtool:
		env := driverConfig.CoreConfig.Env
		plgTokenName := "config_token_pluginany"
		// Create TrcshDriverConfig wrapper
		trcshDriverConfig := &capauth.TrcshDriverConfig{
			DriverConfig: driverConfig,
		}
		err = trcplgtoolbase.CommonMain(&env, &envCtx, &plgTokenName, &region, nil, args, trcshDriverConfig)

	case CmdKubectl:
		// Initialize kubectl configuration
		trcKubeConfig, kubeErr := native.InitTrcKubeConfig(nil, driverConfig.CoreConfig)
		if kubeErr != nil {
			err = kubeErr
		} else {
			// Execute kubectl command
			err = native.KubeCtl(trcKubeConfig, driverConfig)
		}

	case CmdTrcBoot:
		// Simply return the memFs from driverConfig without executing any commands
		// This is used for initializing plugins that need access to the shared memFs
		if driverConfig != nil && driverConfig.MemFs != nil {
			return driverConfig.MemFs
		}
		return nil

	default:
		// Unknown command type
		return nil
	}

	if err != nil {
		// Error occurred, but MemFs may still have partial output
		if driverConfig.CoreConfig != nil && driverConfig.CoreConfig.Log != nil {
			driverConfig.CoreConfig.Log.Printf("ExecuteShellCommand: command execution error: %v\n", err)
		}
	}

	// Return the MemFs where command wrote its output
	if driverConfig.CoreConfig != nil && driverConfig.CoreConfig.Log != nil {
		driverConfig.CoreConfig.Log.Printf("ExecuteShellCommand: returning MemFs (nil=%v)\n", driverConfig.MemFs == nil)
	}
	return driverConfig.MemFs
}

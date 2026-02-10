package hcore

import (
	"errors"
	"fmt"
	"io"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron/pkg/core/util/hive/plugins/trcshcmd/shellcmd"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

var (
	configContext *tccore.ConfigContext
	driverConfig  *config.DriverConfig
)

// GetConfigContext returns the trcshcmd config context for cross-plugin access
func GetConfigContext() *tccore.ConfigContext {
	return configContext
}

// GetDriverConfig returns the trcshcmd driver config
func GetDriverConfig() *config.DriverConfig {
	return driverConfig
}

// hasUnrestrictedAccess checks if the user has unrestricted write access
// by verifying if trcshunrestricted role credentials are present in TokenCache
func hasUnrestrictedAccess() bool {
	if driverConfig == nil || driverConfig.CoreConfig == nil || driverConfig.CoreConfig.TokenCache == nil {
		return false
	}

	// Check if trcshunrestricted role has credentials
	roleName := "trcshunrestricted"
	appRoleSecret := driverConfig.CoreConfig.TokenCache.GetRoleStr(&roleName)
	if appRoleSecret == nil || len(*appRoleSecret) < 2 {
		return false
	}

	// Verify credentials are non-empty (valid UUID format is 36 chars)
	roleID := (*appRoleSecret)[0]
	secretID := (*appRoleSecret)[1]
	return len(roleID) == 36 && len(secretID) == 36
}

func receiver(receive_chan chan tccore.KernelCmd) {
	for {
		event := <-receive_chan
		switch {
		case event.Command == tccore.PLUGIN_EVENT_START:
			go start(event.PluginName)
		case event.Command == tccore.PLUGIN_EVENT_STOP:
			go stop(event.PluginName)
			if configContext != nil && configContext.ErrorChan != nil {
				*configContext.ErrorChan <- errors.New("trcshcmd shutting down")
			}
			return
		case event.Command == tccore.PLUGIN_EVENT_STATUS:
			// TODO
		default:
			// TODO
		}
	}
}

func chat_receiver(chat_receive_chan chan *tccore.ChatMsg) {
	// Signal that trcshcmd is ready for requests
	for {
		event := <-chat_receive_chan
		switch {
		case event == nil:
			continue
		case event.Name != nil && *event.Name == "SHUTDOWN":
			if configContext != nil {
				configContext.Log.Println("trcshcmd shutting down message receiver")
			}
			return
		case event.ChatId != nil && *event.ChatId != "":
			// Handle shell command requests
			cmdType := *event.ChatId
			if event.Response != nil {
				if *event.Response == "Service initializing" && cmdType != shellcmd.CmdTrcBoot {
					return // Allow trcboot to go through when service unavailable.
				} else if *event.Response == "Service unavailable" {
					// Service is actually unavailable and in error state likely.
					return
				}
			}

			// Check if this is a shell command type
			if cmdType == shellcmd.CmdTrcConfig || cmdType == shellcmd.CmdTrcPub ||
				cmdType == shellcmd.CmdTrcSub || cmdType == shellcmd.CmdTrcX ||
				cmdType == shellcmd.CmdTrcInit || cmdType == shellcmd.CmdTrcPlgtool ||
				cmdType == shellcmd.CmdKubectl || cmdType == shellcmd.CmdTrcBoot ||
				cmdType == shellcmd.CmdRm || cmdType == shellcmd.CmdCp ||
				cmdType == shellcmd.CmdMv || cmdType == shellcmd.CmdCat ||
				cmdType == shellcmd.CmdSu {

				if configContext != nil {
					configContext.Log.Printf("Received shell command request: %s\n", cmdType)
				}

				// Authorization check: Block privileged commands without elevated access
				if (cmdType == shellcmd.CmdTrcX || cmdType == shellcmd.CmdTrcInit || cmdType == shellcmd.CmdTrcPub) && !hasUnrestrictedAccess() {
					configContext.Log.Printf("AUTHORIZATION DENIED: Command %s requires elevated access\n", cmdType)

					// Return authorization error immediately without executing command
					errorMsg := fmt.Sprintf("AUTHORIZATION ERROR: '%s' command requires elevated access. Run 'su' to obtain unrestricted credentials.\n", cmdType)
					event.Response = &errorMsg
					event.HookResponse = nil

					// Send error response back
					if configContext != nil && configContext.ChatSenderChan != nil {
						*configContext.ChatSenderChan <- event
					}
					continue
				}

				// Extract args from HookResponse field
				var args []string
				if event.HookResponse != nil {
					if argsList, ok := event.HookResponse.([]string); ok {
						args = argsList
					}
				}

				// Execute command - output written to MemFs
				result := shellcmd.ExecuteShellCommand(cmdType, args, driverConfig)

				// Read output from io/STDIO if it exists
				responseMsg := "Command completed"
				if result != nil {
					if stdioFile, err := result.Open("io/STDIO"); err == nil {
						defer stdioFile.Close()
						if data, readErr := io.ReadAll(stdioFile); readErr == nil {
							if len(data) > 0 {
								responseMsg = string(data)
							}
							if driverConfig != nil && driverConfig.CoreConfig != nil && driverConfig.CoreConfig.Log != nil {
								driverConfig.CoreConfig.Log.Printf("Read io/STDIO output, length: %d\n", len(data))
							}
						} else {
							if driverConfig != nil && driverConfig.CoreConfig != nil && driverConfig.CoreConfig.Log != nil {
								driverConfig.CoreConfig.Log.Printf("Error reading io/STDIO: %v\n", readErr)
							}
						}
					} else {
						if driverConfig != nil && driverConfig.CoreConfig != nil && driverConfig.CoreConfig.Log != nil {
							driverConfig.CoreConfig.Log.Printf("Error opening io/STDIO: %v\n", err)
						}
					}
				} else {
					if driverConfig != nil && driverConfig.CoreConfig != nil && driverConfig.CoreConfig.Log != nil {
						driverConfig.CoreConfig.Log.Println("ExecuteShellCommand returned nil result")
					}
				}
				event.Response = &responseMsg

				// Only return MemFs in HookResponse for trcboot (initial setup)
				// After that, both plugins share the same MemFs instance
				if cmdType == shellcmd.CmdTrcBoot {
					event.HookResponse = result
				} else {
					event.HookResponse = nil
				}

				// Send response back to requesting plugin
				if configContext != nil && configContext.ChatSenderChan != nil {
					*configContext.ChatSenderChan <- event
				}
			}
		default:
			if configContext != nil {
				configContext.Log.Println("trcshcmd received non-command chat message")
			}
		}
	}
}

func start(pluginName string) {
	if configContext == nil {
		fmt.Println("no config context initialized for trcshcmd")
		return
	}
	// Initiate final plugin startup sequence.
	go func(cmd_send_chan *chan tccore.KernelCmd) {
		if cmd_send_chan != nil {
			*cmd_send_chan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_START}
		}
	}(configContext.CmdSenderChan)

	configContext.Log.Printf("Shell command plugin %s started\n", pluginName)
}

func stop(pluginName string) {
	if configContext == nil {
		return
	}
	configContext.Log.Printf("Shell command plugin %s stopped\n", pluginName)
}

func GetConfigPaths(pluginName string) []string {
	return []string{}
}

func initPlugin(pluginName string, properties *map[string]any) {
	// Check if running in Kubernetes - refuse to initialize
	if isKubernetes, ok := (*properties)["isKubernetes"].(bool); ok && isKubernetes {
		(*properties)["pluginRefused"] = true
		return
	}

	// Get DriverConfig from properties
	if dc, ok := (*properties)["driverConfig"].(*config.DriverConfig); ok {
		driverConfig = dc
		fmt.Printf("trcshcmd initPlugin: received driverConfig=%p\n", driverConfig)
	} else {
		fmt.Printf("trcshcmd initPlugin: no driverConfig in properties\n")
	}

	var err error
	configContext, err = tccore.Init(
		properties,
		"",
		"",
		"",
		"trcshcmd",
		func(s string) {},
		receiver,
		chat_receiver,
	)
	if err != nil {
		fmt.Printf("Failure to init context for trcshcmd: %v\n", err)
		return
	}
}

// Start sends the START command to the trcshcmd plugin
func startPlugin() {
}

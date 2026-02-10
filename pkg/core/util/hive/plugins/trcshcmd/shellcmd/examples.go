package shellcmd

/*
Shell Command Execution via ChatMsg

Plugins communicate with trcshcmd using ChatMsg to execute shell commands.
This follows the same pattern used by other plugins like trcdb and trcshtalk for
inter-plugin communication.

IMPORTANT: The kernel routes messages based on the Query field. To send a message
to trcshcmd, the Query field MUST contain "trcshcmd" as the destination plugin name.

Pattern:
1. Plugin sends ChatMsg with:
   - Name field: source plugin name (e.g., "trcsh")
   - Query field: destination plugin name []string{"trcshcmd"}
   - Response field: command type string (e.g., "trcconfig", "trcpub", "trcboot", etc.)
   - HookResponse field: command arguments as []string for args, or KubectlCommand for kubectl

2. Kernel routes the message to trcshcmd plugin's chat_receiver

3. trcshcmd executes the command and sends response back via ChatMsg with:
   - Response field: status message
   - HookResponse field: CommandResult struct with detailed results, or MemoryFileSystem for trcboot

Example usage from a plugin:

// Execute trcconfig command
pluginName := "trcsh"
cmdType := "trcconfig"
args := []string{"-env=dev", "-servicesWanted=MyService"}

msg := &tccore.ChatMsg{
	Name:         &pluginName,
	Query:        &[]string{"trcshcmd"},  // REQUIRED: address to trcshcmd plugin
	Response:     &cmdType,
	HookResponse: args,  // Pass args via HookResponse
}

*configContext.ChatSenderChan <- msg

Example kubectl with kubeconfig:

// Execute kubectl command
cmdType := "kubectl"
args := []string{"get", "pods"}

kubectlCmd := &KubectlCommand{
figBytes: kubeConfigBytes,

Example kubectl with kubeconfig:

// Execute kubectl command
cmdType := "kubectl"
args := []string{"get", "pods"}

kubectlCmd := &KubectlCommand{
	Args:          args,
	KubeConfigBytes: kubeConfigBytes,
}

msg := &tccore.ChatMsg{
	Name:         &pluginName,
	Query:        &[]string{"trcshcmd"},
	Response:     &cmdType,
	HookResponse: kubectlCmd,
}

*configContext.ChatSenderChan <- msg

Receiving responses:

The kernel will send back a ChatMsg with:
- Response: status message string
- HookResponse: *CommandResult with ExitCode, Stdout, Stderr, Error

In the plugin's chat_receiver:

func chat_receiver(chat_receive_chan chan *tccore.ChatMsg) {
	for {
		event := <-chat_receive_chan
		if event.HookResponse != nil {
			if result, ok := event.HookResponse.(*CommandResult); ok {
				if result.ExitCode == 0 {
					// Success
				} else {
					// Failure
				}
			}
		}
	}
}

Available Commands:
- "trcconfig"  - Configuration management
- "trcpub"     - Publish to vault
- "trcsub"     - Subscribe/pull from vault
- "trcx"       - Extended configuration operations
- "trcinit"    - Initialize tierceron environment
- "trcplgtool" - Plugin tool operations
- "kubectl"    - Kubernetes operations (requires kubeconfig in HookResponse)
- "trcboot"    - Returns the shared MemoryFileSystem without executing any commands

Note: This architecture ensures plugins have NO dependencies on CLI packages.
Only the kernel needs access to trcconfigbase, trcpubbase, etc.
*/

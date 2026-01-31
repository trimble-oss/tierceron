package shellcmd

/*
Shell Command Execution via ChatMsg

Plugins communicate with the kernel using ChatMsg to execute shell commands.
This follows the same pattern used by other plugins like trcdb for inter-plugin communication.

IMPORTANT: Messages must be explicitly addressed to "kernel" in the Query field to avoid
interfering with plugin-to-plugin communications.

Pattern:
1. Plugin sends ChatMsg with:
   - Name field: source plugin name
   - Query field: MUST include "kernel" as destination (e.g., []string{"kernel"})
   - Response field: command type string (e.g., "trcconfig", "trcpub", etc.)
   - HookResponse field: command arguments as []string for args, or KubectlCommand for kubectl

2. Kernel's chat_receiver processes the message and executes the command

3. Kernel sends response back via ChatMsg with:
   - Response field: status message
   - HookResponse field: CommandResult struct with detailed results

Example usage from a plugin:

// Execute trcconfig command
pluginName := "trctrcsh"
cmdType := "trcconfig"
args := []string{"-env=dev", "-servicesWanted=MyService"}

msg := &tccore.ChatMsg{
	Name:         &pluginName,
	Query:        &[]string{"kernel"},  // REQUIRED: address to kernel
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
}

msg := &tccore.ChatMsg{
ame:         &pluginName,
se:     &cmdType,
uery:        &args,
se: kubectlCmd,
}

*configContext.ChatSenderChan <- msg

Receiving responses:

The kernel will send back a ChatMsg with:
- Response: status message string
- HookResponse: *CommandResult with ExitCode, Stdout, Stderr, Error

In the plugin's chat_receiver:

func chat_receiver(chat_receive_chan chan *tccore.ChatMsg) {
{
t := <-chat_receive_chan
event.HookResponse != nil {
result, ok := event.HookResponse.(*CommandResult); ok {
result.ExitCode == 0 {
Success
else {
Failure
Available Commands:
- "trcconfig"  - Configuration management
- "trcpub"     - Publish to vault
- "trcsub"     - Subscribe/pull from vault
- "trcx"       - Extended configuration operations
- "trcinit"    - Initialize tierceron environment
- "trcplgtool" - Plugin tool operations
- "kubectl"    - Kubernetes operations (requires kubeconfig in HookResponse)

Note: This architecture ensures plugins have NO dependencies on CLI packages.
Only the kernel needs access to trcconfigbase, trcpubbase, etc.
*/

package core

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/plugincoreopts"
	"github.com/trimble-oss/tierceron-core/v2/trcshfs/trcshio"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron-core/v2/core/pluginsync"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trctrcsh/shell"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	configContext *tccore.ConfigContext
	memFs         trcshio.MemoryFileSystem
	memFsReady    chan bool
	sender        chan error
	dfstat        *tccore.TTDINode
)

func receiver(receive_chan chan tccore.KernelCmd) {
	for {
		event := <-receive_chan
		switch {
		case event.Command == tccore.PLUGIN_EVENT_START:
			go start(event.PluginName)
		case event.Command == tccore.PLUGIN_EVENT_STOP:
			go stop(event.PluginName)
			sender <- errors.New("hello shutting down")
			return
		case event.Command == tccore.PLUGIN_EVENT_STATUS:
			// TODO
		default:
			// TODO
		}
	}
}

func chat_receiver(chat_receive_chan chan *tccore.ChatMsg) {
	for {
		event := <-chat_receive_chan
		switch {
		case event == nil:
			continue
		case event.Name != nil && *event.Name == "SHUTDOWN":
			if configContext != nil {
				configContext.Log.Println("trcsh shutting down chat receiver")
			}
			return
		case event.HookResponse != nil:
			// Check if this is a MemoryFileSystem response from trcshcmd
			if mfs, ok := event.HookResponse.(trcshio.MemoryFileSystem); ok {
				configContext.Log.Println("Received MemoryFileSystem from trcshcmd")
				SetMemFs(mfs)
				// Signal that memFs is ready
				if memFsReady != nil {
					select {
					case memFsReady <- true:
					default:
					}
				}
			}
			// Also process through registered hooks using RoutingId
			if event.RoutingId != nil && *event.RoutingId != "" {
				if hook, ok := shell.GetChatMsgHooks().Get(*event.RoutingId); ok {
					if hook(event) {
						shell.GetChatMsgHooks().Remove(*event.RoutingId)
					}
				}
			}
		default:
			// Try processing through registered hooks first using RoutingId
			if event.RoutingId != nil && *event.RoutingId != "" {
				if hook, ok := shell.GetChatMsgHooks().Get(*event.RoutingId); ok {
					if hook(event) {
						shell.GetChatMsgHooks().Remove(*event.RoutingId)
					}
				}
			}
			if configContext != nil {
				configContext.Log.Println("trcsh received chat message")
			}
		}
	}
}

func init() {
	if plugincoreopts.BuildOptions.IsPluginHardwired() {
		return
	}
	peerExe, err := os.Open("plugins/trcsh.so")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Trcsh unable to sha256 plugin")
		return
	}

	defer peerExe.Close()

	h := sha256.New()
	if _, err := io.Copy(h, peerExe); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to copy file for sha256 of plugin: %s\n", err)
		return
	}
	sha := hex.EncodeToString(h.Sum(nil))
	fmt.Fprintf(os.Stderr, "trcsh Version: %s\n", sha)
}

func send_dfstat() {
	if configContext == nil || configContext.DfsChan == nil || dfstat == nil {
		fmt.Fprintln(os.Stderr, "Dataflow Statistic channel not initialized properly for trcsh.")
		return
	}
	dfsctx, _, err := dfstat.GetDeliverStatCtx()
	if err != nil {
		configContext.Log.Println("Failed to get dataflow statistic context: ", err)
		send_err(err)
		return
	}
	tccore.SendDfStat(configContext, dfsctx, dfstat)
}

func send_err(err error) {
	if configContext == nil || configContext.ErrorChan == nil || err == nil {
		fmt.Fprintln(os.Stderr, "Failure to send error message, error channel not initialized properly for trcsh.")
		return
	}
	if dfstat != nil {
		dfsctx, _, err := dfstat.GetDeliverStatCtx()
		if err != nil {
			configContext.Log.Println("Failed to get dataflow statistic context: ", err)
			return
		}
		dfstat.UpdateDataFlowStatistic(dfsctx.FlowGroup,
			dfsctx.FlowName,
			dfsctx.StateName,
			dfsctx.StateCode,
			2,
			func(msg string, err error) {
				configContext.Log.Println(msg, err)
			})
		tccore.SendDfStat(configContext, dfsctx, dfstat)
	}
	*configContext.ErrorChan <- err
}

func InitServer(port int, certBytes []byte, keyBytes []byte) (net.Listener, *grpc.Server, error) {
	var err error

	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		log.Fatalf("Couldn't construct key pair: %v", err)
	}
	creds := credentials.NewServerTLSFromCert(&cert)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to listen:", err)
		return nil, nil, err
	}

	grpcServer := grpc.NewServer(grpc.Creds(creds))

	return lis, grpcServer, nil
}

func start(pluginName string) {
	if configContext == nil {
		fmt.Fprintln(os.Stderr, "no config context initialized for trcsh")
		return
	}

	if configContext.Config != nil {
		dfstat = tccore.InitDataFlow(nil, configContext.ArgosId, false)
		dfstat.UpdateDataFlowStatistic("System",
			pluginName,
			"Start up",
			"1",
			1,
			func(msg string, err error) {
				configContext.Log.Println(msg, err)
			})
		send_dfstat()
	} else {
		configContext.Log.Println("Missing common configs")
		send_err(errors.New("missing common configs"))
		return
	}

	// Acknowledge plugin has started - kernel needs this to set State = 1
	*configContext.CmdSenderChan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_START}
	configContext.Log.Println("Sent PLUGIN_EVENT_START to kernel")

	// Request initial memFs from trcshcmd using trcboot command
	// The kernel ensures trcshcmd State = 1 only after it signals ready,
	// so routing will wait until trcshcmd is actually ready to handle requests
	configContext.Log.Println("Requesting initial MemoryFileSystem from trcshcmd")
	memFsReady = make(chan bool, 1)
	pluginSenderName := "trcsh"
	trcbootCmd := "trcboot"
	msg := &tccore.ChatMsg{
		Name:   &pluginSenderName,     // Source plugin name
		Query:  &[]string{"trcshcmd"}, // Destination is trcshcmd
		ChatId: &trcbootCmd,           // Command to execute
	}
	if configContext.ChatSenderChan != nil {
		*configContext.ChatSenderChan <- msg
	}

	// Block waiting for memFs - shell cannot start without it
	configContext.Log.Println("Waiting for MemoryFileSystem response...")
	<-memFsReady
	configContext.Log.Println("MemoryFileSystem received, launching shell")

	// Launch interactive shell with bubbletea
	configContext.Log.Println("Starting interactive shell...")
	var err error
	if memFs != nil {
		err = shell.RunShell(configContext.ChatSenderChan, memFs)
	} else {
		err = shell.RunShell(configContext.ChatSenderChan)
	}
	if err != nil {
		configContext.Log.Printf("Shell exited with error: %v\n", err)
		send_err(err)
	} else {
		configContext.Log.Println("Shell exited normally")
	}

	// Exit the entire program when shell exits (either normally or with error)
	exec.Command("stty", "sane").Run() // Reset terminal to sane defaults
	os.Exit(0)
}

func stop(pluginName string) {
	if configContext != nil {
		configContext.Log.Println("trcsh received shutdown message from kernel.")
	}
	if configContext != nil {
		dfstat.UpdateDataFlowStatistic("System",
			pluginName,
			"Shutdown",
			"0",
			1, func(msg string, err error) {
				if err != nil {
					configContext.Log.Println(tccore.SanitizeForLogging(err.Error()))
				} else {
					configContext.Log.Println(tccore.SanitizeForLogging(msg))
				}
			})
		send_dfstat()
		*configContext.CmdSenderChan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_STOP}
	}
	dfstat = nil
}

func GetConfigContext(pluginName string) *tccore.ConfigContext { return configContext }

func SetMemFs(mfs trcshio.MemoryFileSystem) {
	memFs = mfs
}

func GetMemFs() trcshio.MemoryFileSystem {
	return memFs
}

func GetConfigPaths(pluginName string) []string {
	return []string{}
}

func PostInit(configContext *tccore.ConfigContext) {
	configContext.Start = start
	sender = *configContext.ErrorChan
	go receiver(*configContext.CmdReceiverChan)
	go chat_receiver(*configContext.ChatReceiverChan)
}

func Init(pluginName string, properties *map[string]any) {
	var err error

	// Refuse to run on Kubernetes
	if isK8s, ok := (*properties)["isKubernetes"].(bool); ok && isK8s {
		(*properties)["log"].(*log.Logger).Printf("Trcsh plugin is not allowed to run on Kubernetes. Refusing to initialize.")
		(*properties)["pluginRefused"] = true
		return
	}

	// Wait for trcshcmd to be ready before initializing
	// This prevents race conditions where trcsh tries to use trcshcmd before it's ready
	logger := (*properties)["log"].(*log.Logger)
	logger.Println("Waiting for trcshcmd plugin to be ready before trcsh initialization...")
	pluginsync.WaitForPluginReady("trcshcmd")
	logger.Println("trcshcmd is ready, proceeding with trcsh initialization")

	configContext, err = tccore.Init(properties,
		tccore.TRCSHHIVEK_CERT,
		tccore.TRCSHHIVEK_KEY,
		"", // No common config path needed for trcsh
		"trcsh",
		start,
		receiver,
		chat_receiver,
	)
	if err != nil {
		(*properties)["log"].(*log.Logger).Printf("Initialization error: %v", err)
		return
	}
}

func GetPluginMessages(pluginName string) []string {
	return []string{}
}

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

	"github.com/trimble-oss/tierceron-core/v2/buildopts/plugincoreopts"

	"github.com/trimble-oss/tierceron-core/v2/core"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/pluginlib"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	configContext *tccore.ConfigContext
	sender        chan error
	dfstat        *tccore.TTDINode
)

func receiver(receive_chan *chan core.KernelCmd) {
	for {
		event := <-*receive_chan
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
	pluginlib.SendDfStat(configContext, dfsctx, dfstat)
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
		pluginlib.SendDfStat(configContext, dfsctx, dfstat)
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
	var config map[string]any

	if config != nil {
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

func GetConfigPaths(pluginName string) []string {
	return []string{}
}

func PostInit(configContext *tccore.ConfigContext) {
	configContext.Start = start
	sender = *configContext.ErrorChan
	go receiver(configContext.CmdReceiverChan)
}

func Init(pluginName string, properties *map[string]any) {
	var err error

	configContext, err = pluginlib.Init(pluginName, properties, PostInit)
	if err != nil {
		(*properties)["log"].(*log.Logger).Printf("Initialization error: %v", err)
		return
	}
}

func GetPluginMessages(pluginName string) []string {
	return []string{}
}

package common

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/plugincoreopts"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const (
	MASHUP_CERT = "Common/MashupCert.crt.mf.tmpl"
	COMMON_PATH = "config"
)

// GetConfigPaths builds the list of config cert paths depending on locality.
func GetConfigPaths(isLocal bool) []string {
	if !isLocal {
		return []string{
			COMMON_PATH,
			tccore.TRCSHHIVEK_CERT,
			tccore.TRCSHHIVEK_KEY,
		}
	}
	return []string{
		COMMON_PATH,
		tccore.TRCSHHIVEK_CERT,
		tccore.TRCSHHIVEK_KEY,
		MASHUP_CERT,
	}
}

// InitTrcshTalk contains the original Init logic; caller assigns returned context to its package variable.
func InitTrcshTalk(pluginName string, properties *map[string]interface{}, startFn func(string), receiverFn func(chan tccore.KernelCmd), chatReceiverFn func(chan *tccore.ChatMsg)) (*tccore.ConfigContext, error) {
	ctx, err := tccore.Init(properties,
		tccore.TRCSHHIVEK_CERT,
		tccore.TRCSHHIVEK_KEY,
		COMMON_PATH,
		"hiveplugin",
		startFn,
		receiverFn,
		chatReceiverFn,
	)
	if ctx == nil {
		return ctx, err
	}
	// Apply prefixed logger.
	ctx.Log = log.New(ctx.Log.Writer(), "[trcshtalk]", log.LstdFlags)
	return ctx, err
}

// AttachMashupCert adds the mashup cert if present.
func AttachMashupCert(ctx *tccore.ConfigContext, properties *map[string]interface{}) error {
	if ctx == nil || properties == nil {
		return errors.New("nil context or properties")
	}
	if cert, ok := (*properties)[MASHUP_CERT]; ok {
		certbytes := cert.([]byte)
		(*ctx.ConfigCerts)[MASHUP_CERT] = certbytes
		ctx.Log.Println("Attached mashup cert")
		return nil
	}
	ctx.Log.Println("Failed to attach mashup cert")
	return errors.New("missing mashup cert")
}

// SendDFStat finalizes and sends the provided dataflow stat copy.
func SendDFStat(ctx *tccore.ConfigContext, dfstat *tccore.TTDINode) {
	if ctx == nil || ctx.DfsChan == nil || dfstat == nil {
		fmt.Fprintln(os.Stderr, "Dataflow Statistic channel not initialized properly for trcshtalk.")
		return
	}
	dfsctx, _, err := dfstat.GetDeliverStatCtx()
	if err != nil {
		ctx.Log.Println("Failed to get dataflow statistic context: ", err)
		return
	}
	dfstat.Name = ctx.ArgosId
	dfstat.FinishStatistic("", "", "", ctx.Log, true, dfsctx)
	ctx.Log.Printf("Sending dataflow statistic to kernel: %s\n", dfstat.Name)
	dfstatClone := *dfstat
	go func(dsc *tccore.TTDINode) {
		if ctx != nil && *ctx.DfsChan != nil {
			*ctx.DfsChan <- dsc
		}
	}(&dfstatClone)
}

// SendErr updates df statistics (if present) and forwards an error on the error channel.
func SendErr(ctx *tccore.ConfigContext, dfstat *tccore.TTDINode, err error) {
	if ctx == nil || ctx.ErrorChan == nil {
		return
	}
	if dfstat != nil {
		dfsctx, _, derr := dfstat.GetDeliverStatCtx()
		if derr == nil {
			dfstat.UpdateDataFlowStatistic(dfsctx.FlowGroup,
				dfsctx.FlowName,
				dfsctx.StateName,
				dfsctx.StateCode,
				2,
				func(msg string, e error) { ctx.Log.Println(msg, e) })
			dfstat.Name = ctx.ArgosId
			dfstat.FinishStatistic("", "", "", ctx.Log, true, dfsctx)
			ctx.Log.Printf("Sending failed dataflow statistic to kernel: %s\n", dfstat.Name)
			go func(sender chan *tccore.TTDINode, dfs *tccore.TTDINode) { sender <- dfs }(*ctx.DfsChan, dfstat)
		}
	}
	*ctx.ErrorChan <- err
}

// StopServer encapsulates server shutdown mechanics (excluding global resets handled by caller).
func StopServer(ctx *tccore.ConfigContext, grpcServer interface{ Stop() }, dfstat *tccore.TTDINode, shutdownChan chan bool, shutdownConfirmChan chan bool, pluginName string) {
	if ctx != nil {
		ctx.Log.Println("Trcshtalk received shutdown message from kernel.")
		ctx.Log.Println("Stopping server")
	}
	if grpcServer != nil {
		grpcServer.Stop()
	}
	if ctx != nil {
		shutdownChan <- true
		<-shutdownConfirmChan
		ctx.Log.Println("Stopped server")
		ctx.Log.Println("Stopped server for trcshtalk.")
		if dfstat != nil {
			dfstat.UpdateDataFlowStatistic("System",
				"trcshtalk",
				"Shutdown",
				"0",
				1, func(msg string, err error) {
					if err != nil {
						ctx.Log.Println(tccore.SanitizeForLogging(err.Error()))
					} else {
						ctx.Log.Println(tccore.SanitizeForLogging(msg))
					}
				})
			SendDFStat(ctx, dfstat)
		}
		*ctx.CmdSenderChan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_STOP}
	}
}

// ChatReceiver is intentionally empty for this plugin.
func ChatReceiver(rec_chan chan *tccore.ChatMsg) {}

// LogPluginVersion computes and prints a sha256 of the plugin binary if present.
func LogPluginVersion(path string) {
	if plugincoreopts.BuildOptions.IsPluginHardwired() {
		return
	}
	peerExe, err := os.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Trcshtalk unable to sha256 plugin")
		return
	}
	defer peerExe.Close()
	h := sha256.New()
	if _, err := io.Copy(h, peerExe); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to copy file for sha256 of plugin: %s\n", err)
		return
	}
	sha := hex.EncodeToString(h.Sum(nil))
	fmt.Fprintf(os.Stderr, "TrcshTalk Version: %s\n", sha)
}

// ReceiverLoop processes kernel commands and delegates to provided start/stop functions.
func ReceiverLoop(receive_chan chan tccore.KernelCmd, startFn func(string), stopFn func(string)) {
	for {
		event := <-receive_chan
		switch event.Command {
		case tccore.PLUGIN_EVENT_START:
			go startFn(event.PluginName)
		case tccore.PLUGIN_EVENT_STOP:
			go stopFn(event.PluginName)
			return
		case tccore.PLUGIN_EVENT_STATUS:
			// no-op for now
		default:
			// no-op
		}
	}
}

// StartCore consolidates environment + server startup. The caller supplies:
// - pluginName: name used in kernel events
// - ctx: initialized config context
// - isTalkBackLocal flag already derivable by caller for cert path decisions
// - registerFn: invoked to register service implementations onto the grpc.Server
// It returns the listener, server, dataflow stat node, and a possible error.
func StartCore(
	pluginName string,
	ctx *tccore.ConfigContext,
	shutdownChan chan bool,
	shutdownConfirmChan chan bool,
	registerFn func(grpcServer *grpc.Server),
) (lisAddr string, grpcServer *grpc.Server, dfstat *tccore.TTDINode, err error) {
	if ctx == nil {
		return "", nil, nil, errors.New("nil config context")
	}
	portInterface, ok := (*ctx.Config)["grpc_server_port"]
	if !ok {
		ctx.Log.Println("Missing config: gprc_server_port")
		return "", nil, nil, errors.New("missing config: gprc_server_port")
	}
	var trcshtalkPort int
	switch v := portInterface.(type) {
	case int:
		trcshtalkPort = v
	case string:
		trcshtalkPort, err = strconv.Atoi(v)
		if err != nil {
			return "", nil, nil, err
		}
	default:
		return "", nil, nil, errors.New("invalid port type")
	}

	// server_mode currently not influencing generic start behavior; ignored here.

	ctx.Log.Printf("Server listening on :%d\n", trcshtalkPort)
	lis, gServer, serr := InitServer(trcshtalkPort,
		(*ctx.ConfigCerts)[tccore.TRCSHHIVEK_CERT],
		(*ctx.ConfigCerts)[tccore.TRCSHHIVEK_KEY])
	if serr != nil {
		ctx.Log.Printf("Failed to start server: %v\n", serr)
		return "", nil, nil, serr
	}

	grpc_health_v1.RegisterHealthServer(gServer, health.NewServer())
	if registerFn != nil {
		registerFn(gServer)
	}
	ctx.Log.Printf("server listening at %v\n", lis.Addr())
	go func(l net.Listener) {
		*ctx.CmdSenderChan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_START}
		if err := gServer.Serve(l); err != nil {
			ctx.Log.Println("Failed to serve:", err)
			SendErr(ctx, nil, err)
			return
		}
	}(lis)

	dfstat = tccore.InitDataFlow(nil, ctx.ArgosId, false)
	dfstat.UpdateDataFlowStatistic("System", "trcshtalk", "Start up", "1", 1, func(msg string, err error) { ctx.Log.Println(msg, err) })
	SendDFStat(ctx, dfstat)
	return lis.Addr().String(), gServer, dfstat, nil
}

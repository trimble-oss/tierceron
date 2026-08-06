package common

import (
	"errors"
	"fmt"
	"strconv"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcshtalk/buildopts/coreopts"
	"google.golang.org/grpc"
)

// StartWithServerModes consolidates legacy server_mode logic
// (standard | trcshtalkback | talkback-kernel-plugin | both | trcshtalk | client | client-both)
// and delegates plugin-specific pieces (service registration, talkback loop, client cert init) via callbacks.
// Returns the started gRPC server (if any), the dataflow stat (if initialized), and any error.
func StartWithServerModes(
	pluginName string,
	ctx *tccore.ConfigContext,
	shutdownChan chan bool,
	shutdownConfirmChan chan bool,
	startTrashTalking func(remoteServerName string, port int, ttbToken *string, isRemote bool),
	registerService func(gs *grpc.Server),
	initClientCert func(cert []byte),
) (*grpc.Server, *tccore.TTDINode, error) {
	if ctx == nil {
		return nil, nil, nil
	}

	// Determine server_mode (legacy modes: standard | trcshtalkback | talkback-kernel-plugin | both | trcshtalk | client | client-both)
	serverMode := ModeStandard
	if modeInterface, ok := (*ctx.Config)[CfgServerMode]; ok {
		if m, ok := modeInterface.(string); ok && m != "" {
			serverMode = m
		}
	} else if modeInterface, ok := (*ctx.Config)[CfgMode]; ok {
		if m, ok := modeInterface.(string); ok && m != "" {
			serverMode = m
		}
	}
	if serverMode == ModeTrcshTalk {
		serverMode = ModeBoth
	}
	usesDirectGRPC := serverMode == ModeTalkbackKernel || serverMode == ModeClient || serverMode == ModeClientBoth
	usesHubConfig := serverMode == ModeClient || serverMode == ModeClientBoth
	runsTalkback := serverMode == ModeTalkback || serverMode == ModeTalkbackKernel || serverMode == ModeBoth || serverMode == ModeClient || serverMode == ModeClientBoth
	runsServer := serverMode == ModeStandard || serverMode == ModeBoth || serverMode == ModeClientBoth

	var dfstat *tccore.TTDINode
	initializedDF := false
	var grpcServer *grpc.Server

	// Talkback configuration (may run with or without server depending on mode)
	if runsTalkback {
		isRemote := true
		var clientCert []byte
		var haveCert bool
		if usesDirectGRPC { // direct gRPC dial path
			isRemote = false
			clientCert, haveCert = (*ctx.ConfigCerts)[tccore.TRCSHHIVEK_CERT]
		} else if coreopts.IsTrcshTalkBackLocal() { // remote talkback w/ mashup cert when running locally
			clientCert, haveCert = (*ctx.ConfigCerts)[MASHUP_CERT]
		}

		// Parse remote port when using direct gRPC dial modes.
		talkbackPort := 0
		if !isRemote {
			portKey := CfgRemotePort
			portLabel := CfgRemotePort
			if usesHubConfig {
				portKey = CfgTrcshTalkHubPort
				portLabel = CfgTrcshTalkHubPort
			}
			if portInterface, ok := (*ctx.Config)[portKey]; ok {
				switch v := portInterface.(type) {
				case int:
					talkbackPort = v
				case string:
					if p, err := strconv.Atoi(v); err == nil {
						talkbackPort = p
					} else {
						SendErr(ctx, dfstat, fmt.Errorf("failed to parse %s: %w", portLabel, err))
						return nil, dfstat, err
					}
				}
			}
		}

		if haveCert {
			initClientCert(clientCert)
		} else if !isRemote && coreopts.IsTrcshTalkBackLocal() { // strict requirement for local mashup cert
			SendErr(ctx, dfstat, errors.New("missing mashup cert"))
			return nil, dfstat, errors.New("missing mashup cert")
		}

		var remoteServerName string
		if usesHubConfig {
			if serverNameInterface, ok := (*ctx.Config)[CfgTrcshTalkHubName]; ok {
				if rsn, ok := serverNameInterface.(string); ok {
					remoteServerName = rsn
				}
			}
		} else if serverNameInterface, ok := (*ctx.Config)[CfgRemoteName]; ok {
			if rsn, ok := serverNameInterface.(string); ok {
				remoteServerName = rsn
			}
		}
		var ttbTokenPtr *string
		if ttbTokenInterface, ok := (*ctx.Config)[CfgTTBToken]; ok {
			if ttbToken, ok := ttbTokenInterface.(string); ok {
				ttbTokenPtr = &ttbToken
			}
		}
		canStartTalkback := false
		if usesDirectGRPC {
			canStartTalkback = remoteServerName != "" && talkbackPort > 0
		} else {
			canStartTalkback = remoteServerName != "" && ttbTokenPtr != nil
		}
		if canStartTalkback {
			// Launch talkback loop
			go func(ttbt *string, port int, remote bool, mode string) {
				// Emit start event if we are NOT also starting the server (pure talkback modes)
				if mode == ModeTalkback || mode == ModeTalkbackKernel || mode == ModeClient {
					*ctx.CmdSenderChan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_START}
				}
				startTrashTalking(remoteServerName, port, ttbt, remote)
				shutdownConfirmChan <- true
			}(ttbTokenPtr, talkbackPort, isRemote, serverMode)
		} else if runsTalkback {
			if usesDirectGRPC {
				if usesHubConfig {
					ctx.Log.Printf("Talkback not started: missing trcshtalk hub name (%s) or trcshtalk hub port (%d).", remoteServerName, talkbackPort)
				} else {
					ctx.Log.Printf("Talkback not started: missing remote name (%s) or remote port (%d).", remoteServerName, talkbackPort)
				}
			} else {
				ctx.Log.Printf("Talkback not started: missing remote name (%s) or token present=%t.", remoteServerName, ttbTokenPtr != nil)
			}
		}

		// If no server will be started (pure talkback modes), still initialize DF stat like legacy code
		if !runsServer {
			if !initializedDF { // avoid double init if server also starts
				df := tccore.InitDataFlow(nil, ctx.ArgosId, false)
				df.UpdateDataFlowStatistic("System", "trcshtalk", "Start up", "1", 1, func(msg string, err error) { ctx.Log.Println(msg, err) })
				SendDFStat(ctx, df)
				dfstat = df
				initializedDF = true
			}
		}
	}

	// Start server only if mode requires it
	if runsServer {
		_, gServer, df, err := StartCore(pluginName, ctx, shutdownChan, shutdownConfirmChan, func(gs *grpc.Server) {
			registerService(gs)
		})
		if err != nil {
			SendErr(ctx, dfstat, err)
			return nil, dfstat, err
		}
		grpcServer = gServer
		dfstat = df
		initializedDF = true
	} else {
		ctx.Log.Printf("Server not started due to server_mode=%s", serverMode)
	}

	return grpcServer, dfstat, nil
}

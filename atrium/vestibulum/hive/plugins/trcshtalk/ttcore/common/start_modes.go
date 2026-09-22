package common

import (
	"errors"
	"fmt"
	"strconv"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcshtalk/buildopts/coreopts"
	"google.golang.org/grpc"
)

func resolveTrcshTalkMode(config *map[string]interface{}) string {
	if config == nil {
		return ModeStandard
	}

	for _, key := range []string{CfgTrcshTalkMode, CfgMode, CfgServerMode} {
		if modeInterface, ok := (*config)[key]; ok {
			if mode, ok := modeInterface.(string); ok && mode != "" {
				return mode
			}
		}
	}

	return ModeStandard
}

// StartWithServerModes consolidates legacy server_mode logic (standard | trcshtalkback | talkback-kernel-plugin | both)
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

	serverMode := resolveTrcshTalkMode(ctx.Config)
	isHubClient := serverMode == ModeHubClient

	var dfstat *tccore.TTDINode
	initializedDF := false
	var grpcServer *grpc.Server

	// Talkback configuration (may run with or without server depending on mode)
	if serverMode == ModeTalkback || serverMode == ModeTalkbackKernel || serverMode == ModeBoth || isHubClient {
		isRemote := true
		var clientCert []byte
		var haveCert bool
		if isHubClient {
			isRemote = false
			clientCert, haveCert = (*ctx.ConfigCerts)[SERVICE_CLIENT_ROOT_CA]
		} else if serverMode == ModeTalkbackKernel {
			isRemote = false
			clientCert, haveCert = (*ctx.ConfigCerts)[tccore.TRCSHHIVEK_CERT]
		} else if coreopts.IsTrcshTalkBackLocal() { // remote talkback w/ mashup cert when running locally
			clientCert, haveCert = (*ctx.ConfigCerts)[MASHUP_CERT]
		}

		// Parse remote (local-dial) port only needed for direct gRPC modes.
		talkbackPort := 0
		if !isRemote {
			portKey := CfgRemotePort
			if isHubClient {
				portKey = CfgTrcshTalkHubPort
			}
			if portInterface, ok := (*ctx.Config)[portKey]; ok {
				switch v := portInterface.(type) {
				case int:
					talkbackPort = v
				case string:
					if p, err := strconv.Atoi(v); err == nil {
						talkbackPort = p
					} else {
						SendErr(ctx, dfstat, fmt.Errorf("failed to parse %s: %w", portKey, err))
						return nil, dfstat, err
					}
				}
			}
		}

		if haveCert {
			initClientCert(clientCert)
		} else if isHubClient {
			SendErr(ctx, dfstat, errors.New("missing service client root CA"))
			return nil, dfstat, errors.New("missing service client root CA")
		} else if !isRemote && coreopts.IsTrcshTalkBackLocal() {
			SendErr(ctx, dfstat, errors.New("missing mashup cert"))
			return nil, dfstat, errors.New("missing mashup cert")
		} else if coreopts.IsTrcshTalkBackLocal() {
			SendErr(ctx, dfstat, errors.New("missing mashup cert"))
			return nil, dfstat, errors.New("missing mashup cert")
		}

		var remoteServerName string
		serverNameKey := CfgRemoteName
		if isHubClient {
			serverNameKey = CfgTrcshTalkHubName
		}
		if serverNameInterface, ok := (*ctx.Config)[serverNameKey]; ok {
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
		canStartTalkback := remoteServerName != "" && ttbTokenPtr != nil
		if isHubClient {
			canStartTalkback = remoteServerName != "" && talkbackPort > 0
		}
		if canStartTalkback {
			// Launch talkback loop
			go func(ttbt *string, port int, remote bool, mode string) {
				// Emit start event if we are NOT also starting the server (pure talkback modes)
				if mode == ModeTalkback || mode == ModeTalkbackKernel || mode == ModeHubClient {
					*ctx.CmdSenderChan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_START}
				}
				startTrashTalking(remoteServerName, port, ttbt, remote)
				shutdownConfirmChan <- true
			}(ttbTokenPtr, talkbackPort, isRemote, serverMode)
		} else if isHubClient {
			ctx.Log.Printf("Talkback not started: missing trcshtalk hub name (%s) or trcshtalk hub port (%d).", remoteServerName, talkbackPort)
		} else if serverMode == ModeTalkback || serverMode == ModeTalkbackKernel || serverMode == ModeBoth {
			ctx.Log.Printf("Talkback not started: missing remote name (%s) or token present=%t.", remoteServerName, ttbTokenPtr != nil)
		}

		// If no server will be started (pure talkback modes), still initialize DF stat like legacy code
		if serverMode == ModeTalkback || serverMode == ModeTalkbackKernel || isHubClient {
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
	if serverMode == ModeStandard || serverMode == ModeBoth {
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

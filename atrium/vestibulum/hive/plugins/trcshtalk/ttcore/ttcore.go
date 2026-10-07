package ttcore

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"slices"
	"strings"
	"sync"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcshtalk/buildopts/coreopts"
	pb "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcshtalk/trcshtalksdk"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcshtalk/ttcore/common"

	// removed rand; now using common.GenMsgID
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type interactionServiceServer struct {
	pb.UnimplementedTrcshTalkServiceServer
}

var (
	configContext  *tccore.ConfigContext
	grpcServer     *grpc.Server
	dfstat         *tccore.TTDINode
	sharedTTBToken string
)

var (
	shutdownChan        chan bool                   = make(chan bool)
	shutdownConfirmChan chan bool                   = make(chan bool)
	proxyRequestChan    chan *pb.InteractionRequest = make(chan *pb.InteractionRequest, 128)
	proxyResponseChans  sync.Map
)

// Interact runs each requested interaction and returns its combined results.
func (s *interactionServiceServer) Interact(ctx context.Context, req *pb.InteractionRequest) (*pb.InteractionResponse, error) {
	if err := common.ValidateIncomingTTBToken(configContext, ctx); err != nil {
		if configContext != nil {
			configContext.Log.Printf("Rejecting Interact request: %v", err)
		}
		return nil, status.Error(codes.Unauthenticated, "invalid talkback token")
	}
	if _, _, ok := loggingDirective(req.GetData()); ok {
		return runLocalInteractions(req)
	}
	if len(req.GetInteractions()) == 0 {
		if response, handled := postProxyResponse(req); handled {
			return response, nil
		}
		return dequeueProxyRequest(ctx, req)
	}
	if requestSupportedByDeployments(req.GetInteractions(), common.SupportedDeploymentsSet(configContext)) {
		return runLocalInteractions(req)
	}
	return enqueueProxyRequest(ctx, req)
}

func runLocalInteractions(req *pb.InteractionRequest) (*pb.InteractionResponse, error) {
	cmds := req.GetInteractions()
	queries := []string{}
	queryTest := req.GetQueryId() + ":"
	if pluginName, action, ok := loggingDirective(req.GetData()); ok {
		queries = append(queries, pluginName)
		queryTest = "log " + action
	} else if slices.Contains(cmds, pb.Interactions_ALL) {
		// run all
		// set queries to all cmds
		configContext.Log.Println("Running all queries.")
		queries = append(queries, "healthcheck")
	} else {
		for _, q := range cmds {
			if q == pb.Interactions_HEALTH_CHECK {
				// set name to plugin...
				configContext.Log.Println("Running healthcheck interaction.")
				queries = append(queries, "healthcheck")
			} else if q == pb.Interactions_TRCDB {
				configContext.Log.Println("Running trcdb interaction.")
				queries = append(queries, "trcdb")
				trcdb_tests := req.GetData()
				for i, test := range trcdb_tests {
					if i == 0 {
						queryTest = fmt.Sprintf("%s%s", queryTest, test)
					} else {
						queryTest = fmt.Sprintf("%s,%s", queryTest, test)
					}
				}
			}

			// else if q == 4 {
			// 	configContext.Log.Println("TrcshTalk shutting down chat receiver.")
			// 	shutdown := "SHUTDOWN"
			// 	*configContext.ChatSenderChan <- &tccore.ChatMsg{
			// 		Name:  &shutdown,
			// 		Query: &[]string{"trcshtalk"},
			// 	}
			// 	return &pb.InteractionResponse{
			// 		MessageId: req.MessageId,
			// 		Results:   "Shutting down interaction runner.",
			// 	}, nil
			// }
		}
	}
	name := "trcshtalk"
	*configContext.ChatSenderChan <- &tccore.ChatMsg{
		RoutingId: &req.MessageId,
		ChatId:    &queryTest,
		Name:      &name,
		Query:     &queries,
	}
	results := ""
	finished_queries := make(map[string]string)
	configContext.Log.Printf("Sent queries to kernel: %d\n", len(queries))
	for {
		event := <-*configContext.ChatReceiverChan
		configContext.Log.Println("TrcshTalk received message from kernel.")
		switch {
		case len(finished_queries) == len(queries):
			configContext.Log.Println("Formatting responses from kernel.")
			for _, v := range finished_queries {
				results = results + v + " "
			}
			configContext.Log.Printf("Sending response to chat from kernel: %s\n", results)
			return &pb.InteractionResponse{
				MessageId: *event.RoutingId,
				Results:   results,
			}, nil
		default:
			configContext.Log.Printf("Received response from query: %s\n", *(*event).Query)
			if len(*(*event).Query) == 1 && event.Response != nil && (*event).Response != nil {
				configContext.Log.Printf("Processing response from query: %s\n", *(*event).Query)
				finished_queries[(*event.Query)[0]] = *(*event).Response
			}
			if len(finished_queries) == len(queries) {
				configContext.Log.Println("Formatting responses.")
				for _, v := range finished_queries {
					results = results + v + " "
				}
				configContext.Log.Printf("Sending response to chat: %s\n", results)
				return &pb.InteractionResponse{
					MessageId: *event.RoutingId,
					Results:   results,
				}, nil
			}
		}
	}
}

func loggingDirective(data []string) (string, string, bool) {
	fields := data
	if len(fields) == 1 {
		fields = strings.Fields(fields[0])
	}
	if len(fields) == 4 && strings.TrimPrefix(strings.ToLower(fields[0]), "@") == "trcshtalk" {
		fields = fields[1:]
	}
	if len(fields) != 3 || strings.ToLower(fields[1]) != "log" {
		return "", "", false
	}
	action := strings.ToLower(fields[2])
	if fields[0] == "" || (action != "start" && action != "stop") {
		return "", "", false
	}
	return fields[0], action, true
}

func enqueueProxyRequest(ctx context.Context, req *pb.InteractionRequest) (*pb.InteractionResponse, error) {
	responseChan := make(chan *pb.InteractionResponse, 1)
	proxyResponseChans.Store(req.GetMessageId(), responseChan)
	defer proxyResponseChans.Delete(req.GetMessageId())

	select {
	case proxyRequestChan <- req:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	select {
	case response := <-responseChan:
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func dequeueProxyRequest(ctx context.Context, req *pb.InteractionRequest) (*pb.InteractionResponse, error) {
	supportedDeployments := common.SupportedDeploymentsFromIncomingContext(ctx)
	var proxyRequest *pb.InteractionRequest
	pendingRequests := len(proxyRequestChan)
	if pendingRequests == 0 {
		select {
		case proxyRequest = <-proxyRequestChan:
			if !requestSupportedByDeployments(proxyRequest.GetInteractions(), supportedDeployments) {
				proxyRequestChan <- proxyRequest
				return &pb.InteractionResponse{MessageId: req.GetMessageId(), Results: ""}, nil
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	} else {
		for i := 0; i < pendingRequests; i++ {
			proxyRequest = <-proxyRequestChan
			if requestSupportedByDeployments(proxyRequest.GetInteractions(), supportedDeployments) {
				break
			}
			proxyRequestChan <- proxyRequest
			proxyRequest = nil
		}
		if proxyRequest == nil {
			return &pb.InteractionResponse{MessageId: req.GetMessageId(), Results: ""}, nil
		}
	}

	requestBytes, err := protojson.Marshal(proxyRequest)
	if err != nil {
		return nil, err
	}
	return &pb.InteractionResponse{MessageId: req.GetMessageId(), Results: string(requestBytes)}, nil
}

func postProxyResponse(req *pb.InteractionRequest) (*pb.InteractionResponse, bool) {
	if len(req.GetData()) == 0 {
		return nil, false
	}
	responseChanValue, ok := proxyResponseChans.Load(req.GetMessageId())
	if !ok {
		return nil, false
	}
	response := &pb.InteractionResponse{MessageId: req.GetMessageId(), Results: req.GetData()[0]}
	responseChanValue.(chan *pb.InteractionResponse) <- response
	return &pb.InteractionResponse{MessageId: req.GetMessageId(), Results: "Response posted"}, true
}

func requestSupportedByDeployments(interactions []pb.Interactions, supportedDeployments map[string]struct{}) bool {
	if len(supportedDeployments) == 0 {
		return true
	}
	requiredDeployments := map[string]struct{}{}
	for _, interaction := range interactions {
		switch interaction {
		case pb.Interactions_ALL, pb.Interactions_HEALTH_CHECK:
			requiredDeployments["healthcheck"] = struct{}{}
		case pb.Interactions_TRCDB:
			requiredDeployments["trcdb"] = struct{}{}
		}
	}
	if len(requiredDeployments) == 0 {
		return true
	}
	for deployment := range requiredDeployments {
		if _, ok := supportedDeployments[deployment]; !ok {
			return false
		}
	}
	return true
}

func GetConfigContext(pluginName string) *tccore.ConfigContext { return configContext }
func GetConfigPaths(pluginName string) []string {
	return common.GetConfigPaths(coreopts.IsTrcshTalkBackLocal())
}

func Init(pluginName string, properties *map[string]interface{}) {
	ctx, err := common.InitTrcshTalk(pluginName, properties, start, receiver, chatReceiver)
	if ctx == nil {
		return
	}
	configContext = ctx
	if configContext.Config != nil {
		if ttbToken, ok := (*configContext.Config)[common.CfgTTBToken].(string); ok && ttbToken != "" {
			sharedTTBToken = ttbToken
			if kernelSecretsAny, ok := (*configContext.Config)["PLUGINCOMMONSECRETS"]; ok {
				if kernelSecrets, ok := kernelSecretsAny.(*sync.Map); ok && kernelSecrets != nil {
					kernelSecrets.Store(common.CfgTTBToken, &sharedTTBToken)
				}
			}
		}
	}
	if coreopts.IsTrcshTalkBackLocal() {
		_ = common.AttachMashupCert(configContext, properties)
	}
	if err != nil && configContext != nil {
		configContext.Log.Println("Failure to initialize trcshtalk.")
		configContext.Log.Println(err.Error())
	}
	configContext.Log.Printf("Successfully initialized trcshtalk for env: %s region: %s\n", configContext.Env, configContext.Region)
}

func init() { common.LogPluginVersion("/usr/local/trcshk/plugins/trcshtalk.so") }

// Wrapper helpers removed; call common.SendDFStat / common.SendErr directly when needed.

func receiver(receive_chan chan tccore.KernelCmd) { common.ReceiverLoop(receive_chan, start, stop) }

// InitServer now provided by common.InitServer (kept wrapper for backward compatibility)
func InitServer(port int, certBytes []byte, keyBytes []byte) (net.Listener, *grpc.Server, error) {
	return common.InitServer(port, certBytes, keyBytes)
}

var mashupCertBytes []byte

func InitCertBytes(cert []byte) {
	mashupCertBytes = cert
}

// GenMsgId delegated to common.GenMsgID
func GenMsgId(env, region string, isBroadcast bool) string {
	return common.GenMsgID(env, region, isBroadcast)
}

// processTrcshTalkRequest now accepts a factory for constructing a new response message
// so the creation of protobuf responses can be overridden or swapped with a compatible type.
func processTrcshTalkRequest(serverName string, port int, ttbToken *string, isRemote bool, interactionReq *pb.InteractionRequest, newResp func() proto.Message, isBroadcast ...bool) (proto.Message, error) {
	b := false
	if len(isBroadcast) > 0 {
		b = isBroadcast[0]
	}
	return common.ProcessTrcshTalkRequestGeneric(
		configContext,
		serverName,
		port,
		ttbToken,
		isRemote,
		proto.Message(interactionReq),
		func(m proto.Message, id string) { m.(*pb.InteractionRequest).MessageId = id },
		newResp,
		func(m proto.Message) string {
			if r, ok := m.(*pb.InteractionResponse); ok {
				return r.Results
			}
			return ""
		},
		shutdownChan,
		b,
		GenMsgId,
		mashupCertBytes,
		func(serverName string, certBytes []byte) (*tls.Config, error) {
			if certBytes == nil {
				return nil, fmt.Errorf("nil cert bytes")
			}
			block, _ := pem.Decode(certBytes)
			if block == nil {
				return nil, fmt.Errorf("failed to decode cert pem")
			}
			parsed, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, err
			}
			pool := x509.NewCertPool()
			pool.AddCert(parsed)
			return &tls.Config{ServerName: serverName, RootCAs: pool, MinVersion: tls.VersionTLS12}, nil
		},
		func(conn *grpc.ClientConn) any { return pb.NewTrcshTalkServiceClient(conn) },
		func(client any, ctx context.Context, req proto.Message) (proto.Message, error) {
			return client.(pb.TrcshTalkServiceClient).Interact(ctx, req.(*pb.InteractionRequest))
		},
	)
}

// Delegated implementations (extracted to common)
func StartTrashTalking(remoteServerName string, port int, ttbToken *string, isRemote bool) {
	common.StartTrashTalkingGeneric(
		configContext,
		shutdownChan,
		func() any { return &pb.InteractionRequest{} },
		func(msg string) any { return &pb.InteractionRequest{MessageId: "", Data: []string{msg}} },
		func(req any, broadcast bool) (any, error) {
			return processTrcshTalkRequest(remoteServerName, port, ttbToken, isRemote, req.(*pb.InteractionRequest), func() proto.Message { return &pb.InteractionResponse{} }, broadcast)
		},
		func(resp any) string {
			if resp != nil {
				return resp.(*pb.InteractionResponse).GetResults()
			} else {
				return ""
			}
		},
		func(data string) (any, error) {
			r := &pb.InteractionRequest{}
			return r, protojson.Unmarshal([]byte(data), r)
		},
		func(r any) any { return TrcshTalkBack(r.(*pb.InteractionRequest)) },
		func(orig any, tb any) any {
			return &pb.InteractionRequest{MessageId: orig.(*pb.InteractionRequest).MessageId, Data: []string{tb.(*pb.InteractionResponse).Results}}
		},
	)
}

// Keep TrcshTalkBack local since its logic is pb-specific and thus not part of reusable common code.
func TrcshTalkBack(req *pb.InteractionRequest) *pb.InteractionResponse {
	cmds := req.GetInteractions()
	if len(cmds) == 0 && strings.TrimSpace(req.GetQueryId()) == "" {
		if messageID, results, ok := common.ProcessProxyEvent(configContext, req.MessageId, req.GetData()); ok {
			return &pb.InteractionResponse{MessageId: messageID, Results: results}
		}
	}

	queries := []string{}
	queryTest := req.GetQueryId() + ":"
	if slices.Contains(cmds, pb.Interactions_ALL) {
		configContext.Log.Println("Running all queries.")
		queries = append(queries, "healthcheck")
	} else {
		for _, q := range cmds {
			if q == pb.Interactions_HEALTH_CHECK {
				health_data := req.GetData()
				if len(health_data) == 1 && health_data[0] == "PROGRESS" {
					queryTest = "PROGRESS"
				}
				configContext.Log.Println("Running healthcheck interaction.")
				queries = append(queries, "healthcheck")
			} else if q == pb.Interactions_TRCDB {
				configContext.Log.Println("Running trcdb interaction.")
				queries = append(queries, "trcdb")
				for i, trcdb_report := range req.GetData() {
					if i == 0 && trcdb_report == "PROGRESS" {
						queryTest = "PROGRESS"
						break
					}
					if i == 0 {
						queryTest = fmt.Sprintf("%s%s", queryTest, trcdb_report)
					} else {
						queryTest = fmt.Sprintf("%s,%s", queryTest, trcdb_report)
					}
				}
			}
		}
	}
	msgID, results := common.CollectQueryResponses(configContext, &req.MessageId, "trcshtalk", &queryTest, queries)
	return &pb.InteractionResponse{MessageId: msgID, Results: results}
}

// startOnce is a pointer so it can be reinitialized on stop to allow restart.
var startOnce *sync.Once = &sync.Once{}

func start(pluginName string) {
	startOnce.Do(func() {
		gs, df, err := common.StartWithServerModes(
			pluginName,
			configContext,
			shutdownChan,
			shutdownConfirmChan,
			StartTrashTalking,
			func(gs *grpc.Server) { pb.RegisterTrcshTalkServiceServer(gs, &interactionServiceServer{}) },
			InitCertBytes,
		)
		if err != nil {
			return
		}
		if gs != nil {
			grpcServer = gs
		}
		if df != nil {
			dfstat = df
		}
	})
}

func stop(pluginName string) {
	common.StopServer(configContext, grpcServer, dfstat, shutdownChan, shutdownConfirmChan, pluginName)
	common.CloseHubClientConnections()
	grpcServer = nil
	dfstat = nil
	proxyRequestChan = make(chan *pb.InteractionRequest, 128)
	proxyResponseChans = sync.Map{}
	// Reset once so start can happen again if needed.
	startOnce = &sync.Once{}
}

func chatReceiver(chatReceiverChan chan *tccore.ChatMsg) { common.ChatReceiver(chatReceiverChan) }

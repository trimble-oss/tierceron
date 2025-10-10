package ttcore

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"slices"
	"sync"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcshtalk/buildopts/coreopts"
	pb "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcshtalk/trcshtalksdk"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcshtalk/ttcore/common"

	// removed rand; now using common.GenMsgID
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type diagnosticsServiceServer struct {
	pb.UnimplementedTrcshTalkServiceServer
}

var (
	configContext *tccore.ConfigContext
	grpcServer    *grpc.Server
	dfstat        *tccore.TTDINode
)

var (
	shutdownChan        chan bool = make(chan bool)
	shutdownConfirmChan chan bool = make(chan bool)
)

// Runs diagnostic services for each Diagnostic within the DiagnosticRequest.
// Returns DiagnosticResponse, forwarding the MessageId of the DiagnosticRequest,
// and providing the results of the diagnostics ran.
func (s *diagnosticsServiceServer) RunDiagnostics(ctx context.Context, req *pb.DiagnosticRequest) (*pb.DiagnosticResponse, error) {
	// TODO: Implement diagnostics plugin
	// if req.Diagnostics contains 0, run all
	// else run each
	cmds := req.GetDiagnostics()
	queries := []string{}
	queryTest := req.GetQueryId() + ":"
	if slices.Contains(cmds, pb.Diagnostics_ALL) {
		// run all
		// set queries to all cmds
		configContext.Log.Println("Running all queries.")
		queries = append(queries, "healthcheck")
	} else {
		for _, q := range cmds {
			if q == pb.Diagnostics_HEALTH_CHECK {
				// set name to plugin...
				configContext.Log.Println("Running healthcheck diagnostic.")
				queries = append(queries, "healthcheck")
			} else if q == pb.Diagnostics_TRCDB {
				configContext.Log.Println("Running trcdb diagnostic.")
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
			// 	return &pb.DiagnosticResponse{
			// 		MessageId: req.MessageId,
			// 		Results:   "Shutting down diagnostic runner.",
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
	// Placeholder code
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
			return &pb.DiagnosticResponse{
				MessageId: *event.RoutingId,
				Results:   results,
			}, nil
		default:
			configContext.Log.Printf("Received response from query: %s\n", *(*event).Query)
			if len(*(*event).Query) == 1 && event.Response != nil && (*event).Response != nil {
				configContext.Log.Printf("Processing response from query: %s\n", *(*event).Query)
				finished_queries[(*event.Query)[0]] = *((*event).Response)
			}
			if len(finished_queries) == len(queries) {
				configContext.Log.Println("Formatting responses.")
				for _, v := range finished_queries {
					results = results + v + " "
				}
				configContext.Log.Printf("Sending response to chat: %s\n", results)
				return &pb.DiagnosticResponse{
					MessageId: *event.RoutingId,
					Results:   results,
				}, nil
			}
		}
	}
	// res := &pb.DiagnosticResponse{
	// 	MessageId: req.MessageId,
	// 	Results:   "gRPCS client/server successful. Diagnostic service not yet implemented.",
	// }
	// // End placeholder code

	// return res, nil
}

func GetConfigContext(pluginName string) *tccore.ConfigContext { return configContext }
func GetConfigPaths(pluginName string) []string {
	return common.GetConfigPaths(coreopts.IsTrcshTalkBackLocal())
}

func Init(pluginName string, properties *map[string]interface{}) {
	ctx, err := common.InitTrcshTalk(pluginName, properties, start, receiver, chat_receiver)
	if ctx == nil {
		return
	}
	configContext = ctx
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
func processTrcshTalkRequest(serverName string, port int, ttbToken *string, isRemote bool, diagReq *pb.DiagnosticRequest, newResp func() proto.Message, isBroadcast ...bool) (proto.Message, error) {
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
		proto.Message(diagReq),
		func(m proto.Message, id string) { m.(*pb.DiagnosticRequest).MessageId = id },
		newResp,
		func(m proto.Message) string {
			if r, ok := m.(*pb.DiagnosticResponse); ok {
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
			return &tls.Config{ServerName: serverName, RootCAs: pool}, nil
		},
		func(conn *grpc.ClientConn) any { return pb.NewTrcshTalkServiceClient(conn) },
		func(client any, ctx context.Context, req proto.Message) (proto.Message, error) {
			return client.(pb.TrcshTalkServiceClient).RunDiagnostics(ctx, req.(*pb.DiagnosticRequest))
		},
	)
}

// Delegated implementations (extracted to common)
func StartTrashTalking(remoteServerName string, port int, ttbToken *string, isRemote bool) {
	common.StartTrashTalkingGeneric(
		configContext,
		shutdownChan,
		func() any { return &pb.DiagnosticRequest{} },
		func(msg string) any { return &pb.DiagnosticRequest{MessageId: "", Data: []string{msg}} },
		func(req any, broadcast bool) (any, error) {
			return processTrcshTalkRequest(remoteServerName, port, ttbToken, isRemote, req.(*pb.DiagnosticRequest), func() proto.Message { return &pb.DiagnosticResponse{} }, broadcast)
		},
		func(resp any) string { return resp.(*pb.DiagnosticResponse).GetResults() },
		func(data string) (any, error) {
			r := &pb.DiagnosticRequest{}
			return r, protojson.Unmarshal([]byte(data), r)
		},
		func(r any) any { return TrcshTalkBack(r.(*pb.DiagnosticRequest)) },
		func(orig any, tb any) any {
			return &pb.DiagnosticRequest{MessageId: orig.(*pb.DiagnosticRequest).MessageId, Data: []string{tb.(*pb.DiagnosticResponse).Results}}
		},
	)
}

// Keep TrcshTalkBack local since its logic is pb-specific and thus not part of reusable common code.
func TrcshTalkBack(req *pb.DiagnosticRequest) *pb.DiagnosticResponse {
	cmds := req.GetDiagnostics()
	queries := []string{}
	queryTest := req.GetQueryId() + ":"
	if slices.Contains(cmds, pb.Diagnostics_ALL) {
		configContext.Log.Println("Running all queries.")
		queries = append(queries, "healthcheck")
	} else {
		for _, q := range cmds {
			if q == pb.Diagnostics_HEALTH_CHECK {
				health_data := req.GetData()
				if len(health_data) == 1 && health_data[0] == "PROGRESS" {
					queryTest = "PROGRESS"
				}
				configContext.Log.Println("Running healthcheck diagnostic.")
				queries = append(queries, "healthcheck")
			} else if q == pb.Diagnostics_TRCDB {
				configContext.Log.Println("Running trcdb diagnostic.")
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
	return &pb.DiagnosticResponse{MessageId: msgID, Results: results}
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
			func(gs *grpc.Server) { pb.RegisterTrcshTalkServiceServer(gs, &diagnosticsServiceServer{}) },
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
	grpcServer = nil
	dfstat = nil
	// Reset once so start can happen again if needed.
	startOnce = &sync.Once{}
}

func chat_receiver(rec_chan chan *tccore.ChatMsg) { common.ChatReceiver(rec_chan) }

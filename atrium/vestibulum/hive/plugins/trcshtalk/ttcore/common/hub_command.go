package common

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strconv"
	"strings"
	"sync"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron-core/v2/identity"
	pb "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcshtalk/trcshtalksdk"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const HubCommandFullMethodName = "/trcshtalksdk.TrcshTalkHubService/RunCommand"

var (
	hubCommandMu      sync.RWMutex
	hubCommandHandler func(context.Context, string, []string) (string, error)
	hubCommandClient  func(context.Context, string, string) (string, error)
)

func SetHubCommandHandler(handler func(context.Context, string, []string) (string, error)) {
	hubCommandMu.Lock()
	hubCommandHandler = handler
	hubCommandMu.Unlock()
}

func SetHubCommandClient(client func(context.Context, string, string) (string, error)) {
	hubCommandMu.Lock()
	hubCommandClient = client
	hubCommandMu.Unlock()
}

func SendHubCommand(ctx context.Context, messageID string, prompt string) (string, error) {
	if strings.Contains(strings.ToLower(prompt), "region:") {
		return "", status.Error(codes.InvalidArgument, "client region overrides are not allowed")
	}
	hubCommandMu.RLock()
	client := hubCommandClient
	provider := oidcIDTokenProvider
	hubCommandMu.RUnlock()
	if client == nil {
		return "", status.Error(codes.Unavailable, "TrcshTalk hub client is not running")
	}
	if provider == nil {
		return "", status.Error(codes.FailedPrecondition, "Talkctl user session is unavailable; run su")
	}
	token, err := provider(ctx, false)
	if err != nil {
		return "", err
	}
	if token == "" {
		return "", status.Error(codes.Unauthenticated, "Talkctl requires a user session; run su")
	}
	callCtx := withOIDCIDToken(ctx, token)
	result, err := client(callCtx, messageID, prompt)
	if !OIDCIDTokenExpired(err) {
		return result, err
	}
	token, err = provider(ctx, true)
	if err != nil {
		return "", err
	}
	if token == "" {
		return "", status.Error(codes.Unauthenticated, "user reauthentication returned no token")
	}
	return client(withOIDCIDToken(ctx, token), messageID, prompt)
}

func withOIDCIDToken(ctx context.Context, token string) context.Context {
	return withOutgoingMetadata(ctx, identity.OIDCIDTokenMetadataKey, token)
}

type hubCommandServer interface {
	RunCommand(context.Context, *pb.DiagnosticRequest) (*pb.DiagnosticResponse, error)
}

type hubCommandService struct {
	config        *tccore.ConfigContext
	validateToken func(context.Context) error
}

func (service *hubCommandService) RunCommand(ctx context.Context, request *pb.DiagnosticRequest) (*pb.DiagnosticResponse, error) {
	workload := service.validateToken
	if workload == nil {
		if service.config == nil || resolveTrcshTalkMode(service.config.Config) != ModeHub {
			return nil, status.Error(codes.FailedPrecondition, "hub command service is not enabled")
		}
		workload = func(ctx context.Context) error { return ValidateIncomingTTBToken(service.config, ctx) }
	}
	var err error
	if !identity.HasElevatedUserAuthorization(ctx) {
		ctx, err = AuthenticateIncoming(ctx, workload, ConfiguredOIDCIDTokenValidator(service.config))
		if err != nil {
			return nil, err
		}
	}
	if err := RequireElevatedUserAuthorization(ctx); err != nil {
		return nil, err
	}
	if request == nil || strings.TrimSpace(request.GetMessageId()) == "" || strings.TrimSpace(strings.Join(request.GetData(), " ")) == "" {
		return nil, status.Error(codes.InvalidArgument, "missing command or message ID")
	}
	hubCommandMu.RLock()
	handler := hubCommandHandler
	hubCommandMu.RUnlock()
	if handler == nil {
		return nil, status.Error(codes.Unavailable, "command router is unavailable")
	}
	result, err := handler(ctx, request.GetMessageId(), request.GetData())
	if err != nil {
		return nil, err
	}
	return &pb.DiagnosticResponse{MessageId: request.GetMessageId(), Results: result}, nil
}

func hasHubMetadataValue(md metadata.MD, key string) bool {
	for _, value := range md.Get(key) {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func RegisterHubCommandService(server *grpc.Server, config *tccore.ConfigContext, tokenValidators ...func(context.Context) error) {
	service := &hubCommandService{config: config}
	if len(tokenValidators) > 0 {
		service.validateToken = tokenValidators[0]
	}
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: "trcshtalksdk.TrcshTalkHubService",
		HandlerType: (*hubCommandServer)(nil),
		Methods:     []grpc.MethodDesc{{MethodName: "RunCommand", Handler: handleHubCommand}},
	}, service)
}

func handleHubCommand(service any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := &pb.DiagnosticRequest{}
	if err := decode(request); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return service.(hubCommandServer).RunCommand(ctx, request)
	}
	info := &grpc.UnaryServerInfo{Server: service, FullMethod: HubCommandFullMethodName}
	return interceptor(ctx, request, info, func(ctx context.Context, message any) (any, error) {
		return service.(hubCommandServer).RunCommand(ctx, message.(*pb.DiagnosticRequest))
	})
}

func RunHubCommand(ctx context.Context, config *tccore.ConfigContext, messageID string, prompt string) (string, error) {
	if config == nil || config.Config == nil || resolveTrcshTalkMode(config.Config) != ModeHubClient {
		return "", status.Error(codes.FailedPrecondition, "TrcshTalk hub client is not configured")
	}
	serverName, _ := (*config.Config)[CfgTrcshTalkHubName].(string)
	var port int
	switch configuredPort := (*config.Config)[CfgTrcshTalkHubPort].(type) {
	case int:
		port = configuredPort
	case string:
		port, _ = strconv.Atoi(configuredPort)
	}
	token := ExpectedTTBToken(config)
	if strings.TrimSpace(serverName) == "" || port < 1 || port > 65535 || token == "" || strings.TrimSpace(config.KernelID) == "" || len(SupportedDeploymentsSet(config)) == 0 {
		return "", status.Error(codes.FailedPrecondition, "TrcshTalk hub client registration is incomplete")
	}
	if config.ConfigCerts == nil {
		return "", status.Error(codes.FailedPrecondition, "TrcshTalk client certificate is unavailable")
	}
	certBytes := (*config.ConfigCerts)[SERVICE_CLIENT_ROOT_CA]
	ctx = withOutgoingMetadata(ctx, HubClientStatefulSetMetadataKey, config.KernelID)
	request := &pb.DiagnosticRequest{MessageId: messageID, Diagnostics: []pb.Diagnostics{pb.Diagnostics_VICO}, Data: []string{prompt}}
	response, err := ProcessTrcshTalkRequestGeneric(
		config, strings.TrimSpace(serverName), port, &token, false, request,
		func(message proto.Message, id string) { message.(*pb.DiagnosticRequest).MessageId = id },
		func() proto.Message { return &pb.DiagnosticResponse{} },
		func(message proto.Message) string { return message.(*pb.DiagnosticResponse).GetResults() },
		nil, false, func(string, string, bool) string { return messageID }, certBytes,
		func(serverName string, certBytes []byte) (*tls.Config, error) {
			roots := x509.NewCertPool()
			if !roots.AppendCertsFromPEM(certBytes) {
				return nil, fmt.Errorf("invalid TrcshTalk service client root CA")
			}
			return &tls.Config{ServerName: serverName, RootCAs: roots, MinVersion: tls.VersionTLS12}, nil
		},
		func(connection *grpc.ClientConn) any { return connection },
		func(client any, callContext context.Context, message proto.Message) (proto.Message, error) {
			return pb.NewTrcshTalkServiceClient(client.(*grpc.ClientConn)).RunDiagnostics(callContext, message.(*pb.DiagnosticRequest))
		}, ctx,
	)
	if err != nil {
		return "", err
	}
	result := response.(*pb.DiagnosticResponse)
	if result.GetMessageId() != messageID {
		return "", status.Error(codes.Internal, "hub response did not match the command")
	}
	return result.GetResults(), nil
}

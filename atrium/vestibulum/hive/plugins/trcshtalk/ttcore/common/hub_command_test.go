package common

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"log"
	"math/big"
	"net"
	"testing"
	"time"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	pb "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcshtalk/trcshtalksdk"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func preserveHubCommandHooks(t *testing.T) {
	t.Helper()
	hubCommandMu.RLock()
	savedHandler, savedClient := hubCommandHandler, hubCommandClient
	hubCommandMu.RUnlock()
	t.Cleanup(func() {
		SetHubCommandHandler(savedHandler)
		SetHubCommandClient(savedClient)
	})
}

type vicoDiagnosticTestServer struct {
	pb.UnimplementedTrcshTalkServiceServer
	service *hubCommandService
}

func (server *vicoDiagnosticTestServer) RunDiagnostics(ctx context.Context, request *pb.DiagnosticRequest) (*pb.DiagnosticResponse, error) {
	if len(request.GetDiagnostics()) != 1 || request.GetDiagnostics()[0] != pb.Diagnostics_VICO {
		return nil, status.Error(codes.InvalidArgument, "command must target Vico")
	}
	return server.service.RunCommand(ctx, request)
}

func startCommandTestHub(t *testing.T, handler func(context.Context, string, []string) (string, error)) *tccore.ConfigContext {
	t.Helper()
	preserveHubCommandHooks(t)
	SetHubCommandHandler(handler)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{certBytes}, PrivateKey: key}},
		MinVersion:   tls.VersionTLS12,
	})))
	hubConfig := &tccore.ConfigContext{Config: &map[string]any{CfgTrcshTalkMode: ModeHub, CfgTTBToken: "test-token"}}
	pb.RegisterTrcshTalkServiceServer(server, &vicoDiagnosticTestServer{service: &hubCommandService{config: hubConfig}})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})
	return &tccore.ConfigContext{
		KernelID: "trcshz-session",
		Config: &map[string]any{
			CfgTrcshTalkMode:    ModeHubClient,
			CfgTrcshTalkHubName: "127.0.0.1",
			CfgTrcshTalkHubPort: listener.Addr().(*net.TCPAddr).Port,
			CfgTTBToken:         "test-token",
			"deployments":       "trcshtalk,talkctl",
		},
		ConfigCerts: &map[string][]byte{SERVICE_CLIENT_ROOT_CA: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})},
		Log:         log.New(io.Discard, "", 0),
	}
}

func TestHubCommandTransportPreservesContextAndCorrelation(t *testing.T) {
	type receivedCommand struct {
		id       string
		data     []string
		metadata metadata.MD
		bounded  bool
	}
	received := make(chan receivedCommand, 1)
	clientConfig := startCommandTestHub(t, func(ctx context.Context, id string, data []string) (string, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		_, bounded := ctx.Deadline()
		received <- receivedCommand{id: id, data: data, metadata: md.Copy(), bounded: bounded}
		return "first\nsecond", nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := RunHubCommand(ctx, clientConfig, "shell-command-1", "inspect worker")
	if err != nil || result != "first\nsecond" {
		t.Fatalf("command failed: result=%q err=%v", result, err)
	}
	command := <-received
	if command.id != "shell-command-1" || len(command.data) != 1 || command.data[0] != "inspect worker" || !command.bounded {
		t.Fatalf("incorrect command envelope: %#v", command)
	}
	for key, expected := range map[string]string{
		TTBTokenMetadataKey:             "test-token",
		HubClientPodMetadataKey:         "trcshz-session",
		HubClientStatefulSetMetadataKey: "trcshz-session",
		SupportedDeploymentsMetadataKey: "trcshtalk,talkctl",
	} {
		values := command.metadata.Get(key)
		if len(values) != 1 || values[0] != expected {
			t.Errorf("incorrect %s metadata: %v", key, values)
		}
	}
}

func TestHubCommandRejectsInvalidRequests(t *testing.T) {
	preserveHubCommandHooks(t)
	SetHubCommandHandler(func(context.Context, string, []string) (string, error) {
		t.Error("invalid command reached the router")
		return "", nil
	})
	service := &hubCommandService{config: &tccore.ConfigContext{Config: &map[string]any{CfgTrcshTalkMode: ModeHub, CfgTTBToken: "test-token"}}}
	for _, test := range []struct {
		name       string
		missingKey string
		messageID  string
		prompt     string
		wantCode   codes.Code
	}{
		{name: "missing token", missingKey: TTBTokenMetadataKey, messageID: "command-1", prompt: "inspect worker", wantCode: codes.Unauthenticated},
		{name: "missing client", missingKey: HubClientPodMetadataKey, messageID: "command-1", prompt: "inspect worker", wantCode: codes.PermissionDenied},
		{name: "missing group", missingKey: HubClientStatefulSetMetadataKey, messageID: "command-1", prompt: "inspect worker", wantCode: codes.PermissionDenied},
		{name: "missing deployments", missingKey: SupportedDeploymentsMetadataKey, messageID: "command-1", prompt: "inspect worker", wantCode: codes.PermissionDenied},
		{name: "missing ID", prompt: "inspect worker", wantCode: codes.InvalidArgument},
		{name: "missing command", messageID: "command-1", wantCode: codes.InvalidArgument},
	} {
		t.Run(test.name, func(t *testing.T) {
			md := metadata.Pairs(TTBTokenMetadataKey, "test-token", HubClientPodMetadataKey, "shell-session", HubClientStatefulSetMetadataKey, "shell-session", SupportedDeploymentsMetadataKey, "trcshtalk,talkctl")
			md.Delete(test.missingKey)
			ctx := metadata.NewIncomingContext(context.Background(), md)
			_, err := service.RunCommand(ctx, &pb.DiagnosticRequest{MessageId: test.messageID, Data: []string{test.prompt}})
			if status.Code(err) != test.wantCode {
				t.Fatalf("incorrect error: got %v want %v", err, test.wantCode)
			}
		})
	}
}

func TestHubCommandTransportRejectsBadTokenAndCancellation(t *testing.T) {
	clientConfig := startCommandTestHub(t, func(context.Context, string, []string) (string, error) {
		t.Error("invalid command reached the router")
		return "", nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	(*clientConfig.Config)[CfgTTBToken] = "wrong-token"
	if _, err := RunHubCommand(ctx, clientConfig, "command-1", "inspect worker"); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("bad token was not rejected: %v", err)
	}
	(*clientConfig.Config)[CfgTTBToken] = "test-token"
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	if _, err := RunHubCommand(cancelled, clientConfig, "command-2", "inspect worker"); status.Code(err) != codes.Canceled {
		t.Fatalf("cancellation was not propagated: %v", err)
	}
}

func TestSendHubCommandRequiresRunningClient(t *testing.T) {
	preserveHubCommandHooks(t)
	SetHubCommandClient(nil)
	if _, err := SendHubCommand(context.Background(), "command-1", "inspect worker"); status.Code(err) != codes.Unavailable {
		t.Fatalf("missing client was not reported: %v", err)
	}
	SetHubCommandClient(func(ctx context.Context, id string, prompt string) (string, error) {
		return id + ":" + prompt, nil
	})
	result, err := SendHubCommand(context.Background(), "command-1", "inspect worker")
	if err != nil || result != "command-1:inspect worker" {
		t.Fatalf("registered client was not used: %q %v", result, err)
	}
}

package tbtgapi

import (
	"context"
	"crypto/tls"
	"embed"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	echocore "github.com/trimble-oss/tierceron/installation/trcshhive/trcshk/echo/echocore" // Update package path as needed
	ttsdk "github.com/trimble-oss/tierceron/installation/trcshhive/trcshk/echo/trcshtalksdk"
	util "github.com/trimble-oss/tierceron/installation/trcshhive/trcshk/echo/util" // Update package path as needed
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const (
	COMMON_PATH      = "/local_config/application"
	CREDENTIALS_PATH = "credentials"
)

type diagnosticsServiceServer struct {
	ttsdk.UnimplementedTrcshTalkServiceServer
}

var grpcServer *grpc.Server
var serverAddr *string //another way to do this...
var dfstat *tccore.TTDINode
var configContext *tccore.ConfigContext

func SetConfigContext(cc *tccore.ConfigContext) {
	configContext = cc
}

func InitConfigCerts(mc embed.FS, mk embed.FS) error {

	mashupCertBytes, err := mc.ReadFile("tls/mashup.crt")
	if err != nil {
		return err
	}

	mashupKeyBytes, err := mk.ReadFile("tls/mashup.key")
	if err != nil {
		return err
	}

	(*configContext.ConfigCerts)[tccore.TRCSHHIVEK_CERT] = mashupCertBytes
	(*configContext.ConfigCerts)[tccore.TRCSHHIVEK_KEY] = mashupKeyBytes

	return nil
}

func send_dfstat() {
	if configContext == nil || configContext.DfsChan == nil || dfstat == nil {
		fmt.Println("Dataflow Statistic channel not initialized properly for echo.")
		return
	}
	dfsctx, _, err := dfstat.GetDeliverStatCtx()
	if err != nil {
		configContext.Log.Println("Failed to get dataflow statistic context: ", err)
		send_err(err)
		return
	}
	dfstat.Name = configContext.ArgosId
	dfstat.FinishStatistic("", "", "", configContext.Log, true, dfsctx)
	configContext.Log.Printf("Sending dataflow statistic to kernel: %s\n", dfstat.Name)
	*configContext.DfsChan <- dfstat
}

func send_err(err error) {
	if configContext == nil || configContext.ErrorChan == nil || err == nil {
		fmt.Println("Failure to send error message, error channel not initialized properly for echo.")
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
		dfstat.Name = configContext.ArgosId
		dfstat.FinishStatistic("", "", "", configContext.Log, true, dfsctx)
		configContext.Log.Printf("Sending failed dataflow statistic to kernel: %s\n", dfstat.Name)
		go func(sender chan *tccore.TTDINode, dfs *tccore.TTDINode) {
			sender <- dfs
		}(*configContext.DfsChan, dfstat)
	}
	*configContext.ErrorChan <- err
}

// Runs diagnostic services for each Diagnostic within the DiagnosticRequest.
// Returns DiagnosticResponse, forwarding the MessageId of the DiagnosticRequest,
// and providing the results of the diagnostics ran.
func (s *diagnosticsServiceServer) RunDiagnostics(ctx context.Context, req *ttsdk.DiagnosticRequest) (*ttsdk.DiagnosticResponse, error) {
	response, _, err := echocore.RunDiagnostics(ctx, req)
	if err != nil {
		configContext.Log.Printf("Diagnostics error: %v", err)
		send_err(err)
	}

	return response, err
}

// Initialize GrpcServer portion
func InitServer(port int, certBytes []byte, keyBytes []byte) (net.Listener, *grpc.Server, error) {
	var err error

	var grpcServer *grpc.Server

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Println("Failed to listen:", err)
		return nil, nil, err
	}

	if echocore.IsKernelPluginMode() {
		cert, err := tls.X509KeyPair(certBytes, keyBytes)
		if err != nil {
			log.Printf("Couldn't construct key pair: %v\n", err) //Should this just return instead?? - no panic
		}
		creds := credentials.NewServerTLSFromCert(&cert)

		grpcServer = grpc.NewServer(grpc.Creds(creds))
	} else {
		grpcServer = grpc.NewServer()
	}

	return lis, grpcServer, nil
}

func InitGServer() (func(error), func()) {
	if portInterface, ok := (*configContext.Config)["grpc_server_port"]; ok {
		var echoPort int
		if port, ok := portInterface.(int); ok {
			echoPort = port
		} else {
			var err error
			echoPort, err = strconv.Atoi(portInterface.(string))
			if err != nil {
				configContext.Log.Printf("Failed to process server port: %v", err)
				send_err(err)
				return send_err, send_dfstat
			}
		}
		fmt.Printf("Server listening on :%d\n", echoPort)
		lis, gServer, err := InitServer(echoPort,
			(*configContext.ConfigCerts)[tccore.TRCSHHIVEK_CERT],
			(*configContext.ConfigCerts)[tccore.TRCSHHIVEK_KEY])
		if err != nil {
			configContext.Log.Printf("Failed to start server: %v", err)
			send_err(err)
			return send_err, send_dfstat
		}
		configContext.Log.Println("Starting server")

		grpcServer = gServer
		grpc_health_v1.RegisterHealthServer(grpcServer, health.NewServer())
		ttsdk.RegisterTrcshTalkServiceServer(grpcServer, &diagnosticsServiceServer{})
		// reflection.Register(grpcServer)
		addr := lis.Addr().String()
		serverAddr = &addr
		log.Printf("Setting up server to listen at %v", lis.Addr())
		go func(l net.Listener, cmd_send_chan *chan int) {
			if echocore.IsKernelPluginMode() {
				if cmd_send_chan != nil {
					*cmd_send_chan <- tccore.PLUGIN_EVENT_START
				}
			}
			log.Printf("server listening at %v", lis.Addr())
			if err := grpcServer.Serve(l); err != nil {
				configContext.Log.Println("Failed to serve:", err)
				send_err(err)
				return
			}
		}(lis, configContext.CmdSenderChan)
		dfstat = tccore.InitDataFlow(nil, configContext.ArgosId, false)
		dfstat.UpdateDataFlowStatistic("System",
			"TrcshTalkBack",
			"Start up",
			"1",
			1,
			func(msg string, err error) {
				configContext.Log.Println(msg, err)
			})
		send_dfstat()
	} else {
		configContext.Log.Println("Missing config: gprc_server_port")
		send_err(errors.New("missing config: gprc_server_port"))
		return send_err, send_dfstat
	}
	return send_err, send_dfstat
}

func StopGServer() {
	if grpcServer == nil || configContext == nil {
		fmt.Println("no server initialized for echo")
		return
	}
	configContext.Log.Println("Healthcheck received shutdown message from kernel.")
	configContext.Log.Println("Stopping server")
	fmt.Println("Stopping server")
	grpcServer.Stop()
	fmt.Println("Stopped server")
	configContext.Log.Println("Stopped server for echo.")
	dfstat.UpdateDataFlowStatistic("System", "TrcshTalkBack", "Shutdown", "0", 1, nil)
	send_dfstat()
	*configContext.CmdSenderChan <- tccore.PLUGIN_EVENT_STOP
	configContext = nil
	grpcServer = nil
	dfstat = nil
}

func HelloWorldDiagnostic() string {
	if configContext == nil ||
		(*configContext.ConfigCerts) == nil ||
		(*configContext.ConfigCerts)[tccore.TRCSHHIVEK_CERT] == nil ||
		(*configContext.ConfigCerts)[tccore.TRCSHHIVEK_KEY] == nil {
		return "Improper config context for echo diagnostic."
	}
	cert, err := tls.X509KeyPair((*configContext.ConfigCerts)[tccore.TRCSHHIVEK_CERT], (*configContext.ConfigCerts)[tccore.TRCSHHIVEK_KEY])
	if err != nil {
		log.Printf("Couldn't construct key pair: %v\n", err)
		return "Unable to run diagnostic for echo."
	}
	creds := credentials.NewServerTLSFromCert(&cert)

	conn, err := grpc.NewClient(*serverAddr, grpc.WithTransportCredentials(creds))
	if err != nil {
		configContext.Log.Printf("did not connect: %v\n", err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s := &diagnosticsServiceServer{}
	r, err := s.RunDiagnostics(ctx, &ttsdk.DiagnosticRequest{
		MessageId:   util.GenMsgId("dev"),
		Diagnostics: []ttsdk.Diagnostics{ttsdk.Diagnostics_HEALTH_CHECK},
	})
	if err != nil {
		configContext.Log.Printf("could not greet: %v\n", err)
	}
	configContext.Log.Printf("Greeting: %s\n", r.GetResults())
	return r.GetResults()
}

package hccore

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	pb "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trchealthcheck/hellosdk"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type server struct {
	pb.UnimplementedGreeterServer
}

var (
	configContext *tccore.ConfigContext
	grpcServer    *grpc.Server
	serverAddr    *string // another way to do this...
	dfstat        *tccore.TTDINode
)

func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	configContext.Log.Printf("Received: %v", in.GetName())
	return &pb.HelloReply{Message: "Hello " + in.GetName()}, nil
}

const (
	COMMON_PATH = "config"
)

func init() {
	peerExe, err := os.Open("/usr/local/trcshk/plugins/healthcheck.so")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Healthcheck unable to sha256 plugin")
		return
	}

	defer peerExe.Close()

	h := sha256.New()
	if _, err := io.Copy(h, peerExe); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to copy file for sha256 of plugin: %s\n", err)
		return
	}
	sha := hex.EncodeToString(h.Sum(nil))
	fmt.Fprintf(os.Stderr, "HealthCheck Version: %s\n", sha)
}

func send_dfstat() {
	if configContext == nil || configContext.DfsChan == nil || dfstat == nil {
		fmt.Fprintln(os.Stderr, "Dataflow Statistic channel not initialized properly for healthcheck.")
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
	dfstatClone := *dfstat
	go func(dsc *tccore.TTDINode) {
		if configContext != nil && *configContext.DfsChan != nil {
			*configContext.DfsChan <- dsc
		}
	}(&dfstatClone)
}

func send_err(err error) {
	if configContext == nil || configContext.ErrorChan == nil || err == nil {
		fmt.Fprintln(os.Stderr, "Failure to send error message, error channel not initialized properly for healthcheck.")
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

func chat_receiver(chat_receive_chan chan *tccore.ChatMsg) {
	for {
		event := <-chat_receive_chan
		switch {
		case event == nil:
			fallthrough
		case *event.Name == "SHUTDOWN":
			configContext.Log.Println("healthcheck shutting down message receiver")
			return
		case event.Response != nil && *event.Response == "Service unavailable":
			configContext.Log.Println("Healthcheck unable to access chat service.")
			return
		case event.ChatId != nil && (*event).ChatId != nil && *event.ChatId == "PROGRESS":
			configContext.Log.Println("healthcheck received progress chat message")
			response := "Running Healthcheck Diagnostic..."
			(*event).Response = &response
			*configContext.ChatSenderChan <- event
		default:
			configContext.Log.Println("healthcheck received chat message")
			response := HelloWorldDiagnostic()
			(*event).Response = &response
			*configContext.ChatSenderChan <- event
		}
	}
}

func HelloWorldDiagnostic() string {
	if configContext == nil ||
		(*configContext.ConfigCerts) == nil ||
		(*configContext.ConfigCerts)[tccore.TRCSHHIVEK_CERT] == nil ||
		(*configContext.ConfigCerts)[tccore.TRCSHHIVEK_KEY] == nil {
		return "Improper config context for healthcheck diagnostic."
	}
	cert, err := tls.X509KeyPair((*configContext.ConfigCerts)[tccore.TRCSHHIVEK_CERT], (*configContext.ConfigCerts)[tccore.TRCSHHIVEK_KEY])
	if err != nil {
		configContext.Log.Printf("Couldn't construct key pair: %v\n", err)
		return "Unable to run diagnostic for healthcheck."
	}
	creds := credentials.NewServerTLSFromCert(&cert)

	conn, err := grpc.NewClient(*serverAddr, grpc.WithTransportCredentials(creds))
	if err != nil {
		configContext.Log.Printf("did not connect: %v\n", err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s := &server{}
	r, err := s.SayHello(ctx, &pb.HelloRequest{Name: "World!"})
	if err != nil {
		configContext.Log.Printf("could not greet: %v\n", err)
	}
	configContext.Log.Printf("Greeting: %s\n", r.GetMessage())
	return r.GetMessage()
}

func receiver(receive_chan chan tccore.KernelCmd) {
	for {
		event := <-receive_chan
		switch {
		case event.Command == tccore.PLUGIN_EVENT_START:
			go start(event.PluginName)
		case event.Command == tccore.PLUGIN_EVENT_STOP:
			go stop(event.PluginName)
			return
		case event.Command == tccore.PLUGIN_EVENT_STATUS:
			// TODO
		default:
			// TODO
		}
	}
}

func InitServer(port int, certBytes []byte, keyBytes []byte) (net.Listener, *grpc.Server, error) {
	var err error

	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		configContext.Log.Printf("Couldn't construct key pair: %v\n", err) // Should this just return instead?? - no panic
	}
	creds := credentials.NewServerTLSFromCert(&cert)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		configContext.Log.Println("Failed to listen:", err)
		return nil, nil, err
	}

	grpcServer := grpc.NewServer(grpc.Creds(creds))

	return lis, grpcServer, nil
}

func start(pluginName string) {
	if configContext == nil {
		fmt.Fprintln(os.Stderr, "no config context initialized for healthcheck")
		return
	}

	if portInterface, ok := (*configContext.Config)["grpc_server_port"]; ok {
		var healthcheckPort int
		if port, ok := portInterface.(int); ok {
			healthcheckPort = port
		} else {
			var err error
			healthcheckPort, err = strconv.Atoi(portInterface.(string))
			if err != nil {
				configContext.Log.Printf("Failed to process server port: %v", err)
				send_err(err)
				return
			}
		}
		configContext.Log.Printf("Server listening on :%d\n", healthcheckPort)
		lis, gServer, err := InitServer(healthcheckPort,
			(*configContext.ConfigCerts)[tccore.TRCSHHIVEK_CERT],
			(*configContext.ConfigCerts)[tccore.TRCSHHIVEK_KEY])
		if err != nil {
			configContext.Log.Printf("Failed to start server: %v", err)
			send_err(err)
			return
		}
		configContext.Log.Println("Starting server")

		grpcServer = gServer
		grpc_health_v1.RegisterHealthServer(grpcServer, health.NewServer())
		pb.RegisterGreeterServer(grpcServer, &server{})
		// reflection.Register(grpcServer)
		addr := lis.Addr().String()
		serverAddr = &addr
		configContext.Log.Printf("server listening at %v", lis.Addr())
		go func(l net.Listener, cmd_send_chan *chan tccore.KernelCmd) {
			if cmd_send_chan != nil {
				*cmd_send_chan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_START}
			}
			if err := grpcServer.Serve(l); err != nil {
				configContext.Log.Println("Failed to serve:", err)
				send_err(err)
				return
			}
		}(lis, configContext.CmdSenderChan)
		dfstat = tccore.InitDataFlow(nil, configContext.ArgosId, false)
		dfstat.UpdateDataFlowStatistic("System",
			"healthcheck",
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
		return
	}
}

func stop(pluginName string) {
	if configContext != nil {
		configContext.Log.Println("Healthcheck received shutdown message from kernel.")
		configContext.Log.Println("Stopping server")
	}
	if grpcServer != nil {
		grpcServer.Stop()
	} else {
		fmt.Fprintln(os.Stderr, "no server initialized for healthcheck")
	}
	if configContext != nil {
		configContext.Log.Println("Stopped server")
		configContext.Log.Println("Stopped server for healthcheck.")
		dfstat.UpdateDataFlowStatistic("System",
			"healthcheck",
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
	grpcServer = nil
	dfstat = nil
}

func GetConfigContext(pluginName string) *tccore.ConfigContext { return configContext }

func Init(pluginName string, properties *map[string]any) {
	var err error
	configContext, err = tccore.Init(properties,
		tccore.TRCSHHIVEK_CERT,
		tccore.TRCSHHIVEK_KEY,
		COMMON_PATH,
		"hiveplugin", // Categorize as hiveplugin
		start,
		receiver,
		chat_receiver,
	)
	if err != nil {
		if configContext != nil {
			configContext.Log.Println("Successfully initialized healthcheck.")
			fmt.Fprintln(os.Stderr, err.Error())
		} else {
			fmt.Fprintln(os.Stderr, err.Error())
			return
		}
	}
	// Change logging context
	configContext.Log = log.New(configContext.Log.Writer(), "[healthcheck]", log.LstdFlags)
	configContext.Log.Println("Successfully initialized healthcheck.")
}

func GetConfigPaths(pluginName string) []string {
	return []string{
		COMMON_PATH,
		tccore.TRCSHHIVEK_CERT,
		tccore.TRCSHHIVEK_KEY,
	}
}

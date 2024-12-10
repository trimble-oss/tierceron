package hcore

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

	"github.com/trimble-oss/tierceron-core/v2/core"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	pb "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trchealthcheck/hellosdk" // Update package path as needed
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type server struct {
	pb.UnimplementedGreeterServer
}

var configContext *tccore.ConfigContext
var grpcServer *grpc.Server
var sender chan error
var serverAddr *string //another way to do this...
var dfstat *tccore.TTDINode

func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Printf("Received: %v", in.GetName())
	return &pb.HelloReply{Message: "Hello " + in.GetName()}, nil
}

const (
	HELLO_CERT  = "Common/hello.crt.mf.tmpl"
	HELLO_KEY   = "Common/hellokey.key.mf.tmpl"
	COMMON_PATH = "config"
)

func receiver(receive_chan chan core.KernelCmd) {
	for {
		event := <-receive_chan
		switch {
		case event.Command == tccore.PLUGIN_EVENT_START:
			go start(event.PluginName)
		case event.Command == tccore.PLUGIN_EVENT_STOP:
			go stop(event.PluginName)
			sender <- errors.New("hello shutting down")
			return
		case event.Command == tccore.PLUGIN_EVENT_STATUS:
			//TODO
		default:
			//TODO
		}
	}
}

func init() {
	peerExe, err := os.Open("plugins/healthcheck.so")
	if err != nil {
		fmt.Println("Unable to sha256 plugin")
		return
	}

	defer peerExe.Close()

	h := sha256.New()
	if _, err := io.Copy(h, peerExe); err != nil {
		fmt.Printf("Unable to copy file for sha256 of plugin: %s\n", err)
		return
	}
	sha := hex.EncodeToString(h.Sum(nil))
	fmt.Printf("HealthCheck Version: %s\n", sha)
}

func send_dfstat() {
	if configContext == nil || configContext.DfsChan == nil || dfstat == nil {
		fmt.Println("Dataflow Statistic channel not initialized properly for healthcheck.")
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
		fmt.Println("Failure to send error message, error channel not initialized properly for healthcheck.")
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

func InitServer(port int, certBytes []byte, keyBytes []byte) (net.Listener, *grpc.Server, error) {
	var err error

	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		log.Fatalf("Couldn't construct key pair: %v", err)
	}
	creds := credentials.NewServerTLSFromCert(&cert)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Println("Failed to listen:", err)
		return nil, nil, err
	}

	grpcServer := grpc.NewServer(grpc.Creds(creds))

	return lis, grpcServer, nil
}

func start(pluginName string) {
	if configContext == nil {
		fmt.Println("no config context initialized for healthcheck")
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
			pluginName,
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
		fmt.Println("no server initialized for healthcheck")
	}
	if configContext != nil {
		configContext.Log.Println("Stopped server")
		configContext.Log.Println("Stopped server for healthcheck.")
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
	grpcServer = nil
	dfstat = nil
}

func GetConfigContext(pluginName string) *tccore.ConfigContext { return configContext }

func GetConfigPaths(pluginName string) []string {
	return []string{
		COMMON_PATH,
		HELLO_CERT,
		HELLO_KEY,
	}
}

func Init(pluginName string, properties *map[string]interface{}) {
	if properties == nil {
		fmt.Println("Missing initialization components")
		return
	}
	var logger *log.Logger
	if _, ok := (*properties)["log"].(*log.Logger); ok {
		logger = (*properties)["log"].(*log.Logger)
	}

	configContext = &tccore.ConfigContext{
		Config: properties,
		Start:  start,
		Log:    logger,
	}

	var certbytes []byte
	var keybytes []byte
	if cert, ok := (*properties)[HELLO_CERT]; ok {
		certbytes = cert.([]byte)
		(*configContext.ConfigCerts)[HELLO_CERT] = certbytes
	}
	if key, ok := (*properties)[HELLO_KEY]; ok {
		keybytes = key.([]byte)
		(*configContext.ConfigCerts)[HELLO_KEY] = keybytes
	}
	if _, ok := (*properties)[COMMON_PATH]; !ok {
		fmt.Println("Missing common config components")
		return
	}

	if channels, ok := (*properties)[tccore.PLUGIN_EVENT_CHANNELS_MAP_KEY]; ok {
		if chans, ok := channels.(map[string]interface{}); ok {
			if rchan, ok := chans[tccore.PLUGIN_CHANNEL_EVENT_IN]; ok {
				if rc, ok := rchan.(chan core.KernelCmd); ok && rc != nil {
					go receiver(rc)
				} else {
					configContext.Log.Println("Unsupported receiving channel passed into hello")
					return
				}
			} else {
				configContext.Log.Println("No receiving channel passed into hello")
				return
			}
			if schan, ok := chans[tccore.PLUGIN_CHANNEL_EVENT_OUT]; ok {
				if sc, ok := schan.(chan error); ok && sc != nil {
					sender = sc
				} else {
					configContext.Log.Println("Unsupported sending channel passed into hello")
					return
				}
			} else {
				configContext.Log.Println("No sending channel passed into hello")
				return
			}
		} else {
			configContext.Log.Println("No channels passed into hello")
			return
		}
	}
}

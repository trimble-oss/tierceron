package hcore

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"

	"github.com/trimble-oss/tierceron-core/v2/core"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	pb "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcmutabilis/mutabilissdk" // Update package path as needed
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type server struct {
	pb.UnimplementedGreeterServer
}

var configContextMap map[string]*tccore.ConfigContext
var grpcServer *grpc.Server
var sender chan error

func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Printf("Received: %v", in.GetName())
	return &pb.HelloReply{Message: "Hello " + in.GetName()}, nil
}

const (
	HELLO_CERT  = "Common/hello.crt.mf.tmpl"
	HELLO_KEY   = "Common/hellokey.key.mf.tmpl"
	COMMON_PATH = "config"
)

func receiverMutabile(configContext *tccore.ConfigContext, receive_chan chan core.KernelCmd) {
	for {
		event := <-receive_chan
		switch {
		case event.Command == tccore.PLUGIN_EVENT_START:
			go configContext.Start(event.PluginName)
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

func InitServer(pluginName string, port int, certBytes []byte, keyBytes []byte) (net.Listener, *grpc.Server, error) {
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
	if configContextMap == nil {
		fmt.Println("no config context initialized for healthcheck")
		return
	}

	if configContext, ok := configContextMap[pluginName]; ok {

		if portInterface, ok := (*configContext.Config)["grpc_server_port"]; ok {
			var helloPort int
			if port, ok := portInterface.(int); ok {
				helloPort = port
			} else {
				var err error
				helloPort, err = strconv.Atoi(portInterface.(string))
				if err != nil {
					configContext.Log.Printf("Failed to process server port: %v", err)
					if sender != nil {
						sender <- err
					}
					return
				}
			}

			fmt.Printf("Server listening on :%d\n", helloPort)
			lis, gServer, err := InitServer(pluginName, helloPort,
				(*configContext.ConfigCerts)[HELLO_CERT],
				(*configContext.ConfigCerts)[HELLO_KEY])
			if err != nil {
				configContext.Log.Printf("Failed to start server: %v", err)
				if sender != nil {
					sender <- err
				}
				return
			}
			configContext.Log.Println("Starting server")

			grpcServer = gServer
			grpc_health_v1.RegisterHealthServer(grpcServer, health.NewServer())
			pb.RegisterGreeterServer(grpcServer, &server{})
			log.Printf("server listening at %v", lis.Addr())
			if err := grpcServer.Serve(lis); err != nil {
				configContext.Log.Println("Failed to serve:", err)
				if sender != nil {
					sender <- err
				}
				return
			}
		} else {
			configContext.Log.Println("Missing config: gprc_server_port")
			if sender != nil {
				sender <- errors.New("missing config: gprc_server_port")
			}
			return
		}
	}
}

func stop(pluginName string) {
	if grpcServer == nil || configContextMap == nil {
		fmt.Println("no server initialized for mutabilis")
		return
	}
	configContext := configContextMap[pluginName]
	if configContext == nil {
		fmt.Println("no context initialized for mutabilis")
		return
	}
	configContext.Log.Println("Stopping server")
	fmt.Println("Stopping server")
	grpcServer.Stop()
	fmt.Println("Stopped server")
	configContext = nil
	grpcServer = nil
	sender = nil
}

func GetConfigContext(pluginName string) *tccore.ConfigContext {
	return configContextMap[pluginName]
}

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
	configContext := configContextMap[pluginName]

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
					go receiverMutabile(configContext, rc)
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

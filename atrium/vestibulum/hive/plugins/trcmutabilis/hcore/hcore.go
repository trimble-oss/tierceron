package hcore

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/trimble-oss/tierceron-core/v2/core"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	pb "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcmutabilis/mutabilissdk" // Update package path as needed
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcmutabilis/trcshio/trcshzig"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
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
var serverAddr *string //another way to do this...
var dfstat *tccore.TTDINode

func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Printf("Received: %v", in.GetName())
	return &pb.HelloReply{Message: "Hello " + in.GetName()}, nil
}

func receiverMutabile(configContext *tccore.ConfigContext, receive_chan *chan core.KernelCmd) {
	for {
		event := <-*receive_chan
		switch {
		case event.Command == tccore.PLUGIN_EVENT_START:
			go configContext.Start(event.PluginName)
		case event.Command == tccore.PLUGIN_EVENT_STOP:
			go stop(event.PluginName)
			sender <- errors.New("mutabilis shutting down")
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

func send_dfstat(pluginName string) {
	if configContext, ok := configContextMap[pluginName]; ok {
		if configContext == nil || configContext.DfsChan == nil || dfstat == nil {
			fmt.Println("Dataflow Statistic channel not initialized properly for healthcheck.")
			return
		}
		dfsctx, _, err := dfstat.GetDeliverStatCtx()
		if err != nil {
			configContext.Log.Println("Failed to get dataflow statistic context: ", err)
			send_err(pluginName, err)
			return
		}
		tccore.SendDfStat(configContext, dfsctx, dfstat)
	}
}

func send_err(pluginName string, err error) {
	if configContext, ok := configContextMap[pluginName]; ok {
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
			tccore.SendDfStat(configContext, dfsctx, dfstat)
		}
		*configContext.ErrorChan <- err
	}
}

func start(pluginName string) {
	if configContextMap == nil {
		fmt.Println("no config context initialized for mutabilis")
		return
	}

	if configContext, ok := configContextMap[pluginName]; ok {
		// TODO: Chewbacca, exec java here...
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
				(*configContext.ConfigCerts)[tccore.TRCSHHIVEK_CERT],
				(*configContext.ConfigCerts)[tccore.TRCSHHIVEK_KEY])
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
			addr := lis.Addr().String()
			serverAddr = &addr
			configContext.Log.Printf("server listening at %v", lis.Addr())
			go func(l net.Listener, cmd_send_chan *chan tccore.KernelCmd) {
				if cmd_send_chan != nil {
					*cmd_send_chan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_START}
				}
				if err := grpcServer.Serve(l); err != nil {
					configContext.Log.Println("Failed to serve:", err)
					send_err(pluginName, err)
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
			send_dfstat(pluginName)
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
	configContext := configContextMap[pluginName]

	if configContext != nil {
		configContext.Log.Println("Mutabilis received shutdown message from kernel.")
		configContext.Log.Println("Stopping server")
	}
	if grpcServer != nil {
		grpcServer.Stop()
	} else {
		fmt.Println("no server initialized for mutabilis")
	}
	if configContext != nil {
		configContext.Log.Println("Stopped server")
		configContext.Log.Println("Stopped server for mutabilis.")
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
		send_dfstat(pluginName)
		*configContext.CmdSenderChan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_STOP}
	}
	grpcServer = nil
	dfstat = nil
	sender = nil
}

func GetConfigContext(pluginName string) *tccore.ConfigContext {
	return configContextMap[pluginName]
}

// Start here...
func GetConfigPaths(pluginName string) []string {
	retrictedMappingsMap := coreopts.BuildOptions.GetPluginRestrictedMappings()

	if pluginRestrictedMappings, ok := retrictedMappingsMap[pluginName]; ok {
		for _, restrictedMapping := range pluginRestrictedMappings {
			if strings.Contains(restrictedMapping[0], "-templateFilter=") {
				return restrictedMapping
			}
		}
	}
	return []string{}
}

func PostInit(configContext *tccore.ConfigContext) {
	configContext.Start = start
	sender = *configContext.ErrorChan
	go receiverMutabile(configContext, configContext.CmdReceiverChan)
}

func Init(pluginName string, properties *map[string]interface{}) {
	var err error
	if configContextMap == nil {
		configContextMap = map[string]*tccore.ConfigContext{}
	}
	configContextMap[pluginName], err = tccore.InitPost(pluginName, properties, PostInit)
	if err != nil && properties != nil && (*properties)["log"] != nil {
		(*properties)["log"].(*log.Logger).Printf("Initialization error: %v", err)
		return
	}
	mntDir, err := trcshzig.ZigInit(configContextMap[pluginName], pluginName, properties)
	if err != nil && properties != nil && (*properties)["log"] != nil {
		(*properties)["log"].(*log.Logger).Printf("File system initialization error: %v\n", err)
		return
	}

	// Convert all properties to mem files....
	for propKey, _ := range *properties {
		trcshzig.LinkMemFile(configContextMap[pluginName], *properties, propKey, pluginName, mntDir)
	}
	pluginDir := fmt.Sprintf("/usr/local/trcshk/plugins/%s", pluginName)
	err = trcshzig.ExecPlugin(configContextMap[pluginName], pluginName, *properties, pluginDir)
	if err != nil && configContextMap[pluginName].Log != nil {
		configContextMap[pluginName].Log.Printf("Unable to exec plugin %s\n", pluginName)
	}
}

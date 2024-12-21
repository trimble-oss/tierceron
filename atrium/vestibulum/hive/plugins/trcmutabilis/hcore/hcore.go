package hcore

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/trimble-oss/tierceron-core/v2/core"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	pb "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcmutabilis/mutabilissdk" // Update package path as needed
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
			dfstat.Name = configContext.ArgosId
			dfstat.FinishStatistic("", "", "", configContext.Log, true, dfsctx)
			configContext.Log.Printf("Sending failed dataflow statistic to kernel: %s\n", dfstat.Name)
			go func(sender chan *tccore.TTDINode, dfs *tccore.TTDINode) {
				sender <- dfs
			}(*configContext.DfsChan, dfstat)
		}
		*configContext.ErrorChan <- err
	}
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
				pluginFilters := strings.Split(restrictedMapping[0], "-templateFilter=")
				return pluginFilters
			}
		}
	}
	return []string{}
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

	configContext := &tccore.ConfigContext{
		Config: properties,
		Start:  start,
		Log:    logger,
	}
	configContextMap[pluginName] = configContext

	// Convert all properties to mem files....
	for propKey, propValue := range *properties {
		data := propValue.([]byte)
		fd, err := unix.MemfdCreate(propKey, unix.MFD_CLOEXEC)
		if err != nil {
			log.Fatal("Failed to create memory file:", err)
		}

		// Convert the file descriptor to *os.File
		file := os.NewFile(uintptr(fd), propKey)
		defer file.Close()

		// Resize file to match data length
		if _, err := file.Write(make([]byte, len(data))); err != nil {
			log.Fatal("Failed to resize file:", err)
		}
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

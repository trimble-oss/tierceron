package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"

	pb "github.com/trimble-oss/tierceron/installation/trclocal/trcshtalk/trcshtalksdk" // Update package path as needed

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type server struct {
	pb.UnimplementedGreeterServer
}

var configContext *ConfigContext
var grpcServer *grpc.Server
var err_sender chan error
var ttdi_sender chan *tccore.TTDINode
var dfstat *tccore.TTDINode
var argosId string

func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Printf("Received: %v", in.GetName())
	return &pb.HelloReply{Message: "Hello " + in.GetName()}, nil
}

const (
	TRCSHTALK_CERT = "Common/PluginCertTrcshTalk.crt.mf.tmpl"
	TRCSHTALK_KEY  = "Common/PluginCertTrcshTalkKey.key.mf.tmpl"
	COMMON_PATH    = "config"
)

func GetConfigPaths() []string {
	return []string{
		COMMON_PATH,
		TRCSHTALK_CERT,
		TRCSHTALK_KEY,
	}
}

func init() {
	peerExe, err := os.Open("/usr/local/trcshk/plugins/trcshtalk.so")
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
	fmt.Printf("TrcshTalk Version: %s\n", sha)
}

func send_dfstat() {
	if ttdi_sender == nil || configContext == nil || dfstat == nil {
		fmt.Println("Dataflow Statistic channel not initialized properly for trcshtalk.")
		return
	}
	dfsctx, _, err := dfstat.GetDeliverStatCtx()
	if err != nil {
		configContext.Log.Println("Failed to get dataflow statistic context: ", err)
		send_err(err)
		return
	}
	dfstat.Name = argosId
	dfstat.FinishStatistic("", "", "", configContext.Log, true, dfsctx)
	configContext.Log.Printf("Sending dataflow statistic to kernel: %s\n", dfstat.Name)
	ttdi_sender <- dfstat
}

func send_err(err error) {
	if err_sender == nil || configContext == nil {
		fmt.Println("Failure to send error message, error channel not initialized properly for trcshtalk.")
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
		dfstat.Name = argosId
		dfstat.FinishStatistic("", "", "", configContext.Log, true, dfsctx)
		configContext.Log.Printf("Sending failed dataflow statistic to kernel: %s\n", dfstat.Name)
		go func(sender chan *tccore.TTDINode, dfs *tccore.TTDINode) {
			sender <- dfs
		}(ttdi_sender, dfstat)
	}
	err_sender <- err
}

func receiver(receive_chan chan int) {
	for {
		event := <-receive_chan
		switch {
		case event == tccore.PLUGIN_EVENT_START:
			go start()
		case event == tccore.PLUGIN_EVENT_STOP:
			go stop()
			return
		case event == tccore.PLUGIN_EVENT_STATUS:
			//TODO
		default:
			//TODO
		}
	}
}

func InitServer(port int, certBytes []byte, keyBytes []byte) (net.Listener, *grpc.Server, error) {
	var err error

	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		log.Fatalf("Couldn't construct key pair: %v", err) //Should this just return instead?? - no panic
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

type ConfigContext struct {
	Config *map[string]interface{}
	Start  func()
	Cert   []byte
	Key    []byte
	Log    *log.Logger
}

func start() {
	if configContext == nil {
		fmt.Println("no config context initialized for trcshtalk")
		return
	}
	if portInterface, ok := (*configContext.Config)["grpc_server_port"]; ok {
		var trcshtalkPort int
		if port, ok := portInterface.(int); ok {
			trcshtalkPort = port
		} else {
			var err error
			trcshtalkPort, err = strconv.Atoi(portInterface.(string))
			if err != nil {
				configContext.Log.Printf("Failed to process server port: %v", err)
				send_err(err)
				return
			}
		}
		fmt.Printf("Server listening on :%d\n", trcshtalkPort)
		lis, gServer, err := InitServer(trcshtalkPort, configContext.Cert, configContext.Key)
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
		log.Printf("server listening at %v", lis.Addr())
		go func(l net.Listener) {
			if err := grpcServer.Serve(l); err != nil {
				configContext.Log.Println("Failed to serve:", err)
				send_err(err)
				return
			}
		}(lis)
		dfstat = tccore.InitDataFlow(nil, argosId, false)
		dfstat.UpdateDataFlowStatistic("System",
			"TrcshTalk",
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

func stop() {
	if grpcServer == nil || configContext == nil {
		fmt.Println("no server initialized for trcshtalk")
		return
	}
	configContext.Log.Println("Trcshtalk received shutdown message from kernel.")
	configContext.Log.Println("Stopping server")
	fmt.Println("Stopping server")
	grpcServer.Stop()
	fmt.Println("Stopped server")
	configContext.Log.Println("Stopped server for trcshtalk.")
	dfstat.UpdateDataFlowStatistic("System", "trcshtalk", "Shutdown", "0", 1, nil)
	send_dfstat()
	if err_sender != nil {
		err_sender <- errors.New("trcshtalk shutting down")
	}
	ttdi_sender <- nil
	configContext = nil
	grpcServer = nil
	err_sender = nil
	ttdi_sender = nil
	dfstat = nil
}

func Init(properties *map[string]interface{}) {
	if properties == nil {
		fmt.Println("Missing initialization components")
		return
	}
	var logger *log.Logger
	if _, ok := (*properties)["log"].(*log.Logger); ok {
		logger = (*properties)["log"].(*log.Logger)
	} else {
		fmt.Println("Missing log from kernel for trcshtalk.")
		return
	}

	var env string
	if e, ok := (*properties)["env"].(string); ok {
		env = e
	} else {
		fmt.Println("Missing env from kernel for trcshtalk")
		logger.Println("Missing env from kernel for trcshtalk")
		return
	}

	if len(argosId) == 0 {
		splitEnv := strings.Split(env, "-")
		if len(splitEnv) == 2 {
			argosId = "trcshk-" + splitEnv[1]
		} else {
			argosId = "trcshk"
		}
	}
	logger.Printf("Starting initialization for dataflow: %s\n", argosId)
	var certbytes []byte
	var keybytes []byte
	var config_properties *map[string]interface{}
	if cert, ok := (*properties)[TRCSHTALK_CERT]; ok {
		certbytes = cert.([]byte)
	} else {
		fmt.Println("Missing cert for trcshtalk.")
		logger.Println("Missing cert for trcshtalk.")
		return
	}
	if key, ok := (*properties)[TRCSHTALK_KEY]; ok {
		keybytes = key.([]byte)
	} else {
		fmt.Println("Missing key for trcshtalk.")
		logger.Println("Missing key for trcshtalk.")
		return
	}
	if common, ok := (*properties)[COMMON_PATH]; ok {
		config_properties = common.(*map[string]interface{})
	} else {
		fmt.Println("Missing common config components for trcshtalk.")
		logger.Println("Missing common config components for trcshtalk.")
		return
	}

	configContext = &ConfigContext{
		Config: config_properties,
		Start:  start,
		Cert:   certbytes,
		Key:    keybytes,
		Log:    logger,
	}

	if channels, ok := (*properties)[tccore.PLUGIN_EVENT_CHANNELS_MAP_KEY]; ok {
		if chans, ok := channels.(map[string]interface{}); ok {
			if rchan, ok := chans[tccore.PLUGIN_CHANNEL_EVENT_IN]; ok {
				if rc, ok := rchan.(chan int); ok && rc != nil {
					configContext.Log.Println("Receiver initialized for trcshtalk.")
					go receiver(rc)
				} else {
					configContext.Log.Println("Unsupported receiving channel passed into trcshtalk")
					return
				}
			} else {
				configContext.Log.Println("No receiving channel passed into trcshtalk")
				return
			}
			if schan, ok := chans[tccore.PLUGIN_CHANNEL_EVENT_OUT]; ok {
				if sc_map, ok := schan.(map[string]interface{}); ok {
					if dfstat_chan, ok := sc_map[tccore.DATA_FLOW_STAT_CHANNEL].(chan *tccore.TTDINode); ok {
						configContext.Log.Println("Dataflow statistics channel initialized for trcshtalk.")
						ttdi_sender = dfstat_chan
					} else {
						configContext.Log.Println("Unsupported dataflow statistics channel passed into trcshtalk")
						return
					}
					if err_chan, ok := sc_map[tccore.ERROR_CHANNEL].(chan error); ok {
						configContext.Log.Println("Error channel initialized for trcshtalk.")
						err_sender = err_chan
					} else {
						configContext.Log.Println("Unsupported error sending channel passed into trcshtalk")
						return
					}
				} else {
					configContext.Log.Println("Unsupported sending channel passed into trcshtalk")
					return
				}
			} else {
				configContext.Log.Println("No sending channel passed into trcshtalk")
				return
			}
		} else {
			configContext.Log.Println("No channels passed into trcshtalk")
			return
		}
	}
	configContext.Log.Println("Successfully initialized trcshtalk.")
}

func main() {
	logFilePtr := flag.String("log", "./trcshtalk.log", "Output path for log file") //renamed from trchealthcheck.log
	flag.Parse()
	config := make(map[string]interface{})

	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		os.Exit(-1)
	}
	logger := log.New(f, "[trcshtalk]", log.LstdFlags)
	config["log"] = logger

	data, err := os.ReadFile("config.yml")
	if err != nil {
		logger.Println("Error reading YAML file:", err)
		os.Exit(-1)
	}

	// Create an empty map for the YAML data
	var configCommon map[string]interface{}

	// Unmarshal the YAML data into the map
	err = yaml.Unmarshal(data, &configCommon)
	if err != nil {
		logger.Println("Error unmarshaling YAML:", err)
		os.Exit(-1)
	}
	config[COMMON_PATH] = &configCommon

	trcshtalkCertBytes, err := os.ReadFile("./local_config/trcshtalk.crt")
	if err != nil {
		log.Printf("Couldn't load cert: %v", err)
	}

	trcshtalkKeyBytes, err := os.ReadFile("./local_config/trcshtalkkey.key")
	if err != nil {
		log.Printf("Couldn't load key: %v", err)
	}
	config[TRCSHTALK_CERT] = trcshtalkCertBytes
	config[TRCSHTALK_KEY] = trcshtalkKeyBytes

	Init(&config)
	configContext.Start()
	wait := make(chan bool)
	wait <- true
}

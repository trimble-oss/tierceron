package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"

	"gopkg.in/yaml.v2"

	"github.com/trimble-oss/tierceron/installation/trcshhive/trcshk/echo/echocore"
	pb "github.com/trimble-oss/tierceron/installation/trcshhive/trcshk/echo/trcshtalksdk"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/encoding/protojson"
)

type echoServiceServer struct {
	pb.UnimplementedTrcshTalkServiceServer
}

var configContext *tccore.ConfigContext
var grpcServer *grpc.Server
var dfstat *tccore.TTDINode

// Runs diagnostic services for each Diagnostic within the DiagnosticRequest.
// Returns DiagnosticResponse, forwarding the MessageId of the DiagnosticRequest,
// and providing the results of the diagnostics ran.
func (s *echoServiceServer) RunDiagnostics(ctx context.Context, req *pb.DiagnosticRequest) (*pb.DiagnosticResponse, error) {
	env := req.MessageId
	if bus, ok := echocore.GlobalEchoNetwork.Get(env); ok {
		if len(req.Data) > 0 {
			// Response
			go func(r *pb.DiagnosticRequest) {
				response := &pb.DiagnosticResponse{}
				protojson.Unmarshal([]byte(req.Data[0]), response)
				(*bus).OutChan <- response
			}(req)
			return nil, nil
		} else {
			// Asking for new queries
			for {
				newReq := <-(*bus).InChan

				jsonBytes, err := protojson.Marshal(newReq)
				if err != nil {
					fmt.Printf("Trouble serializing...")
					continue
				}
				return &pb.DiagnosticResponse{
					MessageId: req.MessageId,
					Results:   string(jsonBytes),
				}, nil
			}
		}
	}
	return nil, errors.New("Unsupported environment")
}

const (
	MASHUP_CERT = "Common/MashupCert.crt.mf.tmpl"
	COMMON_PATH = "config"
)

func GetConfigPaths() []string {
	return []string{
		COMMON_PATH,
		tccore.TRCSHHIVEK_CERT,
		tccore.TRCSHHIVEK_KEY,
		MASHUP_CERT,
	}
}

func init() {
	peerExe, err := os.Open("/usr/local/trcshk/plugins/echo.so")
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
	fmt.Printf("Echo Version: %s\n", sha)
}

func send_dfstat() {
	if configContext.DfsChan == nil || configContext == nil || dfstat == nil {
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
	if configContext.ErrorChan == nil || configContext == nil {
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

var mashupCertBytes []byte

func InitCertBytes(cert []byte) {
	mashupCertBytes = cert
}

func subdirInterceptor(subdir string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// Modify the method name to include the subdirectory
		method = subdir + "/" + method

		// Call the original invoker with the modified method name
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func getEchoRequest(serverName string, port int, diagReq *pb.DiagnosticRequest) (*pb.DiagnosticResponse, error) {
	if mashupCertBytes == nil {
		log.Printf("Cert not initialized.")
		return nil, errors.New("cert initialization failure")
	}
	mashupBlock, _ := pem.Decode([]byte(mashupCertBytes))
	mashupClientCert, err := x509.ParseCertificate(mashupBlock.Bytes)
	if err != nil {
		log.Printf("failed to serve: %v", err)
		return nil, err
	}

	mashupCertPool := x509.NewCertPool()
	mashupCertPool.AddCert(mashupClientCert)
	clientDialOptions := grpc.WithDefaultCallOptions( /* grpc.MaxCallRecvMsgSize(maxMessageLength), grpc.MaxCallSendMsgSize(maxMessageLength)*/ )

	conn, err := grpc.Dial(serverName+":"+strconv.Itoa(int(port)),
		clientDialOptions,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{ServerName: "", RootCAs: mashupCertPool, InsecureSkipVerify: true})),
		grpc.WithUnaryInterceptor(subdirInterceptor("/grpc")))

	if err != nil {
		log.Printf("getReport: fail to dial: %v", err)
		return nil, err
	}
	defer conn.Close()
	client := pb.NewTrcshTalkServiceClient(conn)

	diagRes, err := client.RunDiagnostics(context.Background(), diagReq)
	if err != nil {
		log.Printf("getReport: bad response: %v", err)
		return &pb.DiagnosticResponse{
			MessageId: diagReq.GetMessageId(),
			Results:   "Unable to obtain report from Hive.",
		}, err
	}
	log.Printf("getReport: success, response returned: %s", diagRes.Results)
	return diagRes, nil
}

func StartTrashTalking(serverName string, port int) {
	for {
		response, err := getEchoRequest(serverName, port, &pb.DiagnosticRequest{})
		if err != nil {
			fmt.Printf("Error sending response: %d\n", err.Error())
			continue
		}
		queryData := response.GetResults()
		request := &pb.DiagnosticRequest{}

		err = json.Unmarshal([]byte(queryData), &queryData)
		if err != nil {
			fmt.Printf("Error sending response: %d\n", err.Error())
			continue
		}

		talkBackResponse := EchoBack(request)
		requestResponse := &pb.DiagnosticRequest{MessageId: request.MessageId}
		requestResponse.Data[0] = talkBackResponse.Results
		_, err = getEchoRequest(serverName, port, requestResponse)
		if err != nil {
			fmt.Printf("Error sending response: %d\n", err.Error())
			continue
		}
	}
}

// The new endpoint kind of.
func EchoBack(req *pb.DiagnosticRequest) *pb.DiagnosticResponse {
	env := req.MessageId
	if bus, ok := echocore.GlobalEchoNetwork.Get(env); ok {
		if len(req.Data) > 0 {
			// Response
			go func(r *pb.DiagnosticRequest) {
				response := &pb.DiagnosticResponse{}
				protojson.Unmarshal([]byte(req.Data[0]), response)
				(*bus).OutChan <- response
			}(req)
		} else {
			// Asking for new queries
			for {
				newReq := <-(*bus).InChan

				jsonBytes, err := protojson.Marshal(newReq)
				if err != nil {
					fmt.Printf("Trouble serializing...")
					continue
				}
				return &pb.DiagnosticResponse{
					MessageId: req.MessageId,
					Results:   string(jsonBytes),
				}
			}
		}
	}
	return &pb.DiagnosticResponse{
		MessageId: req.MessageId,
		Results:   string("Unsupported environment"),
	}
}

func start() {
	if configContext == nil {
		fmt.Println("no config context initialized for echo")
		return
	}
	echocore.InitNetwork([]string{"dev"})

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
				return
			}

			if mashupCert, ok := (*configContext.ConfigCerts)[MASHUP_CERT]; ok {
				InitCertBytes(mashupCert)
			} else {
				err = errors.New("Missing mashup cert")
				configContext.Log.Printf("Missing mashup cert\n")
				send_err(err)
				return
			}

			if serverNameInterface, ok := (*configContext.Config)["grpc_server_name"]; ok {
				if serverName, ok := serverNameInterface.(string); ok {
					go StartTrashTalking(serverName, echoPort)
				}
			}
		}
	} else {
		configContext.Log.Println("Missing config: gprc_server_port")
		send_err(errors.New("missing config: gprc_server_port"))
		return
	}
}

func stop() {
	if grpcServer == nil || configContext == nil {
		fmt.Println("no server initialized for echo")
		return
	}
	configContext.Log.Println("Trcshtalk received shutdown message from kernel.")
	configContext.Log.Println("Stopping server")
	fmt.Println("Stopping server")
	grpcServer.Stop()
	fmt.Println("Stopped server")
	configContext.Log.Println("Stopped server for echo.")
	dfstat.UpdateDataFlowStatistic("System", "echo", "Shutdown", "0", 1, nil)
	send_dfstat()
	*configContext.CmdSenderChan <- tccore.PLUGIN_EVENT_STOP
	configContext = nil
	grpcServer = nil
	dfstat = nil
}

func chat_receiver(rec_chan chan *tccore.ChatMsg) {
	//not needed for echo
}

func Init(properties *map[string]interface{}) {
	var err error
	configContext, err = tccore.Init(properties,
		tccore.TRCSHHIVEK_CERT,
		tccore.TRCSHHIVEK_KEY,
		COMMON_PATH,
		"echo",
		start,
		receiver,
		chat_receiver,
	)

	// Also add mashup manually.
	if cert, ok := (*properties)[MASHUP_CERT]; ok {
		certbytes := cert.([]byte)
		(*configContext.ConfigCerts)[MASHUP_CERT] = certbytes
		fmt.Println("Attached mashup cert")
	} else {
		fmt.Println("Failed to attach mashup cert")
	}

	if err != nil {
		if configContext != nil {
			configContext.Log.Println("Failure to initialize echo.")
			fmt.Println(err.Error())
		} else {
			fmt.Println(err.Error())
		}
	}

	configContext.Log.Println("Successfully initialized echo.")
}

func main() {
	logFilePtr := flag.String("log", "./echo.log", "Output path for log file") //renamed from trchealthcheck.log
	flag.Parse()
	config := make(map[string]interface{})

	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		os.Exit(-1)
	}
	logger := log.New(f, "[echo]", log.LstdFlags)
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

	echoCertBytes, err := os.ReadFile("./local_config/hive.crt")
	if err != nil {
		log.Printf("Couldn't load cert: %v", err)
	}

	echoKeyBytes, err := os.ReadFile("./local_config/hivekey.key")
	if err != nil {
		log.Printf("Couldn't load key: %v", err)
	}
	config[tccore.TRCSHHIVEK_CERT] = echoCertBytes
	config[tccore.TRCSHHIVEK_KEY] = echoKeyBytes

	Init(&config)
	configContext.Start()
	wait := make(chan bool)
	wait <- true
}

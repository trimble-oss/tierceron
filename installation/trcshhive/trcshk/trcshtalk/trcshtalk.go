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
	"slices"
	"strconv"

	"gopkg.in/yaml.v2"

	pb "github.com/trimble-oss/tierceron/installation/trcshhive/trcshk/trcshtalk/trcshtalksdk"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type trcshtalkServiceServer struct {
	pb.UnimplementedTrcshTalkServiceServer
}

var configContext *tccore.ConfigContext
var grpcServer *grpc.Server
var dfstat *tccore.TTDINode

// Runs diagnostic services for each Diagnostic within the DiagnosticRequest.
// Returns DiagnosticResponse, forwarding the MessageId of the DiagnosticRequest,
// and providing the results of the diagnostics ran.
func (s *trcshtalkServiceServer) RunDiagnostics(ctx context.Context, req *pb.DiagnosticRequest) (*pb.DiagnosticResponse, error) {

	// TODO: Implement diagnostics plugin
	// if req.Diagnostics contains 0, run all
	// else run each
	cmds := req.GetDiagnostics()
	queries := []string{}
	tenant_test := req.GetTenantId() + ":"
	if slices.Contains(cmds, pb.Diagnostics_ALL) {
		// run all
		//set queries to all cmds
		fmt.Println("Running all queries.")
		queries = append(queries, "healthcheck")
	} else {
		for _, q := range cmds {
			if q == pb.Diagnostics_HEALTH_CHECK {
				// set name to plugin...
				fmt.Println("Running healthcheck diagnostic.")
				queries = append(queries, "healthcheck")
				// plugin_tests := req.GetData()
				// for i, test := range plugin_tests {
				// 	if i == 0 {
				// 		tenant_test = fmt.Sprintf("%s%s", tenant_test, test)
				// 	} else {
				// 		tenant_test = fmt.Sprintf("%s,%s", tenant_test, test)
				// 	}
				// }
				plugin_tests := req.GetPluginQueries()
				for i, rq := range plugin_tests {
					test := pb.PluginQuery_name[int32(rq)]
					if i == 0 {
						tenant_test = fmt.Sprintf("%s%s", tenant_test, test)
					} else {
						tenant_test = fmt.Sprintf("%s,%s", tenant_test, test)
					}
				}
			}
		}
	}
	name := "trcshtalk"
	*configContext.ChatSenderChan <- &tccore.ChatMsg{
		ChatId: &tenant_test,
		Name:   &name,
		Query:  &queries,
	}
	// Placeholder code
	results := ""
	finished_queries := make(map[string]string)
	fmt.Printf("Sent queries to kernel: %d\n", len(queries))
	for {
		event := <-*configContext.ChatReceiverChan
		fmt.Println("TrcshTalk received message from kernel.")
		switch {
		case len(finished_queries) == len(queries):
			// send results to google chat
			fmt.Println("Formatting responses from kernel.")
			for _, v := range finished_queries {
				results = results + v + " "
			}
			fmt.Printf("Sending response to chat from kernel: %s", results)
			return &pb.DiagnosticResponse{
				MessageId: req.MessageId,
				Results:   results,
			}, nil
		default:
			fmt.Printf("Received response from query: %s\n", *(*event).Query)
			if len(*(*event).Query) == 1 && event.Response != nil && (*event).Response != nil {
				fmt.Printf("Processing response from query: %s\n", *(*event).Query)
				finished_queries[(*event.Query)[0]] = *((*event).Response)
			}
			if len(finished_queries) == len(queries) {
				// send results to google chat
				fmt.Println("Formatting responses.")
				for _, v := range finished_queries {
					results = results + v + " "
				}
				fmt.Printf("Sending response to chat: %s", results)
				return &pb.DiagnosticResponse{
					MessageId: req.MessageId,
					Results:   results,
				}, nil
			}
		}
	}
	// res := &pb.DiagnosticResponse{
	// 	MessageId: req.MessageId,
	// 	Results:   "gRPCS client/server successful. Diagnostic service not yet implemented.",
	// }
	// // End placeholder code

	// return res, nil
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
	if configContext.DfsChan == nil || configContext == nil || dfstat == nil {
		fmt.Println("Dataflow Statistic channel not initialized properly for trcshtalk.")
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

func getTrcshTalkRequest(serverName string, port int, diagReq *pb.DiagnosticRequest) (*pb.DiagnosticResponse, error) {
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
	client := pb.NewDiagnosticServiceClient(conn)

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
		response, err := getTrcshTalkRequest(serverName, port, &pb.DiagnosticRequest{})
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

		talkBackResponse := TrcshTalkBack(request)
		requestResponse := &pb.DiagnosticRequest{MessageId: request.MessageId}
		requestResponse.Data[0] = talkBackResponse.Results
		_, err = getTrcshTalkRequest(serverName, port, requestResponse)
		if err != nil {
			fmt.Printf("Error sending response: %d\n", err.Error())
			continue
		}
	}
}

// The new endpoint kind of.
func TrcshTalkBack(req *pb.DiagnosticRequest) *pb.DiagnosticResponse {
	cmds := req.GetDiagnostics()
	queries := []string{}
	tenant_test := req.GetTenantId() + ":"
	if slices.Contains(cmds, pb.Diagnostics_ALL) {
		// run all
		//set queries to all cmds
		fmt.Println("Running all queries.")
		queries = append(queries, "healthcheck")
	} else {
		for _, q := range cmds {
			if q == pb.Diagnostics_HEALTH_CHECK {
				// set name to plugin..
				fmt.Println("Running healthcheck diagnostic.")
				queries = append(queries, "healthcheck")
				plugin_tests := req.GetPluginQueries()
				// plugin_tests := req.GetData()
				// for i, test := range plugin_tests {
				// 	if i == 0 {
				// 		tenant_test = fmt.Sprintf("%s%s", tenant_test, test)
				// 	} else {
				// 		tenant_test = fmt.Sprintf("%s,%s", tenant_test, test)
				// 	}
				// }
				for i, rq := range plugin_tests {
					test := pb.PluginQuery_name[int32(rq)]
					if i == 0 {
						tenant_test = fmt.Sprintf("%s%s", tenant_test, test)
					} else {
						tenant_test = fmt.Sprintf("%s,%s", tenant_test, test)
					}
				}
			}
		}
	}
	name := "trcshtalk"

	*configContext.ChatSenderChan <- &tccore.ChatMsg{
		Name:   &name,
		ChatId: &tenant_test,
		Query:  &queries,
	}
	// Placeholder code
	results := ""
	finished_queries := make(map[string]string)
	fmt.Printf("Sent queries to kernel: %d\n", len(queries))
	for {
		event := <-*configContext.ChatReceiverChan
		fmt.Println("TrcshTalk received message from kernel.")
		switch {
		case len(finished_queries) == len(queries):
			// send results to google chat
			fmt.Println("Formatting responses.")
			for _, v := range finished_queries {
				results = results + v + " "
			}
			fmt.Printf("Sending response to chat: %s", results)
			return &pb.DiagnosticResponse{
				MessageId: req.MessageId,
				Results:   results,
			}
		default:
			fmt.Printf("Received response from query: %s\n", *(*event).Query)
			if len(*(*event).Query) == 1 && event.Response != nil && (*event).Response != nil {
				fmt.Printf("Processing response from query: %s\n", *(*event).Query)
				finished_queries[(*event.Query)[0]] = *((*event).Response)
			}
			if len(finished_queries) == len(queries) {
				// send results to google chat
				fmt.Println("Formatting responses.")
				for _, v := range finished_queries {
					results = results + v + " "
				}
				fmt.Printf("Sending response to chat: %s", results)
				return &pb.DiagnosticResponse{
					MessageId: req.MessageId,
					Results:   results,
				}
			}
		}
	}
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
					go StartTrashTalking(serverName, trcshtalkPort)
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
	*configContext.CmdSenderChan <- tccore.PLUGIN_EVENT_STOP
	configContext = nil
	grpcServer = nil
	dfstat = nil
}

func chat_receiver(rec_chan chan *tccore.ChatMsg) {
	//not needed for trcshtalk
}

func Init(properties *map[string]interface{}) {
	var err error
	configContext, err = tccore.Init(properties,
		tccore.TRCSHHIVEK_CERT,
		tccore.TRCSHHIVEK_KEY,
		COMMON_PATH,
		"trcshtalk",
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
			configContext.Log.Println("Failure to initialize trcshtalk.")
			fmt.Println(err.Error())
		} else {
			fmt.Println(err.Error())
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

	trcshtalkCertBytes, err := os.ReadFile("./local_config/hive.crt")
	if err != nil {
		log.Printf("Couldn't load cert: %v", err)
	}

	trcshtalkKeyBytes, err := os.ReadFile("./local_config/hivekey.key")
	if err != nil {
		log.Printf("Couldn't load key: %v", err)
	}
	config[tccore.TRCSHHIVEK_CERT] = trcshtalkCertBytes
	config[tccore.TRCSHHIVEK_KEY] = trcshtalkKeyBytes

	Init(&config)
	configContext.Start()
	wait := make(chan bool)
	wait <- true
}

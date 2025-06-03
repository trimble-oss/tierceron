package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/plugincoreopts"

	"github.com/trimble-oss/tierceron-core/v2/core"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	pb "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcshtalk/trcshtalksdk"
	"golang.org/x/exp/rand"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/yaml.v2"
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
				plugin_tests := req.GetQueries()
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
			configContext.Log.Printf("Sending response to chat from kernel: %s\n", results)
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

func GetConfigContext(pluginName string) *tccore.ConfigContext { return configContext }

func GetConfigPaths(pluginName string) []string {
	return []string{
		COMMON_PATH,
		tccore.TRCSHHIVEK_CERT,
		tccore.TRCSHHIVEK_KEY,
		MASHUP_CERT,
	}
}

func Init(pluginName string, properties *map[string]interface{}) {
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
	if err != nil {
		if configContext != nil {
			configContext.Log.Println("Some trouble initializing trcshtalk.")
			fmt.Println(err.Error())
		} else {
			fmt.Println(err.Error())
			return
		}
	}

	// Change logging context
	configContext.Log = log.New(configContext.Log.Writer(), "[trcshtalk]", log.LstdFlags)

	// Also add mashup manually.
	if cert, ok := (*properties)[MASHUP_CERT]; ok {
		certbytes := cert.([]byte)
		(*configContext.ConfigCerts)[MASHUP_CERT] = certbytes
		configContext.Log.Println("Attached mashup cert")
	} else {
		configContext.Log.Println("Failed to attach mashup cert")
	}

	if err != nil {
		if configContext != nil {
			configContext.Log.Println("Failure to initialize trcshtalk.")
			configContext.Log.Println(err.Error())
		} else {
			configContext.Log.Println(err.Error())
		}
	}

	configContext.Log.Println("Successfully initialized trcshtalk.")

}

func init() {
	if plugincoreopts.BuildOptions.IsPluginHardwired() {
		return
	}
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
	dfstatClone := *dfstat
	go func(dsc *tccore.TTDINode) {
		*configContext.DfsChan <- dsc
	}(&dfstatClone)
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
		method = subdir + method

		// Call the original invoker with the modified method name
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func GenMsgId(env string) string {
	rand.Seed(uint64(time.Now().UnixNano()))
	randomNumber := rand.Intn(10000000) // Generate a number between 0 and 10000000
	return fmt.Sprintf("%s:%d", env, randomNumber)
}

func getTrcshTalkRequest(serverName string, port int, ttbToken *string, isRemote bool, diagReq *pb.DiagnosticRequest) (*pb.DiagnosticResponse, error) {
	diagReq.MessageId = GenMsgId(configContext.Env)

	if isRemote {
		configContext.Log.Printf("TrcshTalking remote\n")
		retryCount := 0
		// Make a post request... json serialize and send it...
		requestBytes, err := protojson.Marshal(diagReq)
		if err != nil {
			configContext.Log.Printf("Error marshalling request: %s.  Undeliverable request/reponse.\n", err.Error())
			return nil, err
		}
		req, err := http.NewRequest("POST", fmt.Sprintf("https://%s/grpc", serverName), bytes.NewBuffer(requestBytes))
		if err != nil {
			configContext.Log.Printf("Post failure.  Unrecoverable.\n")
			return nil, err
		}

		// Set the Content-Type header to application/json
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", *ttbToken)

	retrypostgrpc:
		// Send the request using http.DefaultClient
		client := &http.Client{
			Timeout: time.Minute,
		}
		configContext.Log.Printf("TrcshTalk posting request\n")
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			configContext.Log.Printf("Post grpc failure.\n")

			// May time out...
			if retryCount < 5 {
				time.Sleep(time.Second * 3)
				configContext.Log.Printf("Post failure.  Trying again.\n")
				retryCount = retryCount + 1
				goto retrypostgrpc
			} else {
				return nil, err
			}
		}
		configContext.Log.Printf("TrcshTalk got response for request\n")
		defer resp.Body.Close()

		diagRes := &pb.DiagnosticResponse{}

		// Read and print the response
		respData, err := io.ReadAll(resp.Body)
		if err != nil {
			configContext.Log.Printf("Error reading post grpc response: %s\n", err.Error())
			if retryCount < 5 {
				time.Sleep(time.Second * 3)
				configContext.Log.Printf("Post failure.  Trying again.\n")
				retryCount = retryCount + 1
				goto retrypostgrpc
			} else {
				return nil, err
			}
		}
		err = protojson.Unmarshal(respData, diagRes)
		if err != nil {
			configContext.Log.Printf("Error unmarshalling post grpc response: %s\n", err.Error())
			if retryCount < 5 {
				time.Sleep(time.Second * 3)
				configContext.Log.Printf("Post failure.  Trying again.\n")
				retryCount = retryCount + 1
				goto retrypostgrpc
			} else {
				return nil, err
			}
		}

		configContext.Log.Printf("getTrcshTalkRequest: success, response returned: %s\n", diagRes.Results)

		return diagRes, nil
	} else {
		configContext.Log.Printf("TrcshTalking in hardwired\n")
		if mashupCertBytes == nil {
			configContext.Log.Printf("Cert not initialized.\n")
			return nil, errors.New("cert initialization failure")
		}
		mashupBlock, _ := pem.Decode([]byte(mashupCertBytes))
		mashupClientCert, err := x509.ParseCertificate(mashupBlock.Bytes)
		if err != nil {
			configContext.Log.Printf("failed to serve: %v\n", err)
			return nil, err
		}

		mashupCertPool := x509.NewCertPool()
		mashupCertPool.AddCert(mashupClientCert)
		clientDialOptions := grpc.WithDefaultCallOptions( /* grpc.MaxCallRecvMsgSize(maxMessageLength), grpc.MaxCallSendMsgSize(maxMessageLength)*/ )
		conn, err := grpc.Dial(serverName+":"+strconv.Itoa(int(port)),
			clientDialOptions,
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{ServerName: serverName, RootCAs: mashupCertPool, InsecureSkipVerify: true})),
			/*grpc.WithUnaryInterceptor(subdirInterceptor(subdir))*/)

		if err != nil {
			log.Printf("getTrcshTalkRequest: fail to dial: %v", err)
			return nil, err
		}
		defer conn.Close()
		client := pb.NewTrcshTalkServiceClient(conn)

		diagRes, err := client.RunDiagnostics(context.Background(), diagReq)
		if err != nil {
			log.Printf("getTrcshTalkRequest: bad response: %v", err)
			return nil, err
		}
		log.Printf("getTrcshTalkRequest: success, response returned: %s", diagRes.Results)
		return diagRes, nil
	}
}

func StartTrashTalking(remoteServerName string, port int, ttbToken *string, isRemote bool) {
	for {
		log.Printf("Trash talking...\n")
		response, err := getTrcshTalkRequest(remoteServerName, port, ttbToken, isRemote, &pb.DiagnosticRequest{})
		if err != nil {
			log.Printf("Error sending response: %s\n", err.Error())

			if s, ok := status.FromError(err); ok {
				if s.Code() == codes.Unavailable {
					time.Sleep(10 * time.Second)
				}
			}
			continue
		}
		queryData := response.GetResults()
		request := &pb.DiagnosticRequest{}

		err = protojson.Unmarshal([]byte(queryData), request)
		if err != nil {
			fmt.Printf("Error sending response: %s\n", err.Error())
			continue
		}

		retryCount := 0
		talkBackResponse := TrcshTalkBack(request)
		requestResponse := &pb.DiagnosticRequest{MessageId: request.MessageId, Data: []string{"None"}}
		requestResponse.Data[0] = talkBackResponse.Results
	retrytalkback:
		_, err = getTrcshTalkRequest(remoteServerName, port, ttbToken, isRemote, requestResponse)
		if err != nil {
			log.Printf("Error sending response: %s\n", err.Error())
			if retryCount < 5 {
				time.Sleep(time.Second * 3)
				log.Printf("Trying again...\n")
				retryCount = retryCount + 1
				goto retrytalkback
			} else {
				continue
			}
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
		log.Println("Running all queries.")
		queries = append(queries, "healthcheck")
	} else {
		for _, q := range cmds {
			if q == pb.Diagnostics_HEALTH_CHECK {
				// set name to plugin...
				log.Println("Running healthcheck diagnostic.")
				queries = append(queries, "healthcheck")
				plugin_tests := req.GetQueries()
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
			configContext.Log.Printf("Sending response to chat: %s\n", results)
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
				fmt.Printf("Sending response to chat: %s\n", results)
				return &pb.DiagnosticResponse{
					MessageId: req.MessageId,
					Results:   results,
				}
			}
		}
	}
}

func start(pluginName string) {
	if configContext == nil {
		fmt.Println("no config context initialized for trcshtalk")
		return
	}
	isRemote := true
	if portInterface, ok := (*configContext.Config)["grpc_server_port"]; ok {
		var trcshtalkPort int
		var server_mode string = "standard"
		if port, ok := portInterface.(int); ok {
			trcshtalkPort = port
		} else {
			var err error
			trcshtalkPort, err = strconv.Atoi(portInterface.(string))
			if err != nil {
				fmt.Printf("Failed to process server port: %v\n", err)
				send_err(err)
				return
			}
		}

		if modeInterface, ok := (*configContext.Config)["server_mode"]; ok {
			if m, ok := modeInterface.(string); ok {
				server_mode = m
			}
		}

		if server_mode == "trcshtalkback" || server_mode == "talkback-kernel-plugin" || server_mode == "both" {
			var ok bool
			var clientCert []byte

			if server_mode == "talkback-kernel-plugin" {
				isRemote = false
				clientCert, ok = (*configContext.ConfigCerts)[tccore.TRCSHHIVEK_CERT]
			} else {
				clientCert, ok = (*configContext.ConfigCerts)[MASHUP_CERT]
			}
			var trcshtalkbackPort int

			if !isRemote {
				if portInterface, ok := (*configContext.Config)["grpc_server_remote_port"]; ok {
					if port, ok := portInterface.(int); ok {
						trcshtalkbackPort = port
					} else {
						var err error
						trcshtalkbackPort, err = strconv.Atoi(portInterface.(string))
						if err != nil {
							configContext.Log.Printf("Failed to process server port: %v\n", err)
							send_err(err)
							return
						}
					}
				}
			}

			if ok {
				InitCertBytes(clientCert)
			} else {
				err := errors.New("missing mashup cert")
				fmt.Printf("Missing mashup cert\n")
				send_err(err)
				return
			}
			if serverNameInterface, ok := (*configContext.Config)["grpc_server_remote_name"]; ok {
				if remoteServerName, ok := serverNameInterface.(string); ok {
					if ttbTokenInterface, ok := (*configContext.Config)["ttb_token"]; ok {
						if ttbToken, ok := ttbTokenInterface.(string); ok {
							go func(ttbt *string, cmd_send_chan *chan core.KernelCmd) {
								*cmd_send_chan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_START}
								StartTrashTalking(remoteServerName, trcshtalkbackPort, ttbt, isRemote)
							}(&ttbToken, configContext.CmdSenderChan)
						}
					}
				}
			}
		}

		if server_mode == "standard" || server_mode == "both" {
			fmt.Printf("Server listening on :%d\n", trcshtalkPort)
			lis, gServer, err := InitServer(trcshtalkPort,
				(*configContext.ConfigCerts)[tccore.TRCSHHIVEK_CERT],
				(*configContext.ConfigCerts)[tccore.TRCSHHIVEK_KEY])
			if err != nil {
				fmt.Printf("Failed to start server: %v\n", err)
				send_err(err)
				return
			}
			fmt.Println("Starting server")

			grpcServer = gServer
			grpc_health_v1.RegisterHealthServer(grpcServer, health.NewServer())
			pb.RegisterTrcshTalkServiceServer(grpcServer, &trcshtalkServiceServer{})
			// reflection.Register(grpcServer)
			configContext.Log.Printf("server listening at %v\n", lis.Addr())
			go func(l net.Listener, cmd_send_chan *chan core.KernelCmd) {
				*cmd_send_chan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_START}
				if err := grpcServer.Serve(l); err != nil {
					fmt.Println("Failed to serve:", err)
					send_err(err)
					return
				}
			}(lis, configContext.CmdSenderChan)
			dfstat = tccore.InitDataFlow(nil, configContext.ArgosId, false)
			dfstat.UpdateDataFlowStatistic("System",
				"TrcshTalk",
				"Start up",
				"1",
				1,
				func(msg string, err error) {
					configContext.Log.Println(msg, err)
				})
			send_dfstat()
		}
	} else {
		configContext.Log.Println("Missing config: gprc_server_port")
		send_err(errors.New("missing config: gprc_server_port"))
		return
	}
}

func stop(pluginName string) {
	if grpcServer == nil || configContext == nil {
		fmt.Println("no server initialized for trcshtalk")
		return
	}
	configContext.Log.Println("Trcshtalk received shutdown message from kernel.")
	configContext.Log.Println("Stopping server")
	grpcServer.Stop()
	configContext.Log.Println("Stopped server")
	configContext.Log.Println("Stopped server for trcshtalk.")
	dfstat.UpdateDataFlowStatistic("System", "trcshtalk", "Shutdown", "0", 1, nil)
	send_dfstat()
	*configContext.CmdSenderChan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_STOP}
	configContext = nil
	grpcServer = nil
	dfstat = nil
}

func chat_receiver(rec_chan chan *tccore.ChatMsg) {
	//not needed for trcshtalk
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

	Init("trcshtalk", &config)
	configContext.Start("trcshtalk")
	wait := make(chan bool)
	wait <- true
}

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
	"strings"
	"sync"
	"time"

	"github.com/townsendmerino/goinfer/chat"
	"github.com/townsendmerino/goinfer/decoder"
	"github.com/townsendmerino/goinfer/tokenizer"
	"github.com/trimble-oss/tierceron-core/v2/buildopts/plugincoreopts"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	ttsdk "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcshtalk/trcshtalksdk"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	configContext *tccore.ConfigContext
	sender        chan error
	dfstat        *tccore.TTDINode
	grpcServer    *grpc.Server
	localModel    *localModelRuntime
	proxyRequests chan *ttsdk.DiagnosticRequest = make(chan *ttsdk.DiagnosticRequest, 128)
	proxyReplies  sync.Map
)

const (
	COMMON_PATH                 = "./config.yml"
	hubClientPluginsMetadataKey = "x-trc-supported-deployments"
	hubClientRequestTimeout     = 30 * time.Second
)

type localModelRuntime struct {
	maxTokens int
	sampling  decoder.SamplingParams
	model     *decoder.Model
	tokenizer *tokenizer.Tokenizer
	template  *chat.Template
}

type diagnosticsServiceServer struct {
	ttsdk.UnimplementedTrcshTalkServiceServer
}

func receiver(receive_chan chan tccore.KernelCmd) {
	for {
		event := <-receive_chan
		switch {
		case event.Command == tccore.PLUGIN_EVENT_START:
			go start(event.PluginName)
		case event.Command == tccore.PLUGIN_EVENT_STOP:
			go stop(event.PluginName)
			sender <- errors.New("vico shutting down")
			return
		case event.Command == tccore.PLUGIN_EVENT_STATUS:
			// TODO
		default:
			// TODO
		}
	}
}

func init() {
	if plugincoreopts.BuildOptions.IsPluginHardwired() {
		return
	}

	peerExe, err := os.Open("plugins/vico.so")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Vico unable to sha256 plugin")
		return
	}

	defer peerExe.Close()

	h := sha256.New()
	if _, err := io.Copy(h, peerExe); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to copy file for sha256 of plugin: %s\n", err)
		return
	}
	sha := hex.EncodeToString(h.Sum(nil))
	fmt.Fprintf(os.Stderr, "vico Version: %s\n", sha)
}

func send_dfstat() {
	if configContext == nil || configContext.DfsChan == nil || dfstat == nil {
		fmt.Fprintln(os.Stderr, "Dataflow Statistic channel not initialized properly for vico.")
		return
	}
	dfsctx, _, err := dfstat.GetDeliverStatCtx()
	if err != nil {
		configContext.Log.Println("Failed to get dataflow statistic context: ", err)
		send_err(err)
		return
	}
	tccore.SendDfStat(configContext, dfsctx, dfstat)
}

func send_err(err error) {
	if configContext == nil || configContext.ErrorChan == nil || err == nil {
		fmt.Fprintln(os.Stderr, "Failure to send error message, error channel not initialized properly for vico.")
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

func validateIncomingTTBToken(ctx context.Context) error {
	if configContext == nil || configContext.Config == nil {
		return errors.New("missing config context")
	}
	expectedToken, _ := (*configContext.Config)["ttb_token"].(string)
	expectedToken = strings.TrimSpace(expectedToken)
	if expectedToken == "" {
		return nil
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return errors.New("missing incoming metadata")
	}
	for _, value := range md.Get("authorization") {
		if strings.TrimSpace(value) == expectedToken {
			return nil
		}
	}
	return errors.New("invalid talkback token")
}

func promptFromDiagnosticRequest(req *ttsdk.DiagnosticRequest) string {
	if req == nil {
		return ""
	}
	promptParts := make([]string, 0, len(req.GetData()))
	for _, data := range req.GetData() {
		trimmed := strings.TrimSpace(data)
		if trimmed != "" {
			promptParts = append(promptParts, trimmed)
		}
	}
	if len(promptParts) > 0 {
		return strings.Join(promptParts, "\n")
	}
	return strings.TrimSpace(req.GetQueryId())
}

func localModelActiveCount() string {
	if localModel != nil {
		return "1"
	}
	return "0"
}

func normalizePluginName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func isLocalPlugin(name string) bool {
	switch normalizePluginName(name) {
	case "vico":
		return true
	default:
		return false
	}
}

func targetPluginForEvent(event *tccore.ChatMsg) string {
	if event == nil || event.Query == nil || len(*event.Query) != 1 {
		return ""
	}
	if event.Response != nil && strings.TrimSpace(*event.Response) != "" {
		return ""
	}
	targetPlugin := normalizePluginName((*event.Query)[0])
	if targetPlugin == "" || targetPlugin == "trcshcmd" || isLocalPlugin(targetPlugin) {
		return ""
	}
	if event.Name != nil && normalizePluginName(*event.Name) == targetPlugin {
		return ""
	}
	return targetPlugin
}

func supportedPluginsFromIncomingContext(ctx context.Context) map[string]struct{} {
	if ctx == nil {
		return nil
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil
	}
	plugins := map[string]struct{}{}
	for _, value := range md.Get(hubClientPluginsMetadataKey) {
		for _, plugin := range strings.Split(value, ",") {
			plugin = normalizePluginName(plugin)
			if plugin != "" {
				plugins[plugin] = struct{}{}
			}
		}
	}
	if len(plugins) == 0 {
		return nil
	}
	return plugins
}

func requestSupportedByPlugins(req *ttsdk.DiagnosticRequest, supportedPlugins map[string]struct{}) bool {
	targetPlugin := normalizePluginName(req.GetQueryId())
	if targetPlugin == "" {
		return true
	}
	if len(supportedPlugins) == 0 {
		return false
	}
	_, ok := supportedPlugins[targetPlugin]
	return ok
}

func enqueueProxyRequest(ctx context.Context, req *ttsdk.DiagnosticRequest) (*ttsdk.DiagnosticResponse, error) {
	responseChan := make(chan *ttsdk.DiagnosticResponse, 1)
	proxyReplies.Store(req.GetMessageId(), responseChan)
	defer proxyReplies.Delete(req.GetMessageId())

	select {
	case proxyRequests <- req:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	select {
	case response := <-responseChan:
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func dequeueProxyRequest(ctx context.Context, req *ttsdk.DiagnosticRequest) (*ttsdk.DiagnosticResponse, error) {
	supportedPlugins := supportedPluginsFromIncomingContext(ctx)
	var proxyRequest *ttsdk.DiagnosticRequest
	pendingRequests := len(proxyRequests)
	if pendingRequests == 0 {
		select {
		case proxyRequest = <-proxyRequests:
			if !requestSupportedByPlugins(proxyRequest, supportedPlugins) {
				proxyRequests <- proxyRequest
				return &ttsdk.DiagnosticResponse{MessageId: req.GetMessageId(), Results: ""}, nil
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	} else {
		for i := 0; i < pendingRequests; i++ {
			proxyRequest = <-proxyRequests
			if requestSupportedByPlugins(proxyRequest, supportedPlugins) {
				break
			}
			proxyRequests <- proxyRequest
			proxyRequest = nil
		}
		if proxyRequest == nil {
			return &ttsdk.DiagnosticResponse{MessageId: req.GetMessageId(), Results: ""}, nil
		}
	}

	requestBytes, err := protojson.Marshal(proxyRequest)
	if err != nil {
		return nil, err
	}
	return &ttsdk.DiagnosticResponse{MessageId: req.GetMessageId(), Results: string(requestBytes)}, nil
}

func postProxyResponse(req *ttsdk.DiagnosticRequest) (*ttsdk.DiagnosticResponse, bool) {
	if len(req.GetData()) == 0 {
		return nil, false
	}
	responseChanValue, ok := proxyReplies.Load(req.GetMessageId())
	if !ok {
		return nil, false
	}
	response := &ttsdk.DiagnosticResponse{MessageId: req.GetMessageId(), Results: req.GetData()[0]}
	responseChanValue.(chan *ttsdk.DiagnosticResponse) <- response
	return &ttsdk.DiagnosticResponse{MessageId: req.GetMessageId(), Results: "Response posted"}, true
}

func buildProxyDiagnosticRequest(event *tccore.ChatMsg, targetPlugin string) *ttsdk.DiagnosticRequest {
	messageID := fmt.Sprintf("vico:%d", time.Now().UnixNano())
	if event.RoutingId != nil && strings.TrimSpace(*event.RoutingId) != "" {
		messageID = strings.TrimSpace(*event.RoutingId)
	} else {
		event.RoutingId = &messageID
	}

	data := []string{}
	if event.ChatId != nil && strings.TrimSpace(*event.ChatId) != "" {
		data = append(data, strings.TrimSpace(*event.ChatId))
	}
	if prompt := extractPrompt(event); prompt != "" {
		data = append(data, prompt)
	}

	return &ttsdk.DiagnosticRequest{
		MessageId: messageID,
		QueryId:   targetPlugin,
		Data:      data,
	}
}

func forwardToHubClient(event *tccore.ChatMsg, targetPlugin string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hubClientRequestTimeout)
	defer cancel()

	response, err := enqueueProxyRequest(ctx, buildProxyDiagnosticRequest(event, targetPlugin))
	if err != nil {
		return "", err
	}
	if response == nil {
		return "", nil
	}
	return response.GetResults(), nil
}

func isHubClientPoll(req *ttsdk.DiagnosticRequest) bool {
	if req == nil {
		return false
	}
	return len(req.GetData()) == 0 && len(req.GetQueries()) == 0 && strings.TrimSpace(req.GetQueryId()) == ""
}

// Runs Vico diagnostics for the shared trcshtalk RPC contract.
func (s *diagnosticsServiceServer) RunDiagnostics(ctx context.Context, req *ttsdk.DiagnosticRequest) (*ttsdk.DiagnosticResponse, error) {
	if err := validateIncomingTTBToken(ctx); err != nil {
		if configContext != nil {
			configContext.Log.Printf("Rejecting RunDiagnostics request: %v", err)
		}
		return nil, status.Error(codes.Unauthenticated, "invalid talkback token")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "missing diagnostic request")
	}
	if response, handled := postProxyResponse(req); handled {
		return response, nil
	}
	if isHubClientPoll(req) {
		return dequeueProxyRequest(ctx, req)
	}

	prompt := promptFromDiagnosticRequest(req)
	if prompt == "" {
		for _, query := range req.GetQueries() {
			if query == ttsdk.PluginQuery_ACTIVE_COUNT {
				return &ttsdk.DiagnosticResponse{MessageId: req.GetMessageId(), Results: localModelActiveCount()}, nil
			}
		}
		return &ttsdk.DiagnosticResponse{MessageId: req.GetMessageId(), Results: ""}, nil
	}

	response, err := queryLocalModel(prompt)
	if err != nil {
		if configContext != nil {
			configContext.Log.Printf("vico RunDiagnostics failed: %s\n", tccore.SanitizeForLogging(err.Error()))
		}
		send_err(err)
		return nil, status.Error(codes.Unavailable, "vico local model unavailable")
	}

	return &ttsdk.DiagnosticResponse{MessageId: req.GetMessageId(), Results: response}, nil
}

func loadTokenizer(modelPath string) (*tokenizer.Tokenizer, error) {
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(modelPath)), ".gguf") {
		return tokenizer.LoadGGUF(modelPath)
	}
	return tokenizer.Load(modelPath)
}

func initLocalModel() error {
	closeLocalModel()
	if configContext == nil || configContext.Config == nil {
		return errors.New("missing config context")
	}

	modelPath := ""
	if val, ok := (*configContext.Config)["local_model_path"]; ok && val != nil {
		path, ok := val.(string)
		if !ok {
			return fmt.Errorf("invalid local_model_path type %T", val)
		}
		modelPath = strings.TrimSpace(path)
	}
	if modelPath == "" {
		return errors.New("local_model_path is empty")
	}

	modelBackend := ""
	if val, ok := (*configContext.Config)["local_model_backend"]; ok && val != nil {
		backend, ok := val.(string)
		if !ok {
			return fmt.Errorf("invalid local_model_backend type %T", val)
		}
		modelBackend = strings.TrimSpace(backend)
	}

	maxTokens := 0
	if val, ok := (*configContext.Config)["local_model_max_tokens"]; ok && val != nil {
		switch typedVal := val.(type) {
		case int:
			maxTokens = typedVal
		case string:
			parsedMaxTokens, err := strconv.Atoi(strings.TrimSpace(typedVal))
			if err != nil {
				return err
			}
			maxTokens = parsedMaxTokens
		default:
			return fmt.Errorf("invalid local_model_max_tokens type %T", val)
		}
	}
	if maxTokens <= 0 {
		maxTokens = 256
	}

	temperature := 0.2
	if val, ok := (*configContext.Config)["local_model_temperature"]; ok && val != nil {
		switch typedVal := val.(type) {
		case float64:
			temperature = typedVal
		case string:
			parsedTemperature, err := strconv.ParseFloat(strings.TrimSpace(typedVal), 64)
			if err != nil {
				return err
			}
			temperature = parsedTemperature
		default:
			return fmt.Errorf("invalid local_model_temperature type %T", val)
		}
	}

	topK := 40
	if val, ok := (*configContext.Config)["local_model_top_k"]; ok && val != nil {
		switch typedVal := val.(type) {
		case int:
			topK = typedVal
		case string:
			parsedTopK, err := strconv.Atoi(strings.TrimSpace(typedVal))
			if err != nil {
				return err
			}
			topK = parsedTopK
		default:
			return fmt.Errorf("invalid local_model_top_k type %T", val)
		}
	}

	topP := 0.9
	if val, ok := (*configContext.Config)["local_model_top_p"]; ok && val != nil {
		switch typedVal := val.(type) {
		case float64:
			topP = typedVal
		case string:
			parsedTopP, err := strconv.ParseFloat(strings.TrimSpace(typedVal), 64)
			if err != nil {
				return err
			}
			topP = parsedTopP
		default:
			return fmt.Errorf("invalid local_model_top_p type %T", val)
		}
	}

	modelOptions := decoder.Options{Backend: modelBackend}
	if err := modelOptions.Validate(); err != nil {
		return err
	}

	model, err := decoder.Load(modelPath, modelOptions)
	if err != nil {
		return err
	}

	tok, err := loadTokenizer(modelPath)
	if err != nil {
		model.Close()
		return err
	}

	var template *chat.Template
	if detectedTemplate, err := chat.Detect(chat.Meta{ChatTemplate: tok.ChatTemplate(), HasToken: tok.Has}); err == nil {
		template = detectedTemplate
	} else if !errors.Is(err, chat.ErrUnknownTemplate) {
		configContext.Log.Printf("vico local model chat template detection failed: %s\n", tccore.SanitizeForLogging(err.Error()))
	}

	localModel = &localModelRuntime{
		maxTokens: maxTokens,
		sampling: decoder.SamplingParams{
			Temperature: temperature,
			TopK:        topK,
			TopP:        topP,
		},
		model:     model,
		tokenizer: tok,
		template:  template,
	}

	return nil
}

func closeLocalModel() {
	if localModel != nil && localModel.model != nil {
		if err := localModel.model.Close(); err != nil && configContext != nil && configContext.Log != nil {
			configContext.Log.Printf("vico local model shutdown failed: %s\n", tccore.SanitizeForLogging(err.Error()))
		}
	}
	localModel = nil
}

func InitServer(port int, certBytes []byte, keyBytes []byte) (net.Listener, *grpc.Server, error) {
	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("key pair: %w", err)
	}
	creds := credentials.NewServerTLSFromCert(&cert)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, nil, err
	}
	return lis, grpc.NewServer(grpc.Creds(creds)), nil
}

func startCore(pluginName string, registerFn func(grpcServer *grpc.Server)) (string, *grpc.Server, *tccore.TTDINode, error) {
	if configContext == nil {
		return "", nil, nil, errors.New("nil config context")
	}

	portInterface, ok := (*configContext.Config)["grpc_server_port"]
	if !ok {
		configContext.Log.Println("Missing config: grpc_server_port")
		return "", nil, nil, errors.New("missing config: grpc_server_port")
	}

	var port int
	switch v := portInterface.(type) {
	case int:
		port = v
	case string:
		parsedPort, err := strconv.Atoi(v)
		if err != nil {
			return "", nil, nil, err
		}
		port = parsedPort
	default:
		return "", nil, nil, errors.New("invalid port type")
	}

	configContext.Log.Printf("Server listening on :%d\n", port)
	lis, gServer, err := InitServer(
		port,
		(*configContext.ConfigCerts)[tccore.TRCSHHIVEK_CERT],
		(*configContext.ConfigCerts)[tccore.TRCSHHIVEK_KEY],
	)
	if err != nil {
		configContext.Log.Printf("Failed to start server: %v\n", err)
		return "", nil, nil, err
	}

	grpc_health_v1.RegisterHealthServer(gServer, health.NewServer())
	if registerFn != nil {
		registerFn(gServer)
	}
	configContext.Log.Printf("server listening at %v\n", lis.Addr())
	go func(l net.Listener) {
		*configContext.CmdSenderChan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_START}
		if err := gServer.Serve(l); err != nil {
			configContext.Log.Println("Failed to serve:", err)
			send_err(err)
		}
	}(lis)

	startedDF := tccore.InitDataFlow(nil, configContext.ArgosId, false)
	startedDF.UpdateDataFlowStatistic("System",
		pluginName,
		"Start up",
		"1",
		1,
		func(msg string, err error) {
			configContext.Log.Println(msg, err)
		})
	dfstat = startedDF
	send_dfstat()

	return lis.Addr().String(), gServer, startedDF, nil
}

func extractPrompt(event *tccore.ChatMsg) string {
	if event == nil {
		return ""
	}
	if event.Response != nil && strings.TrimSpace(*event.Response) != "" && *event.Response != "Service unavailable" {
		return strings.TrimSpace(*event.Response)
	}
	switch promptValue := event.HookResponse.(type) {
	case string:
		return strings.TrimSpace(promptValue)
	case []string:
		return strings.TrimSpace(strings.Join(promptValue, "\n"))
	case map[string]any:
		if prompt, ok := promptValue["prompt"].(string); ok {
			return strings.TrimSpace(prompt)
		}
	}
	return ""
}

func encodePrompt(runtime *localModelRuntime, prompt string) ([]int, error) {
	if runtime == nil || runtime.tokenizer == nil {
		return nil, errors.New("vico local model is not initialized")
	}
	if runtime.template != nil {
		return runtime.tokenizer.EncodeSegments(runtime.template.RenderSegments("", []chat.Turn{{Role: "user", Content: prompt}}), false)
	}
	return runtime.tokenizer.Encode(prompt, true)
}

func queryLocalModel(prompt string) (string, error) {
	if localModel == nil || localModel.model == nil || localModel.tokenizer == nil {
		return "", errors.New("vico local model is not configured")
	}
	encodedPrompt, err := encodePrompt(localModel, prompt)
	if err != nil {
		return "", err
	}
	responseChan, generation := localModel.model.Generate(context.Background(), encodedPrompt, localModel.maxTokens, localModel.sampling)
	responseTokens := make([]int, 0, localModel.maxTokens)
	for tokenID := range responseChan {
		responseTokens = append(responseTokens, tokenID)
	}
	if generation != nil && generation.Err() != nil {
		return "", generation.Err()
	}
	response, err := localModel.tokenizer.DecodeContinuation(responseTokens)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(response), nil
}

func chat_receiver(chat_receive_chan chan *tccore.ChatMsg) {
	for {
		event := <-chat_receive_chan
		switch {
		case event == nil:
			continue
		case event.Name != nil && *event.Name == "SHUTDOWN":
			configContext.Log.Println("vico shutting down message receiver")
			return
		case event.Response != nil && *(*event).Response == "Service unavailable":
			configContext.Log.Println("Vico unable to access chat service.")
			return
		case event.ChatId != nil && (*event).ChatId != nil && *event.ChatId == "PROGRESS":
			configContext.Log.Println("Sending progress results back to kernel.")
			progressResp := "Running Vico Diagnostics..."
			(*event).Response = &progressResp
			*configContext.ChatSenderChan <- event
		case targetPluginForEvent(event) != "":
			targetPlugin := targetPluginForEvent(event)
			configContext.Log.Printf("Forwarding Vico diagnostic request to hubclient for plugin %s\n", targetPlugin)
			response, err := forwardToHubClient(event, targetPlugin)
			if err != nil {
				configContext.Log.Printf("vico hubclient forwarding failed for %s: %s\n", targetPlugin, tccore.SanitizeForLogging(err.Error()))
				response = fmt.Sprintf("No hubclient response for plugin %s.", targetPlugin)
			}
			(*event).Response = &response
			*configContext.ChatSenderChan <- event
		case extractPrompt(event) != "":
			prompt := extractPrompt(event)
			configContext.Log.Println("vico local model request")
			response, err := queryLocalModel(prompt)
			if err != nil {
				configContext.Log.Printf("vico local model request failed: %s\n", tccore.SanitizeForLogging(err.Error()))
				response = "Vico local model unavailable."
			}
			(*event).Response = &response
			*configContext.ChatSenderChan <- event
		default:
			configContext.Log.Println("vico received chat message")
		}
	}
}

func start(pluginName string) {
	if configContext == nil {
		fmt.Fprintln(os.Stderr, "no config context initialized for vico")
		return
	}

	// cfg, err := loadPluginConfig()
	// if err != nil {
	// 	configContext.Log.Println("Missing common configs")
	// 	send_err(err)
	// 	return
	// }

	_, gServer, startedDF, err := startCore(pluginName, func(gs *grpc.Server) {
		ttsdk.RegisterTrcshTalkServiceServer(gs, &diagnosticsServiceServer{})
	})
	if err != nil {
		send_err(err)
		return
	}
	grpcServer = gServer
	dfstat = startedDF

	if err := initLocalModel(); err != nil {
		configContext.Log.Printf("vico local model startup failed: %s\n", tccore.SanitizeForLogging(err.Error()))
		// send_err(err)
	} else if localModel != nil {
		configContext.Log.Println("vico local model ready")
	}
}

func stop(pluginName string) {
	if configContext != nil {
		configContext.Log.Println("vico received shutdown message from kernel.")
		configContext.Log.Println("Stopping server")
	}
	if grpcServer != nil {
		grpcServer.Stop()
		grpcServer = nil
	}
	closeLocalModel()
	if configContext != nil {
		configContext.Log.Println("Stopped server for vico.")
		if dfstat != nil {
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
		}
		*configContext.CmdSenderChan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_STOP}
	}
	dfstat = nil
	proxyRequests = make(chan *ttsdk.DiagnosticRequest, 128)
	proxyReplies = sync.Map{}
}

func GetConfigContext(pluginName string) *tccore.ConfigContext { return configContext }

func GetConfigPaths(pluginName string) []string {
	return []string{
		COMMON_PATH,
		tccore.TRCSHHIVEK_CERT,
		tccore.TRCSHHIVEK_KEY,
	}
}

func PostInit(configContext *tccore.ConfigContext) {
	configContext.Start = start
	sender = *configContext.ErrorChan
	go receiver(*configContext.CmdReceiverChan)
}

func Init(pluginName string, properties *map[string]any) {
	var err error

	configContext, err = tccore.Init(
		properties,
		tccore.TRCSHHIVEK_CERT,
		tccore.TRCSHHIVEK_KEY,
		COMMON_PATH,
		"vico",
		start,
		receiver,
		chat_receiver,
	)
	if err != nil {
		(*properties)["log"].(*log.Logger).Printf("Initialization error: %v", err)
		return
	}
	if _, ok := (*properties)[COMMON_PATH]; !ok {
		fmt.Fprintln(os.Stderr, "Missing common config components")
		return
	}
}

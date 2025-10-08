package tbtapi

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	echocore "github.com/trimble-oss/tierceron/installation/trcshhive/trcshk/echo/echocore" // Update package path as needed
	ttsdk "github.com/trimble-oss/tierceron/installation/trcshhive/trcshk/echo/trcshtalksdk"
	util "github.com/trimble-oss/tierceron/installation/trcshhive/trcshk/echo/util" // Update package path as needed
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"

	"google.golang.org/api/chat/v1"
	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/encoding/protojson"
)

// const scope = "https://www.googleapis.com/auth/chat.pbot" 	// (no admin approval required)
//

var httpServer *http.Server

var configContext *tccore.ConfigContext

func SetConfigContext(cc *tccore.ConfigContext) {
	configContext = cc
}

func sendResults(credentials string, chatSpace string, diagRes *ttsdk.DiagnosticResponse) {
	var err error
	var client *chat.Service

	credentialOptions := option.WithCredentialsFile("credentials.json")
	if len(credentials) > 0 {
		credentialOptions = option.WithCredentialsJSON([]byte(credentials))
	}

	ctx := context.Background()
	client, err = chat.NewService(ctx, credentialOptions, option.WithScopes(chat.ChatBotScope))
	if err != nil {
		log.Printf("sendResults: failed to connect to chat service: %v", err)
		return
	}

	msg := &chat.Message{
		Text: fmt.Sprintf("Results:\n%s", diagRes.GetResults()),
	}

	_, err = client.Spaces.Messages.Create(chatSpace, msg).Do()
	if err != nil {
		log.Printf("sendResults: failed to create a message: %v", err)
		return
	}
	// log.Printf("sendResults: successfully created a message: %s", msg.Text)
}

func getTalkBackReport(diagReq *ttsdk.DiagnosticRequest) (*ttsdk.DiagnosticResponse, error) {

	env, err := echocore.GetEnvByMessageId(diagReq.MessageId)
	if err != nil {
		log.Printf("getTalkBackReport: message targeting un-authorized bus: %s", diagReq.MessageId)
		return nil, err
	}
	if echoBus, ok := echocore.GlobalEchoNetwork.Get(env); ok {
		log.Printf("getTalkBackReport: message targeting authorized bus: %s", env)

		go func(eb *echocore.EchoBus, dr *ttsdk.DiagnosticRequest) {
			(*eb).RequestsChan <- dr
		}(echoBus, diagReq)
		return <-echoBus.ResponseChan, nil
	}
	return nil, errors.New("invalid env: " + env)
}

// Deprecated...
func getReport(diagReq *ttsdk.DiagnosticRequest) (*ttsdk.DiagnosticResponse, error) {
	var opts []grpc.DialOption

	certPool, _ := x509.SystemCertPool()
	if certPool == nil {
		certPool = x509.NewCertPool()
	}
	certPath := "../../certs/cert_files/dcidevpublic.pem"
	cert, err := kv.Asset(certPath)
	if err != nil {
		log.Printf("getReport: cert setup failure: %v", err)
		return nil, err
	}
	certPool.AppendCertsFromPEM(cert)
	var tlsConfig = &tls.Config{RootCAs: certPool}

	tlsOpt := grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))
	opts = append(opts, tlsOpt)
	conn, err := grpc.NewClient(fmt.Sprintf("%s:%v", "*diagnosticsHost", "*diagnosticsPort"), opts...)
	if err != nil {
		log.Printf("getReport: fail to dial: %v", err)
		return nil, err
	}
	defer conn.Close()
	client := ttsdk.NewTrcshTalkServiceClient(conn)

	diagRes, err := client.RunDiagnostics(context.Background(), diagReq)
	if err != nil {
		log.Printf("getReport: bad response: %v", err)
		return &ttsdk.DiagnosticResponse{
			MessageId: diagReq.GetMessageId(),
			Results:   "Unable to obtain report from Hive.",
		}, err
	}
	log.Printf("getReport: success, response returned: %s", diagRes.Results)
	return diagRes, nil
}

func grpcHandler(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")

	if token != echocore.GetTTBToken() {
		http.NotFound(w, r)
		return
	}

	request := &ttsdk.DiagnosticRequest{}
	queryData, err := io.ReadAll(r.Body)
	log.Printf("grpcHandler message received.\n")

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		log.Printf("Error sending response: %s\n", err.Error())
		fmt.Fprintf(w, `{"text": "Failure reading Message."}`)
		return
	}

	err = protojson.Unmarshal(queryData, request)
	if err != nil {
		log.Printf("Error sending response: %s\n", err.Error())
		fmt.Fprintf(w, `{"text": "Failure unmarshalling Message."}`)
		return
	}

	response, pbFunc, err := echocore.RunDiagnostics(r.Context(), request)
	if err != nil {
		log.Printf("Error sending response: %s\n", err.Error())
		fmt.Fprintf(w, `{"text": "Failure running diagnostics on."}`)
		return
	}

	responseBytes, err := protojson.Marshal(response)
	if err != nil {
		log.Printf("Error sending response.. Did not marshal, dropping...: %s\n", err.Error())
		fmt.Fprintf(w, `{"text": "Failure decoding Message."}`)
		return
	}
	if r.Context().Err() != nil {
		// Sheepishly put the response back...
		log.Printf("Failed writeback...  Putting it back... \n")
		if pbFunc != nil {
			pbFunc()
		}
	} else {
		if _, err := w.Write(responseBytes); err != nil {
			// Sheepishly put the response back...
			log.Printf("Failed writeback...  Putting it back... %s\n", err.Error())
			if pbFunc != nil {
				pbFunc()
			}
		}
	}
}

func verifyToken(token string) error {
	ctx := context.Background()
	payload, err := idtoken.Validate(ctx, token, echocore.GetClientID())
	if err != nil {
		log.Printf("Token validation failure: %v", err)
		return fmt.Errorf("token validation failed: %v", err)
	}

	if payload.Issuer != "https://accounts.google.com" && payload.Issuer != "accounts.google.com" {
		log.Printf("Issuer failure: %s\n", payload.Issuer)
		return fmt.Errorf("invalid issuer: %s", payload.Issuer)
	}

	return nil
}

func messageHandler(w http.ResponseWriter, r *http.Request) {
	//	log.Printf("messageHandler: request received: %v", r)
	// decode incoming message from request body
	var receivedChat chat.DeprecatedEvent
	err := json.NewDecoder(r.Body).Decode(&receivedChat)
	// json.NewDecoder(r.Body).Decode(&receivedChat)
	// var err error = nil
	// receivedChat.User.DisplayName = "Joe Tester"
	// receivedChat.Space.Name = echocore.GetChatSpaceByEnv("dev")
	// receivedChat.Message.Text = "run <feature> tenantID:<id>:Test"
	if err != nil {
		log.Printf("messageHandler: failure decoding request body: %v", err)
		fmt.Fprintf(w, `{"text": "Failure decoding Message."}`)
		return
	}
	if receivedChat.Type != "MESSAGE" {
		// Ignore non message events.
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		log.Printf("Missing bearer\n")
		http.NotFound(w, r)
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")

	// Verify the JWT token
	if err := verifyToken(token); err != nil {
		log.Printf("Verification failure\n")
		http.NotFound(w, r)
		return
	}

	//	log.Printf("messageHandler: inbound message decoded: %s", receivedChat.Message.Text)

	// only continue with authorized spaces
	env, authorized := echocore.IsSpaceAuthorized(receivedChat.Space.Name)

	if !authorized {
		log.Printf("messageHandler: space not authorized: %s", receivedChat.Space.Name)
		fmt.Fprintf(w, `{"text": "Failure authorizing Space."}`)
		return
	}

	log.Printf("messageHandler: message originating from authorized space: %s", receivedChat.Space.Name)

	// parse / command
	switch strings.TrimSpace(receivedChat.Message.Text) {
	case "/help":
		log.Print("messageHandler: received help command")
		return
		// future commands add here
	}

	// reply with confirmation and gen report, or reply with clarification statement
	if util.ValidateRequest(receivedChat.Message.Text) {
		// interactive response with query parameters
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, `{"text": "%s, Running diagnostics..."}`,
			strings.Fields(receivedChat.User.DisplayName)[0])

		// parse incoming message and generate request
		diagnosticRequest := &ttsdk.DiagnosticRequest{
			MessageId:   util.GenMsgId(env),
			Diagnostics: util.ParseDiagnostics(receivedChat.Message.Text),
			TenantId:    util.ParseTenantID(receivedChat.Message.Text),
			Data:        util.ParseData(receivedChat.Message.Text),
		}

		go func(diagReq *ttsdk.DiagnosticRequest, chatSpace string) {
			// get the response from Hive
			diagnosticResponse, err := getTalkBackReport(diagReq)
			if err != nil {
				log.Printf("messageHandler: did not obtain a report")
			}
			credentials := ""
			if credentialsInterface, ok := (*configContext.Config)["credentials"]; ok {
				if cred, ok := credentialsInterface.(string); ok {
					credentials = cred
				}
			}
			// construct and send message from Hive
			sendResults(credentials, chatSpace, diagnosticResponse)
		}(diagnosticRequest, receivedChat.Space.Name)
	} else {
		// default response for no keywords specified
		fmt.Fprintf(w, `{"text": "I am a limited bot, %s. Please use the command '/help' for a list of possible queries."}`,
			strings.Fields(receivedChat.User.DisplayName)[0])
	}
}

func robotHandler(w http.ResponseWriter, _ *http.Request) {
	content, err := os.ReadFile("robots.txt")
	if err != nil {
		http.Error(w, "Could not read robots.txt", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write(content)
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, `{"status": "Healthy"}`)
}

type routeHandler struct{}

func (h *routeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/robots.txt":
		robotHandler(w, r)
	case r.URL.Path == "/health",
		r.URL.Path == "/liveness_check",
		r.URL.Path == "/readiness_check":
		healthHandler(w, r)
	case r.URL.Path == "/grpc" && r.Method == "POST":
		grpcHandler(w, r)
	case r.URL.Path == "/" && r.Method == "POST":
		// messages are POST only
		messageHandler(w, r)
	default:
		http.NotFound(w, r)
		//messageHandler(w, r)
	}
}

func StopHttpServer() {
	if httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(ctx); err != nil {
			log.Fatalf("Server shutdown failed: %v", err)
		}
		httpServer = nil
	}
}

func InitHttpServer(configContext *tccore.ConfigContext, send_err func(error)) {

	if portInterface, ok := (*configContext.Config)["http_server_port"]; ok {
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
		}
		httpServerMux := http.NewServeMux()
		//httpServer.IdleTimeout = 30 * time.Second
		httpServerMux.HandleFunc("/", (&routeHandler{}).ServeHTTP)
		var err error

		var tlsListener net.Listener

		if echocore.IsKernelPluginMode() {
			if _, ok := (*configContext.ConfigCerts)[tccore.TRCSHHIVEK_CERT]; !ok {
				configContext.Log.Printf("missing trcshtalk cert\n")
				send_err(errors.New("missing trcshtalk cert"))
				return
			}
			if _, ok := (*configContext.ConfigCerts)[tccore.TRCSHHIVEK_KEY]; !ok {
				configContext.Log.Printf("missing trcshtalk cert\n")
				send_err(errors.New("missing trcshtalk key"))
				return
			}

			var cert tls.Certificate
			cert, err = tls.X509KeyPair((*configContext.ConfigCerts)[tccore.TRCSHHIVEK_CERT], (*configContext.ConfigCerts)[tccore.TRCSHHIVEK_KEY])
			if err != nil {
				configContext.Log.Printf("Couldn't construct key pair: %v", err)
				log.Printf("Couldn't construct key pair: %v", err)
			} else {
				tlsConfig := &tls.Config{
					Certificates: []tls.Certificate{cert},
				}
				log.Printf("main: listening on port %d", echoPort)
				tlsListener, err = tls.Listen("tcp", fmt.Sprintf(":%d", echoPort), tlsConfig)
			}
		} else {
			log.Printf("main: listening on port %d", echoPort)
			var addr *net.TCPAddr
			addr, err = net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", echoPort))
			if err != nil {
				send_err(err)
				return
			}
			tlsListener, err = net.ListenTCP("tcp", addr)
		}

		if err != nil {
			configContext.Log.Printf("Failed to start listener: %v", err)
			send_err(err)
			return
		}

		httpServer = &http.Server{Handler: httpServerMux}
		go func(hs *http.Server, l *net.Listener) {
			err = hs.Serve(*l)

			if err != nil {
				log.Fatal(err)
			}
		}(httpServer, &tlsListener)
	}
}

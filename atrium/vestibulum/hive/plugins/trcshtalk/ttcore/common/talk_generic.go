package common

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var hubClientConnections sync.Map

type hubClientConnectionKey struct {
	serverName string
	port       int
	certHash   [32]byte
}

func getHubClientConnection(serverName string, port int, certBytes []byte, tlsConfig *tls.Config) (*grpc.ClientConn, error) {
	key := hubClientConnectionKey{
		serverName: serverName,
		port:       port,
		certHash:   sha256.Sum256(certBytes),
	}
	if conn, ok := hubClientConnections.Load(key); ok {
		return conn.(*grpc.ClientConn), nil
	}
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", serverName, port),
		grpc.WithDefaultCallOptions(),
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		return nil, err
	}
	actual, loaded := hubClientConnections.LoadOrStore(key, conn)
	if loaded {
		_ = conn.Close()
		return actual.(*grpc.ClientConn), nil
	}
	return conn, nil
}

func CloseHubClientConnections() {
	hubClientConnections.Range(func(key any, value any) bool {
		if conn, ok := value.(*grpc.ClientConn); ok {
			_ = conn.Close()
		}
		hubClientConnections.Delete(key)
		return true
	})
}

// StartTrashTalkingGeneric is a minimal generic version of the loop that previously
// used concrete protobuf types. All pb specifics are pushed into the provided callbacks.
// No extra structs or abstractions – just interface{} and functions.
func StartTrashTalkingGeneric(
	ctx *tccore.ConfigContext,
	shutdown <-chan bool,
	makeEmptyReq func() any,
	makeBroadcastReq func(msg string) any,
	process func(req any, broadcast bool) (any, error),
	extractResults func(resp any) string,
	unmarshalReq func(serialized string) (any, error),
	talkBack func(req any) any,
	makeReply func(origReq any, talkbackResp any) any,
) {
	if ctx == nil {
		return
	}
	ctx.Log.Printf("Trash talking...\n")
	const maxRetries = 5

	// Broadcast loop
	go func() {
		if ctx.ChatBroadcastChan == nil {
			return
		}
		for msg := range *ctx.ChatBroadcastChan {
			if msg.Response == nil {
				continue
			}
			req := makeBroadcastReq(*msg.Response)
			retry := 0
		retryBroadcast:
			if _, err := process(req, true); err != nil {
				if retry < maxRetries {
					retry++
					time.Sleep(3 * time.Second)
					goto retryBroadcast
				}
				return
			}
		}
	}()

	for {
		select {
		case <-shutdown:
			return
		default:
		}
		emptyReq := makeEmptyReq()
		resp, err := process(emptyReq, false)
		if err != nil {
			continue
		}
		serialized := extractResults(resp)
		if serialized == "" {
			continue
		}
		inbound, err := unmarshalReq(serialized)
		if err != nil {
			continue
		}
		go func(r any) {
			tb := talkBack(r)
			reply := makeReply(r, tb)
			retry := 0
		retryReply:
			if _, err := process(reply, false); err != nil {
				if retry < maxRetries {
					retry++
					time.Sleep(3 * time.Second)
					goto retryReply
				}
			}
		}(inbound)
	}
}

// CollectQueryResponses is a small generic helper that waits for kernel responses
// for each query name previously sent over ChatSenderChan. It concatenates the
// responses in arbitrary completion order (matching the prior inline logic) and
// returns the routing/message id observed in the final event plus the combined
// result string.
//
// It is intentionally pb-agnostic: callers supply only the config context and
// the list of expected query keys. If queries is empty it returns immediately.
// CollectQueryResponses now also performs the initial send of the query list.
// routingId/chatId are passed in so callers don't duplicate the ChatMsg creation line.
// pluginName kept separate so different callers can reuse even if they vary the name.
func CollectQueryResponses(ctx *tccore.ConfigContext, routingId *string, pluginName string, chatId *string, queries []string) (messageId string, results string) {
	if ctx == nil {
		return "", ""
	}
	if len(queries) == 0 {
		return "", ""
	}
	// Send the chat message here (previously in TrcshTalkBack).
	*ctx.ChatSenderChan <- &tccore.ChatMsg{RoutingId: routingId, Name: &pluginName, ChatId: chatId, Query: &queries}

	finished := make(map[string]string)
	ctx.Log.Printf("Sent queries to kernel: %d\n", len(queries))
	for {
		event := <-*ctx.ChatReceiverChan
		ctx.Log.Println("TrcshTalk received message from kernel.")
		if event.RoutingId != nil {
			messageId = *event.RoutingId
		}
		if event.Query != nil && len(*event.Query) == 1 && event.Response != nil && (*event).Response != nil {
			ctx.Log.Printf("Processing response from query: %s\n", *event.Query)
			finished[(*event.Query)[0]] = *event.Response
		}
		if len(finished) == len(queries) {
			ctx.Log.Println("Formatting responses.")
			for _, v := range finished {
				results += v + " "
			}
			ctx.Log.Printf("Sending response to chat: %s\n", results)
			return messageId, results
		}
	}
}

// ProcessTrcshTalkRequestGeneric contains the transport logic (remote HTTP JSON over /grpc
// endpoint with retry, or local direct gRPC call with TLS) formerly embedded in ttcore.
// It is kept here so alternate cores can reuse it. The function still uses the concrete
// protobuf InteractionRequest type because the wire contract is stable; only the response
// construction is injected via newResp.
func ProcessTrcshTalkRequestGeneric(
	ctx *tccore.ConfigContext,
	serverName string,
	port int,
	ttbToken *string,
	isRemote bool,
	diagReq proto.Message,
	setMsgID func(proto.Message, string),
	newResp func() proto.Message,
	extractResult func(proto.Message) string,
	shutdown <-chan bool,
	broadcast bool,
	genMsgID func(env, region string, broadcast bool) string,
	mashupCertBytes []byte,
	tlsConfigProvider func(serverName string, certBytes []byte) (*tls.Config, error),
	newClient func(*grpc.ClientConn) any,
	invokeInteraction func(client any, ctx context.Context, req proto.Message) (proto.Message, error),
) (proto.Message, error) {
	if ctx == nil {
		return nil, errors.New("nil config context")
	}
	// Assign new message id each call via callback.
	setMsgID(diagReq, genMsgID(ctx.Env, ctx.Region, broadcast))

	if isRemote {
		retryCount := 0
		requestBytes, err := protojson.Marshal(diagReq)
		if err != nil {
			ctx.Log.Printf("Error marshalling request: %s. Undeliverable request/response.\n", err.Error())
			return nil, err
		}
		req, err := http.NewRequest("POST", fmt.Sprintf("https://%s/grpc", serverName), bytes.NewBuffer(requestBytes))
		if err != nil {
			ctx.Log.Printf("Post failure. Unrecoverable.\n")
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		if ttbToken != nil {
			req.Header.Set("Authorization", *ttbToken)
		}
		if supportedDeployments := SupportedDeploymentsCSV(ctx); supportedDeployments != "" {
			req.Header.Set("X-Trc-Supported-Deployments", supportedDeployments)
		}
	retrypostgrpc:
		client := &http.Client{Timeout: time.Minute}
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if retryCount < 5 {
				time.Sleep(3 * time.Second)
				retryCount++
				select {
				case <-shutdown:
					return nil, errors.New("shutdown")
				default:
				}
				goto retrypostgrpc
			}
			return nil, err
		}
		defer resp.Body.Close()
		diagRes := newResp()
		respData, err := io.ReadAll(resp.Body)
		if err != nil {
			ctx.Log.Printf("Error reading post grpc response: %s\n", err.Error())
			if retryCount < 5 {
				time.Sleep(3 * time.Second)
				retryCount++
				select {
				case <-shutdown:
					return nil, errors.New("shutdown")
				default:
				}
				goto retrypostgrpc
			}
			return nil, err
		}
		if err = protojson.Unmarshal(respData, diagRes); err != nil {
			errMsg := err.Error()
			if strings.Contains(errMsg, "unknown field \"text\"") {
				ctx.Log.Printf("Kzzz\n")
			} else {
				ctx.Log.Printf("Error unmarshalling post grpc response: %s\n", err.Error())
			}
			if retryCount < 5 {
				time.Sleep(3 * time.Second)
				retryCount++
				select {
				case <-shutdown:
					return nil, errors.New("shutdown")
				default:
				}
				goto retrypostgrpc
			}
			return nil, err
		}
		if res := extractResult(diagRes); res != "" {
			ctx.Log.Printf("ProcessTrcshTalkRequestGeneric: success, response returned: %s\n", res)
		} else {
			ctx.Log.Printf("ProcessTrcshTalkRequestGeneric: success (generic response)\n")
		}
		return diagRes, nil
	}

	// Local (hardwired) path via direct gRPC call (TLS config may be externally provided).
	if mashupCertBytes == nil && tlsConfigProvider == nil {
		ctx.Log.Printf("Cert not initialized.\n")
		return nil, errors.New("cert initialization failure")
	}
	var tlsCfg *tls.Config
	var err error
	if tlsConfigProvider != nil {
		tlsCfg, err = tlsConfigProvider(serverName, mashupCertBytes)
		if err != nil {
			ctx.Log.Printf("tlsConfigProvider error: %v\n", err)
			return nil, err
		}
	} else {
		mashupBlock, _ := pem.Decode([]byte(mashupCertBytes))
		mashupClientCert, parseErr := x509.ParseCertificate(mashupBlock.Bytes)
		if parseErr != nil {
			ctx.Log.Printf("failed to parse cert: %v\n", parseErr)
			return nil, parseErr
		}
		mashupCertPool := x509.NewCertPool()
		mashupCertPool.AddCert(mashupClientCert)
		tlsCfg = &tls.Config{ServerName: serverName, RootCAs: mashupCertPool, MinVersion: tls.VersionTLS12}
	}
	isHubClient := ctx.Config != nil && resolveTrcshTalkMode(ctx.Config) == ModeHubClient
	var conn *grpc.ClientConn
	if isHubClient {
		conn, err = getHubClientConnection(serverName, port, mashupCertBytes, tlsCfg)
	} else {
		conn, err = grpc.Dial(fmt.Sprintf("%s:%d", serverName, port),
			grpc.WithDefaultCallOptions(),
			grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	}
	if err != nil {
		ctx.Log.Printf("ProcessTrcshTalkRequestGeneric: fail to dial: %v\n", err)
		return nil, err
	}
	if !isHubClient {
		defer conn.Close()
	}
	client := newClient(conn)
	callCtx := context.Background()
	if ttbToken != nil {
		callCtx = WithTTBTokenOutgoingContext(callCtx, *ttbToken)
	}
	if supportedDeployments := SupportedDeploymentsCSV(ctx); supportedDeployments != "" {
		callCtx = WithSupportedDeploymentsOutgoingContext(callCtx, supportedDeployments)
	}
	callCtx = WithHubClientPodOutgoingContext(callCtx, ctx)
	callCtx = WithHubClientStatefulSetOutgoingContext(callCtx, ctx)
	interactionRes, err := invokeInteraction(client, callCtx, diagReq)
	if err != nil {
		ctx.Log.Printf("ProcessTrcshTalkRequestGeneric: bad response: %v\n", err)
		return nil, err
	}
	if res := extractResult(interactionRes); res != "" {
		ctx.Log.Printf("ProcessTrcshTalkRequestGeneric: success, response returned: %s\n", res)
	} else {
		ctx.Log.Printf("ProcessTrcshTalkRequestGeneric: success (generic response)\n")
	}
	return proto.Message(interactionRes), nil
}

// CollectQueryResponsesDefault uses the standard plugin name "trcshtalk" so callers
// don't need to repeat the local variable assignment.

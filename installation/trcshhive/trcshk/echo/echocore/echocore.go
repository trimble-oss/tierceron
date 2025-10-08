package echocore

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	cmap "github.com/orcaman/concurrent-map/v2"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	ttsdk "github.com/trimble-oss/tierceron/installation/trcshhive/trcshk/echo/trcshtalksdk"
	"google.golang.org/protobuf/encoding/protojson"
)

type EchoBus struct {
	Env          string
	ChatSpace    string
	RequestsChan chan *ttsdk.DiagnosticRequest
	ResponseChan chan *ttsdk.DiagnosticResponse
}

// Buses will be indexed by environment
type EchoNetwork map[string]*EchoBus

// Complex map indexing EchoBus by both env and chatspace.
var GlobalEchoNetwork *cmap.ConcurrentMap[string, *EchoBus]

var GlobalSupportedEnvMap map[string]bool

var serverMode string

var clientID string

var ttbToken string

func GetClientID() string {
	return clientID
}

func GetTTBToken() string {
	return ttbToken
}

func InitNetwork(configContext *tccore.ConfigContext) {
	echoNetwork := cmap.New[*EchoBus]()
	GlobalEchoNetwork = &echoNetwork

	envs := []string{"dev"}
	GlobalSupportedEnvMap = make(map[string]bool, 4)
	for _, env := range envs {
		GlobalSupportedEnvMap[env] = true
		GlobalEchoNetwork.Set(env, &EchoBus{
			Env:          env,
			RequestsChan: make(chan *ttsdk.DiagnosticRequest, 10),
			ResponseChan: make(chan *ttsdk.DiagnosticResponse, 10),
		})
	}

	if spacesInterface, ok := (*configContext.Config)["chat_spaces"]; ok {
		if spaces, ok := spacesInterface.(string); ok {
			authSpaceEntries := strings.Split(spaces, ",")
			for _, authSpaceEntry := range authSpaceEntries {
				env_Space := strings.Split(authSpaceEntry, ":")

				if echoBus, ok := GlobalEchoNetwork.Get(env_Space[0]); ok {
					echoBus.ChatSpace = env_Space[1]
					GlobalEchoNetwork.Set(echoBus.ChatSpace, echoBus)
					configContext.Log.Printf("Bus initialized for env: %s\n", env_Space[0])
				}
			}
		}
	}
	serverMode = "trcshtalkback"

	if serverModeInterface, ok := (*configContext.Config)["server_mode"]; ok {
		if mode, ok := serverModeInterface.(string); ok {
			configContext.Log.Println("Server_mode initialized.")
			serverMode = mode
		}
	}

	if clientIdInterface, ok := (*configContext.Config)["client_id"]; ok {
		if cid, ok := clientIdInterface.(string); ok {
			configContext.Log.Println("Client_id initialized.")
			clientID = fmt.Sprintf("https://%s.uw.r.appspot.com", cid)
		}
	}

	if ttbtokenInterface, ok := (*configContext.Config)["ttb_token"]; ok {
		if ttb, ok := ttbtokenInterface.(string); ok {
			configContext.Log.Println("ttb_token initialized.")
			ttbToken = ttb
		}
	}

}

func IsKernelPluginMode() bool {
	return serverMode == "talkback-kernel-plugin"
}

// Return true if space is authorized
func IsSpaceAuthorized(aSpace string) (string, bool) {
	if echoBus, ok := GlobalEchoNetwork.Get(aSpace); ok {
		return echoBus.Env, ok
	}
	return "", false
}

func GetEnvByChatspace(aSpace string) (string, bool) {
	if echoBus, ok := GlobalEchoNetwork.Get(aSpace); ok {
		return echoBus.Env, ok
	}
	return "", false
}

// Function used in test.
func GetChatSpaceByEnv(env string) (string, bool) {
	if echoBus, ok := GlobalEchoNetwork.Get(env); ok {
		return echoBus.ChatSpace, ok
	}
	return "", false
}

// Utility function to extract environment of message
func GetEnvByMessageId(messageId string) (string, error) {
	if len(messageId) == 0 {
		return "", errors.New("malformatted messageid")
	}

	messageParts := strings.Split(messageId, ":")
	if GlobalSupportedEnvMap[messageParts[0]] {
		return messageParts[0], nil
	} else {
		return "", errors.New("malformatted messageid")
	}
}

func RunDiagnostics(ctx context.Context, req *ttsdk.DiagnosticRequest) (*ttsdk.DiagnosticResponse, func(), error) {

	env, err := GetEnvByMessageId(req.MessageId)
	if err != nil {
		log.Printf("RunDiagnostics: message targeting un-authorized bus: %s", req.MessageId)
		return nil, nil, err
	}
	if echoBus, ok := GlobalEchoNetwork.Get(env); ok {
		log.Printf("RunDiagnostics: message targeting authorized bus: %s", env)

		if len(req.Data) > 0 {
			log.Printf("RunDiagnostics:posting response message to authorized bus: %s", env)
			// trcshtalk is posting a response to the response channel.
			go func(eb *EchoBus, r *ttsdk.DiagnosticRequest) {
				// Post to response channel the result.
				(*eb).ResponseChan <- &ttsdk.DiagnosticResponse{
					MessageId: r.MessageId,
					Results:   r.Data[0],
				}
			}(echoBus, req)
			return &ttsdk.DiagnosticResponse{
				MessageId: req.MessageId,
				Results:   "Response posted",
			}, nil, nil
		} else {
			log.Printf("RunDiagnostics:requesting message from authorized bus: %s", env)

			// trcshtalk is asking for a new request to process from the request channel.

			var request *ttsdk.DiagnosticRequest
			select {
			case request = <-(*echoBus).RequestsChan:
				break
			case <-ctx.Done(): // This checks if the client has disconnected
				log.Printf("RunDiagnostics:client disconnected: %s", env)
				return nil, nil, err
			}

			log.Printf("RunDiagnostics:message extracted from authorized bus: %s", env)
			requestBytes, err := protojson.Marshal(request)
			if err != nil {
				log.Printf("RunDiagnostics:error unmarshalling request from bus: %s", env)
				return nil, nil, err
			} else {
				log.Printf("RunDiagnostics:returning request from authorized bus: %s", env)
				res := &ttsdk.DiagnosticResponse{
					MessageId: req.MessageId,
					Results:   string(requestBytes),
				}
				return res, func() {
					// The Putback func.
					(*echoBus).RequestsChan <- request
				}, nil
			}
		}
	}
	return nil, nil, errors.New("invalid env: " + env)
}

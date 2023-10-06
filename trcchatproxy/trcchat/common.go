package trcchat

import (
	"bufio"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/trimble-oss/tierceron-nute/mashupsdk"
	"github.com/trimble-oss/tierceron-nute/mashupsdk/server"
	"github.com/trimble-oss/tierceron/trcchatproxy/pubsub"
)

var gchatApp GChatApp
var id int64

//go:embed tls/mashup.crt
var mashupCert embed.FS

//go:embed tls/mashup.key
var mashupKey embed.FS

func (w *GChatApp) InitServer(callerCreds string, insecure bool, maxMessageLength int) {
	if callerCreds != "" {
		server.InitServer(callerCreds, insecure, maxMessageLength, w.MashupSdkApiHandler, w.WClientInitHandler)
	}
}

func CommonInit() {
	gchatApp = GChatApp{
		MashupSdkApiHandler: &GoogleChatHandler{},
		GoogleChatContext:   &GoogleChatContext{},
		WClientInitHandler:  &WorldClientInitHandler{},
	}

	// Initialize local server.
	mashupsdk.InitCertKeyPair(mashupCert, mashupKey)

	gchatworld := GChatApp{
		MashupSdkApiHandler: &GoogleChatHandler{},
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	configPort, err := strconv.ParseInt(port, 10, 64)
	if err != nil {
		fmt.Println(err)
	}

	configs := mashupsdk.MashupConnectionConfigs{
		AuthToken:   "zxc90-2389-v89o102389v-z89a",
		CallerToken: "1283-97z8-xbvy0a2389gsa7",
		Server:      "",
		Port:        configPort,
	}
	encoding, err := json.Marshal(&configs)
	if err != nil {
		fmt.Println(err)
	}

	callerCreds := flag.String("CREDS", string(encoding), "Credentials of caller")
	flag.Parse()
	id = 0
	server.RemoteInitServer(*callerCreds, true, -2, gchatworld.MashupSdkApiHandler, gchatworld.WClientInitHandler)

}

// Processes upserted query from client
// Changes based on msg.Name
func ProcessQuery(msg *mashupsdk.MashupDetailedElement) {
	switch msg.Name {
	case "DialogFlow":
		ProcessDFQuery(msg)
	case "DialogFlowResponse":
		ProcessDFResponse(msg)
	case "GChatResponse":
		ProcessGChatAnswer(msg)
	case "Get Message":
		gchatApp.DetailedElements = gchatApp.DetailedElements[:len(gchatApp.DetailedElements)-1]
		input := ""
		alias := ""
		for input == "" {
			alias, input = getUserInput()
			if input != "" {
				gchatApp.DetailedElements = append(gchatApp.DetailedElements, &mashupsdk.MashupDetailedElement{
					Name:  "GChatQuery",
					Alias: alias,
					Id:    int64(len(gchatApp.DetailedElements)), // Make sure id matches index in elements
					Data:  input,
				})
			} else {
				fmt.Println("An error occurred with reading the input. Please input your question in the command line and press enter!")
			}
		}
	default:
		log.Printf("Message type does not correspond to either GChatQuery or DialogFlow")
	}
}

// Asks user input
// This is a stub version --> potentially shouldn't be needed if user can @askflume in google chat
// However, maybe use this as a way to ask user if there is anything else they would like to ask
func getUserInput() (string, string) {
	var alias string
	var input string
	var err error
	if pubsub.IsManualInteractionEnabled() {
		fmt.Println("This is a simulation of the Flume Chat App. Please type your question below and press enter: ")
		reader := bufio.NewReader(os.Stdin)
		input, err = reader.ReadString('\n')
		alias = fmt.Sprintf("%x", sha256.Sum256([]byte(input))) // Hacky alias...

		if err != nil {
			log.Printf("Error reading input from user: %v", err)
			return "", ""
		}
	} else {
		event := pubsub.SubChatEvent()
		alias = event.Message.ClientAssignedMessageId
		input = event.Message.Text
	}
	return alias, input
}

// Updates ID and returns value
// id should match up with number of queries made by user
func GetId() int64 {
	id += 1
	return id
}

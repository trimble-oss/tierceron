package askflume

import (
	"embed"
	"fmt"
	"log"
	"sync"

	"VaultConfig.TenantConfig/util"
	"github.com/trimble-oss/tierceron-nute/mashupsdk"
	"github.com/trimble-oss/tierceron-nute/mashupsdk/client"

	flowcore "github.com/trimble-oss/tierceron/trcflow/core"
)

var askFlumeContext *flowcore.AskFlumeContext
var tfmcontext *flowcore.TrcFlowMachineContext
var tfContext *flowcore.TrcFlowContext

var msg string

//go:embed tls/mashup.crt
var mashupCert embed.FS

//go:embed tls/mashup.key
var mashupKey embed.FS

// Initializes the Flume side of AskFlume
// Called by ProcessFlowController in buildopts/flowopts/flow_tc.go
func ProcessAskFlumeController(tfmContext *flowcore.TrcFlowMachineContext, trcFlowContext *flowcore.TrcFlowContext) error {
	var err error
	askFlumeContext, err = flowcore.InitAskFlume()
	tfmcontext = tfmContext
	tfContext = trcFlowContext

	if err != nil {
		fmt.Printf("Error initializing AskFlume %v", err)
		return err
	}

	if err != nil {
		fmt.Printf("Error initializing Google Chat %v", err)
		return err
	}
	var askFlumeWg sync.WaitGroup
	askFlumeWg.Add(1)

	go askFlumeFlowReceiver(askFlumeContext, &askFlumeWg)
	go askFlumeFlowSender(askFlumeContext)
	go InitGoogleChat()

	askFlumeWg.Wait()
	return err
}

// Listens for any messages along each channel
// Can be stopped by making askFlumeContext.Close = true
func askFlumeFlowReceiver(askFlumeContext *flowcore.AskFlumeContext, askFlumeWg *sync.WaitGroup) {
	for {
		select {
		case gchat_query := <-askFlumeContext.GchatQueries:
			err := handleGChatQuery(gchat_query)
			if err != nil {
				return
			}
		case dialogflow_query := <-askFlumeContext.DFQueries:
			err := handleDFQuery(dialogflow_query)
			if err != nil {
				return
			}
		case dialogflow_answer := <-askFlumeContext.DFAnswers:
			err := handleDFAnswer(dialogflow_answer)
			if err != nil {
				return
			}
		case gchatanswer := <-askFlumeContext.GchatAnswers:
			err := handleGchatAnswer(gchatanswer)
			if err != nil {
				return
			}
		}
		if askFlumeContext.Close {
			askFlumeWg.Done()
			return
		}
	}
}

// Based on the askFlumeContext.FlowCase, will send the context on a specified channel
// Will terminate if askFlumeContext.Close = true
func askFlumeFlowSender(askFlumeContext *flowcore.AskFlumeContext) error {
	for {
		switch {
		case askFlumeContext.FlowCase == "GChatQuery":
			go func(askFlumeContext *flowcore.AskFlumeContext) {
				askFlumeContext.GchatQueries <- askFlumeContext
			}(askFlumeContext)
			askFlumeContext.FlowCase = ""
		case askFlumeContext.FlowCase == "GChatAnswer":
			go func(askFlumeContext *flowcore.AskFlumeContext) {
				askFlumeContext.GchatAnswers <- askFlumeContext
			}(askFlumeContext)
			askFlumeContext.FlowCase = ""
		case askFlumeContext.FlowCase == "DFQuery":
			go func(askFlumeContext *flowcore.AskFlumeContext) {
				askFlumeContext.DFQueries <- askFlumeContext
			}(askFlumeContext)
			askFlumeContext.FlowCase = ""
		case askFlumeContext.FlowCase == "DFAnswer":
			go func(askFlumeContext *flowcore.AskFlumeContext) {
				askFlumeContext.DFAnswers <- askFlumeContext
			}(askFlumeContext)
			askFlumeContext.FlowCase = ""
		default:
			if askFlumeContext.Close {
				return nil
			}
		}
	}
}

// Processes the given message and updates the context based on
// the message.Data, message.Alias, and message.Name
func GetQuery(message *mashupsdk.MashupDetailedElement) {
	if message != nil && message.Name != "" {
		msg = message.Data
		msg_type := message.Name

		if msg_type == "Get Message" {
			askFlumeContext.Upsert <- &mashupsdk.MashupDetailedElementBundle{
				DetailedElements: Flumeworld.DetailedElements,
			}
		} else if message.Data != "" && msg_type != "DFQuery" && msg_type != "DFAnswer" && msg_type != "GChatQuery" && msg_type != "GChatAnswer" {
			log.Printf("Message does not correspond to any channel")
		} else {
			askFlumeContext.Query.Id = message.Id
			askFlumeContext.Query.Message = msg
			if message.Alias != "" {
				askFlumeContext.Query.Type = message.Alias
			}
			askFlumeContext.FlowCase = msg_type
		}
	} else {
		log.Printf("Message is not properly initialized with data or name fields")
	}
}

// Updates the current element/query being processed to be directed to
// DialogFlow
// Records user's query in the askFlumeContext.Queries array
func handleGChatQuery(askFlumeContext *flowcore.AskFlumeContext) error {
	if askFlumeContext.Query.Message != "" {
		askFlumeContext.Queries = append(askFlumeContext.Queries, askFlumeContext.Query)
		fmt.Println("Received query from google chat channel in Flume: ", askFlumeContext.Query.Message, " with ID: ", askFlumeContext.Query.Id)
		Flumeworld.DetailedElements[askFlumeContext.Query.Id].Name = "DialogFlow"
		Flumeworld.DetailedElements[askFlumeContext.Query.Id].Data = askFlumeContext.Query.Message
		askFlumeContext.Upsert <- &mashupsdk.MashupDetailedElementBundle{
			DetailedElements: Flumeworld.DetailedElements,
		}
	}

	return nil
}

// Processes mapped result of sending a question to DialogFlow
// Calls ProcessAskFlumeEventMapper in VaultConfig.TenantConfig to query trcdb
// Changes setting to "DialogFlowResponse" so the returned result can be processed
// to make it more understandable to the user
func handleDFQuery(askFlumeContext *flowcore.AskFlumeContext) error {
	if askFlumeContext.Query.Message != "" {
		askFlumeContext.Queries = append(askFlumeContext.Queries, askFlumeContext.Query)
		log.Println("Received query from DialogFlowQueries channel in Flume: ", askFlumeContext.Query.Message, " with ID: ", askFlumeContext.Query.Id)
		log.Println("Processing query and accessing database...")

		response := util.ProcessAskFlumeEventMapper(askFlumeContext, askFlumeContext.Query, tfmcontext, tfContext)
		Flumeworld.DetailedElements[askFlumeContext.Query.Id].Alias = response.Type // Determines which category message belongs
		Flumeworld.DetailedElements[askFlumeContext.Query.Id].Name = "DialogFlowResponse"
		Flumeworld.DetailedElements[askFlumeContext.Query.Id].Data = response.Message
		askFlumeContext.Upsert <- &mashupsdk.MashupDetailedElementBundle{
			DetailedElements: Flumeworld.DetailedElements,
		}
	}
	return nil
}

// Allows for response that will be sent to the user to be checked or saved for future analysis
// Switches element settings to be a Google Chat Response
func handleDFAnswer(askFlumeContext *flowcore.AskFlumeContext) error {
	if askFlumeContext.Query.Message != "" {
		log.Println("Received response from DialogFlow channel: ", askFlumeContext.Query.Message, " with ID: ", askFlumeContext.Query.Id)
		Flumeworld.DetailedElements[askFlumeContext.Query.Id].Name = "GChatResponse"
		fmt.Println("Sending formatted response back to Google Chat to send to the user")
		askFlumeContext.Upsert <- &mashupsdk.MashupDetailedElementBundle{
			DetailedElements: Flumeworld.DetailedElements,
		}
	}

	return nil
}

// Response has been sent back to user, new "Get Message" call is set
// so user can ask more questions
func handleGchatAnswer(askFlumeContext *flowcore.AskFlumeContext) error {
	fmt.Println("Sending response back to user and ending flow")
	offset := flowcore.GetId()
	if Flumeworld.MashupDetailedElementLibrary == nil {
		Flumeworld.MashupDetailedElementLibrary = make(map[int64]*mashupsdk.MashupDetailedElement)
	}
	Flumeworld.MashupDetailedElementLibrary[askFlumeContext.Query.Id+offset] = Flumeworld.DetailedElements[0]
	Flumeworld.DetailedElements = []*mashupsdk.MashupDetailedElement{}
	element := mashupsdk.MashupDetailedElement{
		Name: "Get Message",
	}
	Flumeworld.DetailedElements = []*mashupsdk.MashupDetailedElement{&element}
	askFlumeContext.Upsert <- &mashupsdk.MashupDetailedElementBundle{
		DetailedElements: Flumeworld.DetailedElements,
	}
	return nil
}

// Initializes client of server running in cloud
// Sets up polling loop for upserting elements to cloud server
// Processes returned and updated elements
func InitGoogleChat() error {
	// Create client of known server running on cloud
	var params []string
	params = append(params, "flume")
	params = append(params, "")

	var envParams []string
	envParams = append(envParams, "localhost") // "Remote" server name
	envParams = append(envParams, "8080")      // "Remote" server port
	envParams = append(envParams, "localhost") //  client server name
	envParams = append(envParams, "0")         // client server port

	insecure := true
	mashupsdk.InitCertKeyPair(mashupCert, mashupKey)
	Flumeworld.FlumeWorldContext = client.BootstrapInit("trcchatproxy", Flumeworld.MashupSdkApiHandler, envParams, params, &insecure)

	// Set up process to receive messages from cloud server
	element := mashupsdk.MashupDetailedElement{
		Name: "Get Message",
	}
	Flumeworld.DetailedElements = []*mashupsdk.MashupDetailedElement{&element}

	for {
		updated_elements, upsertErr := Flumeworld.FlumeWorldContext.Client.UpsertElements(Flumeworld.FlumeWorldContext, &mashupsdk.MashupDetailedElementBundle{
			AuthToken:        "",
			DetailedElements: Flumeworld.DetailedElements,
		})
		if upsertErr != nil {
			log.Printf("Failed to upsert google chat query to client: %v", upsertErr)
			break //Don't know if I should break loop if error or just keep trying again
		}

		for _, updatedElement := range updated_elements.DetailedElements {
			if updatedElement.Name == "Done" { // Terminates client
				askFlumeContext.Close = true
				return nil
			}
			GetQuery(updatedElement)
		}

		<-askFlumeContext.Upsert
	}
	return nil
}

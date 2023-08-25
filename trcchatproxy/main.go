package main

import (
	"embed"
	"log"

	"github.com/trimble-oss/tierceron-nute/mashupsdk"
	"github.com/trimble-oss/tierceron-nute/mashupsdk/client"
	"github.com/trimble-oss/tierceron-nute/mashupsdk/server"
	// trccontext "github.com/trimble-oss/tierceron/trcchatproxy/context"
)

var insecure *bool

//go:embed tls/mashup.crt
var mashupCert embed.FS

//go:embed tls/mashup.key
var mashupKey embed.FS

var gchatApp GChatApp

type WorldClientInitHandler struct {
}

type GoogleChatHandler struct {
}

type GoogleChatContext struct {
	MashupContext *mashupsdk.MashupContext // Needed for callbacks to other mashups
}

type GChatApp struct {
	MashupSdkApiHandler *GoogleChatHandler
	GoogleChatContext   *GoogleChatContext //*FlumeWorldContext
	// mashupDisplayContext         *mashupsdk.MashupDisplayContext
	WClientInitHandler           *WorldClientInitHandler
	DetailedElements             []*mashupsdk.MashupDetailedElement
	MashupDetailedElementLibrary map[int64]*mashupsdk.MashupDetailedElement
	ElementLoaderIndex           map[string]int64 // mashup indexes by Name
}

func (w *GChatApp) InitServer(callerCreds string, insecure bool, maxMessageLength int) {
	if callerCreds != "" {
		server.InitServer(callerCreds, insecure, maxMessageLength, w.MashupSdkApiHandler, w.WClientInitHandler)
	}
}

func main() {
	secure := true
	insecure = &secure
	gchatApp = GChatApp{
		MashupSdkApiHandler: &GoogleChatHandler{},
		GoogleChatContext:   &GoogleChatContext{},
		WClientInitHandler:  &WorldClientInitHandler{},
	}
	shutdown := make(chan bool)

	// Initialize local server.
	mashupsdk.InitCertKeyPair(mashupCert, mashupKey)
	params := []string{}

	params = append(params, "remote")
	params = append(params, "")
	env_params := []string{}
	env_params = append(env_params, "localhost")
	env_params = append(env_params, "8080")
	env_params = append(env_params, "localhost")
	gchatApp.GoogleChatContext.MashupContext = client.BootstrapInit("trcchatproxy", gchatApp.MashupSdkApiHandler, env_params, params, insecure)
	InitGoogleChatStub()
	log.Printf("Mashup elements delivered.\n")
	<-shutdown
}

func (msdk *GoogleChatHandler) OnDisplayChange(displayHint *mashupsdk.MashupDisplayHint) {
	return
}

func (msdk *GoogleChatHandler) GetElements() (*mashupsdk.MashupDetailedElementBundle, error) {
	return &mashupsdk.MashupDetailedElementBundle{
		AuthToken:        client.GetServerAuthToken(),
		DetailedElements: gchatApp.DetailedElements,
	}, nil
}

func (msdk *GoogleChatHandler) UpsertElements(detailedElementBundle *mashupsdk.MashupDetailedElementBundle) (*mashupsdk.MashupDetailedElementBundle, error) {
	log.Printf("Google Chat world received upsert elements: %v", detailedElementBundle)
	ProcessQuery(detailedElementBundle)
	return &mashupsdk.MashupDetailedElementBundle{
		AuthToken:        client.GetServerAuthToken(),
		DetailedElements: gchatApp.DetailedElements,
	}, nil
}

func (msdk *GoogleChatHandler) TweakStates(elementStateBundle *mashupsdk.MashupElementStateBundle) (*mashupsdk.MashupElementStateBundle, error) {
	return elementStateBundle, nil
}

func (msdk *GoogleChatHandler) ResetStates() {
	return
}

func (msdk *GoogleChatHandler) TweakStatesByMotiv(mashupsdk.Motiv) {
	return
}

func (msdk *GoogleChatHandler) GetMashupElements() (*mashupsdk.MashupDetailedElementBundle, error) {
	return &mashupsdk.MashupDetailedElementBundle{
		AuthToken:        client.GetServerAuthToken(),
		DetailedElements: gchatApp.DetailedElements,
	}, nil
}

func (msdk *GoogleChatHandler) OnResize(displayHint *mashupsdk.MashupDisplayHint) {
	return
}

func (msdk *GoogleChatHandler) UpsertMashupElements(detailedElementBundle *mashupsdk.MashupDetailedElementBundle) (*mashupsdk.MashupDetailedElementBundle, error) {
	log.Printf("Google Chat world received upsert elements: %v", detailedElementBundle)
	return &mashupsdk.MashupDetailedElementBundle{
		AuthToken:        client.GetServerAuthToken(),
		DetailedElements: gchatApp.DetailedElements,
	}, nil
}

func (msdk *GoogleChatHandler) UpsertMashupElementsState(elementStateBundle *mashupsdk.MashupElementStateBundle) (*mashupsdk.MashupElementStateBundle, error) {
	return elementStateBundle, nil
}

func (msdk *GoogleChatHandler) ResetG3NDetailedElementStates() {
	return
}

func (w *WorldClientInitHandler) RegisterContext(context *mashupsdk.MashupContext) {
	gchatApp.GoogleChatContext.MashupContext = context
}

func ProcessQuery(message *mashupsdk.MashupDetailedElementBundle) {
	msg := message.DetailedElements[0]
	if msg.Name == "GChatQuery" {
		ProcessGChatQuery(msg)
	} else if msg.Name == "DialogFlow" {
		ProcessDFQuery(msg)
	} else if msg.Name == "DialogFlowResponse" {
		ProcessDFResponse(msg)
	} else if msg.Name == "GChatResponse" {
		ProcessGChatAnswer(msg)
	} else {
		log.Printf("Message type does not correspond to either GChatQuery or DialogFlow")
	}
}

func InitGoogleChatStub() {
	var upsertErr error

	element := mashupsdk.MashupDetailedElement{
		Name: "GChatQuery",
		Data: "Hello Flume!",
	}
	detailedElements := &mashupsdk.MashupDetailedElementBundle{}
	log.Printf("Delivering mashup elements.\n")
	detailedElements, upsertErr = gchatApp.GoogleChatContext.MashupContext.Client.UpsertElements(gchatApp.GoogleChatContext.MashupContext, &mashupsdk.MashupDetailedElementBundle{
		AuthToken:        " ",
		DetailedElements: []*mashupsdk.MashupDetailedElement{&element},
	})

	if upsertErr != nil {
		log.Printf("Element state initialization failure: %s\n", upsertErr.Error())
	}
	gchatApp.DetailedElements = detailedElements.DetailedElements
}

func ProcessGChatQuery(msg *mashupsdk.MashupDetailedElement) {
	//Send to google chat
	log.Printf("Message received from Google Chat user")
	element := mashupsdk.MashupDetailedElement{
		Name: "GChatQuery",
		Data: "Hello Flume!",
	}
	detailedElements := &mashupsdk.MashupDetailedElementBundle{}
	log.Printf("Delivering mashup elements.\n")
	detailedElements, upsertErr := gchatApp.GoogleChatContext.MashupContext.Client.UpsertElements(gchatApp.GoogleChatContext.MashupContext, &mashupsdk.MashupDetailedElementBundle{
		AuthToken:        " ",
		DetailedElements: []*mashupsdk.MashupDetailedElement{&element},
	})

	if upsertErr != nil {
		log.Printf("Element state initialization failure: %s\n", upsertErr.Error())
	}
	gchatApp.DetailedElements = detailedElements.DetailedElements
}

func ProcessGChatAnswer(msg *mashupsdk.MashupDetailedElement) {
	log.Printf("Message is ready to send to Google Chat user")
}

func ProcessDFQuery(msg *mashupsdk.MashupDetailedElement) {
	log.Printf("DialogFlow received message to process")
	// Map query to a function call

	if msg.Data == "Hello Flume!" {
		msg.Data = "Hello World response"
	}

	element := mashupsdk.MashupDetailedElement{
		Name: "ChatGptQuery",
		Id:   msg.Id,
		Data: msg.Data,
	}

	DetailedElements := []*mashupsdk.MashupDetailedElement{}
	DetailedElements = append(DetailedElements, &element)

	log.Printf("Delivering mashup elements.\n")

	_, upsertErr := gchatApp.GoogleChatContext.MashupContext.Client.UpsertElements(gchatApp.GoogleChatContext.MashupContext, //flumeworld.FlumeWorldContext.C
		&mashupsdk.MashupDetailedElementBundle{
			AuthToken:        " ", //client.GetServerAuthToken()
			DetailedElements: DetailedElements,
		})

	if upsertErr != nil {
		log.Printf("Element state initialization failure: %s\n", upsertErr.Error())
	}

	log.Printf("Mashup elements delivered.\n")
}

func ProcessDFResponse(msg *mashupsdk.MashupDetailedElement) {
	log.Printf("DialogFlow received response from Flume to format")

}

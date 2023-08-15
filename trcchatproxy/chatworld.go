package main

import (
	"log"

	"github.com/trimble-oss/tierceron-nute/mashupsdk"
	//sdk "github.com/trimble-oss/tierceron-nute/mashupsdk"
	"github.com/trimble-oss/tierceron-nute/mashupsdk/client"
	"github.com/trimble-oss/tierceron-nute/mashupsdk/server"
	"github.com/trimble-oss/tierceron/trcflow/core/askflumeserver"
)

// var chatworld GChatApp

type WorldClientInitHandler struct {
}

// type MashupSdkApiHandler struct {
// }

type igchat interface {
	OnDisplayChange(displayHint *mashupsdk.MashupDisplayHint)
	GetElements() (*mashupsdk.MashupDetailedElementBundle, error)
	UpsertElements(detailedElementBundle *mashupsdk.MashupDetailedElementBundle) (*mashupsdk.MashupDetailedElementBundle, error)
	ChatUpsertElements(detailedElementBundle *mashupsdk.MashupDetailedElementBundle) (*mashupsdk.MashupDetailedElementBundle, error)
	TweakStates(elementStateBundle *mashupsdk.MashupElementStateBundle) (*mashupsdk.MashupElementStateBundle, error)
	ResetStates()
	TweakStatesByMotiv(mashupsdk.Motiv)
	ResetG3NDetailedElementStates()
	UpsertMashupElementsState(elementStateBundle *mashupsdk.MashupElementStateBundle) (*mashupsdk.MashupElementStateBundle, error)
	UpsertMashupElements(detailedElementBundle *mashupsdk.MashupDetailedElementBundle) (*mashupsdk.MashupDetailedElementBundle, error)
	OnResize(displayHint *mashupsdk.MashupDisplayHint)
	GetMashupElements() (*mashupsdk.MashupDetailedElementBundle, error)
}

type GoogleChatHandler struct {
	GoogleChatHandler igchat
}

func New(handler igchat) *GoogleChatHandler {
	return &GoogleChatHandler{
		GoogleChatHandler: handler,
	}
}

func (msdk *GoogleChatHandler) OnDisplayChange(displayHint *mashupsdk.MashupDisplayHint) {
	return
}

func (msdk *GoogleChatHandler) GetElements() (*mashupsdk.MashupDetailedElementBundle, error) {
	return &mashupsdk.MashupDetailedElementBundle{
		AuthToken: client.GetServerAuthToken(),
		// DetailedElements: chatworld.DetailedElements,
	}, nil
}

func (msdk *GoogleChatHandler) UpsertElements(detailedElementBundle *mashupsdk.MashupDetailedElementBundle) (*mashupsdk.MashupDetailedElementBundle, error) {
	log.Printf("Google Chat world received upsert elements: %v", detailedElementBundle)
	return &mashupsdk.MashupDetailedElementBundle{
		AuthToken: client.GetServerAuthToken(),
		// DetailedElements: chatworld.DetailedElements,
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
		AuthToken: client.GetServerAuthToken(),
		// DetailedElements: chatworld.DetailedElements,
	}, nil
}

func (msdk *GoogleChatHandler) OnResize(displayHint *mashupsdk.MashupDisplayHint) {
	return
}

func (msdk *GoogleChatHandler) UpsertMashupElements(detailedElementBundle *mashupsdk.MashupDetailedElementBundle) (*mashupsdk.MashupDetailedElementBundle, error) {
	log.Printf("Google Chat world received upsert elements: %v", detailedElementBundle)
	// RegisterFromFlume(detailedElementBundle)
	return &mashupsdk.MashupDetailedElementBundle{
		AuthToken: client.GetServerAuthToken(),
		// DetailedElements: chatworld.DetailedElements,
	}, nil
}

func (msdk *GoogleChatHandler) UpsertMashupElementsState(elementStateBundle *mashupsdk.MashupElementStateBundle) (*mashupsdk.MashupElementStateBundle, error) {
	return elementStateBundle, nil
}

func (msdk *GoogleChatHandler) ResetG3NDetailedElementStates() {
	return
}

func (w *WorldClientInitHandler) RegisterContext(context *mashupsdk.MashupContext) {
	// chatworld.GoogleChatContext.MashupContext = context
}

func ProcessQuery(message *mashupsdk.MashupDetailedElementBundle) {
	msg := message.DetailedElements[0]
	if msg.Name == "GChatQuery" {
		ProcessGChatQuery(msg)
	} else {
		ProcessDFQuery(msg)
	}
}

func ProcessGChatQuery(msg *mashupsdk.MashupDetailedElement) {
	//Send to google chat
	log.Printf("Message sent to google chat user")
}

func ProcessDFQuery(msg *mashupsdk.MashupDetailedElement) {
	log.Printf("DialogFlow received message to process")
	element := mashupsdk.MashupDetailedElement{
		Name: "ChatGptAnswer",
		Id:   msg.Id,
		Data: "Hello Server!",
	}
	DetailedElements := []*mashupsdk.MashupDetailedElement{}
	DetailedElements = append(DetailedElements, &element)
	// for _, detailedElement := range gchatApp.MashupDetailedElementLibrary {
	// 	DetailedElements = append(DetailedElements, detailedElement)
	// }
	log.Printf("Delivering mashup elements.\n")

	// Need to make client Before:::
	// serverConnectionConfigs = client.GetServerConfigs()
	// client.SetServerConfigs(serverConnectionConfigs)

	// Connection with mashup fully established.  Initialize mashup elements.
	// trccontext.SetContext(flumeworld.FlumeWorldContext)
	c := client.MashupClient{}
	flumehandler := askflumeserver.FlumeChat{}
	client.SetHandler(&c, flumehandler.FlumeChat)
	server.SetServerConfigs(serverConnectionConfigs)
	_, upsertErr := c.UpsertElements(flumeworld.FlumeWorldContext, //flumeworld.FlumeWorldContext.C
		&mashupsdk.MashupDetailedElementBundle{
			AuthToken:        "c5376ccf9edc2a02499716c7e4f5599e8a96747e8a762c8ebed7a45074ad192a", //client.GetServerAuthToken()
			DetailedElements: DetailedElements,
		})

	if upsertErr != nil {
		log.Printf("Element state initialization failure: %s\n", upsertErr.Error())
	}

	log.Printf("Mashup elements delivered.\n")
}

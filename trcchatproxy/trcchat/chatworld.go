package trcchat

import (
	"log"

	"github.com/trimble-oss/tierceron-nute/mashupsdk"
	"github.com/trimble-oss/tierceron-nute/mashupsdk/client"
)

type WorldClientInitHandler struct {
}

type GoogleChatHandler struct {
}

type GoogleChatContext struct {
	MashupContext *mashupsdk.MashupContext // Needed for callbacks to other mashups
}

type GChatApp struct {
	MashupSdkApiHandler          *GoogleChatHandler
	GoogleChatContext            *GoogleChatContext
	WClientInitHandler           *WorldClientInitHandler
	DetailedElements             []*mashupsdk.MashupDetailedElement
	MashupDetailedElementLibrary map[int64]*mashupsdk.MashupDetailedElement
	ElementLoaderIndex           map[string]int64 // mashup indexes by Name
}

func (msdk *GoogleChatHandler) OnDisplayChange(displayHint *mashupsdk.MashupDisplayHint) {
}

func (msdk *GoogleChatHandler) GetElements() (*mashupsdk.MashupDetailedElementBundle, error) {
	return &mashupsdk.MashupDetailedElementBundle{
		AuthToken:        client.GetServerAuthToken(),
		DetailedElements: gchatApp.DetailedElements,
	}, nil
}

func (msdk *GoogleChatHandler) UpsertElements(detailedElementBundle *mashupsdk.MashupDetailedElementBundle) (*mashupsdk.MashupDetailedElementBundle, error) {
	log.Printf("Google Chat world received upsert elements: %v", detailedElementBundle.DetailedElements)

	gchatApp.DetailedElements = detailedElementBundle.DetailedElements
	for _, element := range gchatApp.DetailedElements {
		ProcessQuery(element)
	}
	log.Println("Sending data back to client")
	return &mashupsdk.MashupDetailedElementBundle{
		AuthToken:        client.GetServerAuthToken(),
		DetailedElements: gchatApp.DetailedElements,
	}, nil
}

func (msdk *GoogleChatHandler) TweakStates(elementStateBundle *mashupsdk.MashupElementStateBundle) (*mashupsdk.MashupElementStateBundle, error) {
	return elementStateBundle, nil
}

func (msdk *GoogleChatHandler) ResetStates() {
}

func (msdk *GoogleChatHandler) TweakStatesByMotiv(mashupsdk.Motiv) {
}

func (msdk *GoogleChatHandler) GetMashupElements() (*mashupsdk.MashupDetailedElementBundle, error) {
	return &mashupsdk.MashupDetailedElementBundle{
		AuthToken:        client.GetServerAuthToken(),
		DetailedElements: gchatApp.DetailedElements,
	}, nil
}

func (msdk *GoogleChatHandler) OnResize(displayHint *mashupsdk.MashupDisplayHint) {
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
}

func (w *WorldClientInitHandler) RegisterContext(context *mashupsdk.MashupContext) {
	gchatApp.GoogleChatContext.MashupContext = context
}

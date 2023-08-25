package askflumeserver

import (
	"log"

	"github.com/trimble-oss/tierceron-nute/mashupsdk"
	flowcore "github.com/trimble-oss/tierceron/trcflow/core"

	//sdk "github.com/trimble-oss/tierceron-nute/mashupsdk"
	"github.com/trimble-oss/tierceron-nute/mashupsdk/client"
	"github.com/trimble-oss/tierceron-nute/mashupsdk/server"
)

var Flumeworld FlumeWorldApp

type WorldClientInitHandler struct {
}

// type MashupSdkApiHandler struct {
// 	ChatHandler bool
// }

type FlumeHandler struct {
}

type FlumeWorldContext struct {
	MashupContext *mashupsdk.MashupContext // Needed for callbacks to other mashups
}

type FlumeWorldApp struct {
	MashupSdkApiHandler          *FlumeHandler
	FlumeWorldContext            *mashupsdk.MashupContext //*FlumeWorldContext
	mashupDisplayContext         *mashupsdk.MashupDisplayContext
	WClientInitHandler           *WorldClientInitHandler
	DetailedElements             []*mashupsdk.MashupDetailedElement
	MashupDetailedElementLibrary map[int64]*mashupsdk.MashupDetailedElement
	ElementLoaderIndex           map[string]int64 // mashup indexes by Name
}

func (msdk *FlumeHandler) OnDisplayChange(displayHint *mashupsdk.MashupDisplayHint) {
	return
}

func (msdk *FlumeHandler) GetElements() (*mashupsdk.MashupDetailedElementBundle, error) {
	return &mashupsdk.MashupDetailedElementBundle{
		AuthToken:        client.GetServerAuthToken(),
		DetailedElements: Flumeworld.DetailedElements,
	}, nil
}

func (msdk *FlumeHandler) UpsertElements(detailedElementBundle *mashupsdk.MashupDetailedElementBundle) (*mashupsdk.MashupDetailedElementBundle, error) {
	log.Printf("Flume world received upsert elements: %v", detailedElementBundle)
	// flows.GetGChatQuery(detailedElementBundle)
	element := detailedElementBundle.DetailedElements[0]
	element.Id = flowcore.GetId()
	elements := []*mashupsdk.MashupDetailedElement{element}
	Flumeworld.DetailedElements = elements
	GetQuery(detailedElementBundle)
	// Flumeworld.FlumeWorldContext.Client.UpsertElements(Flumeworld.FlumeWorldContext, &mashupsdk.MashupDetailedElementBundle{
	// 	AuthToken:        " ",
	// 	DetailedElements: Flumeworld.DetailedElements,
	// })
	// server.UpsertElements(flumeworld.FlumeWorldContext.Context, detailedElementBundle.AuthToken)

	return &mashupsdk.MashupDetailedElementBundle{
		AuthToken:        client.GetServerAuthToken(),
		DetailedElements: elements,
	}, nil
}

func (msdk *FlumeHandler) TweakStates(elementStateBundle *mashupsdk.MashupElementStateBundle) (*mashupsdk.MashupElementStateBundle, error) {
	return elementStateBundle, nil
}

func (msdk *FlumeHandler) ResetStates() {
	return
}

func (msdk *FlumeHandler) TweakStatesByMotiv(mashupsdk.Motiv) {
	return
}

func (msdk *FlumeHandler) GetMashupElements() (*mashupsdk.MashupDetailedElementBundle, error) {
	return &mashupsdk.MashupDetailedElementBundle{
		AuthToken:        client.GetServerAuthToken(),
		DetailedElements: Flumeworld.DetailedElements,
	}, nil
}

func (msdk *FlumeHandler) OnResize(displayHint *mashupsdk.MashupDisplayHint) {
	return
}

func (msdk *FlumeHandler) UpsertMashupElements(detailedElementBundle *mashupsdk.MashupDetailedElementBundle) (*mashupsdk.MashupDetailedElementBundle, error) {
	log.Printf("Flume world received upsert elements: %v", detailedElementBundle)
	return &mashupsdk.MashupDetailedElementBundle{
		AuthToken:        client.GetServerAuthToken(),
		DetailedElements: Flumeworld.DetailedElements,
	}, nil
}

func (msdk *FlumeHandler) UpsertMashupElementsState(elementStateBundle *mashupsdk.MashupElementStateBundle) (*mashupsdk.MashupElementStateBundle, error) {
	return elementStateBundle, nil
}

func (msdk *FlumeHandler) ResetG3NDetailedElementStates() {
	return
}

func (w *WorldClientInitHandler) RegisterContext(context *mashupsdk.MashupContext) {
	Flumeworld.FlumeWorldContext = context
}

func (w *FlumeWorldApp) InitServer(callerCreds string, insecure bool, maxMessageLength int) {
	if callerCreds != "" {
		server.InitServer(callerCreds, insecure, maxMessageLength, w.MashupSdkApiHandler, w.WClientInitHandler)
	}
}

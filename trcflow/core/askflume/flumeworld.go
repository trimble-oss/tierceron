package askflume

import (
	"log"

	"github.com/trimble-oss/tierceron-nute/mashupsdk"

	"github.com/trimble-oss/tierceron-nute/mashupsdk/client"
	"github.com/trimble-oss/tierceron-nute/mashupsdk/server"
)

var Flumeworld FlumeWorldApp

type WorldClientInitHandler struct {
}

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
}

func (msdk *FlumeHandler) GetElements() (*mashupsdk.MashupDetailedElementBundle, error) {
	return &mashupsdk.MashupDetailedElementBundle{
		AuthToken:        client.GetServerAuthToken(),
		DetailedElements: Flumeworld.DetailedElements,
	}, nil
}

func (msdk *FlumeHandler) UpsertElements(detailedElementBundle *mashupsdk.MashupDetailedElementBundle) (*mashupsdk.MashupDetailedElementBundle, error) {
	log.Printf("Flume world received upsert elements: %v", detailedElementBundle)

	return &mashupsdk.MashupDetailedElementBundle{
		AuthToken:        client.GetServerAuthToken(),
		DetailedElements: nil,
	}, nil
}

func (msdk *FlumeHandler) TweakStates(elementStateBundle *mashupsdk.MashupElementStateBundle) (*mashupsdk.MashupElementStateBundle, error) {
	return elementStateBundle, nil
}

func (msdk *FlumeHandler) ResetStates() {
}

func (msdk *FlumeHandler) TweakStatesByMotiv(mashupsdk.Motiv) {
}

func (msdk *FlumeHandler) GetMashupElements() (*mashupsdk.MashupDetailedElementBundle, error) {
	return &mashupsdk.MashupDetailedElementBundle{
		AuthToken:        client.GetServerAuthToken(),
		DetailedElements: Flumeworld.DetailedElements,
	}, nil
}

func (msdk *FlumeHandler) OnResize(displayHint *mashupsdk.MashupDisplayHint) {
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
}

func (w *WorldClientInitHandler) RegisterContext(context *mashupsdk.MashupContext) {
	Flumeworld.FlumeWorldContext = context
}

func (w *FlumeWorldApp) InitServer(callerCreds string, insecure bool, maxMessageLength int) {
	if callerCreds != "" {
		server.InitServer(callerCreds, insecure, maxMessageLength, w.MashupSdkApiHandler, w.WClientInitHandler)
	}
}

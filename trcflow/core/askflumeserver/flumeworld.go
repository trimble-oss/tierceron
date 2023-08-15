package askflumeserver

import (
	"log"

	"github.com/trimble-oss/tierceron-nute/mashupsdk"
	//sdk "github.com/trimble-oss/tierceron-nute/mashupsdk"
	"github.com/trimble-oss/tierceron-nute/mashupsdk/client"
	"github.com/trimble-oss/tierceron-nute/mashupsdk/server"
)

var flumeworld FlumeWorldApp

type WorldClientInitHandler struct {
}

// type MashupSdkApiHandler struct {
// 	ChatHandler bool
// }

type ichat interface {
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

type FlumeChat struct {
	FlumeChat ichat
}

func New(flumechat ichat) *FlumeChat {
	return &FlumeChat{
		FlumeChat: flumechat,
	}
}

type FlumeWorldContext struct {
	MashupContext *mashupsdk.MashupContext // Needed for callbacks to other mashups
}

type FlumeWorldApp struct {
	MashupSdkApiHandler          *FlumeChat
	FlumeWorldContext            *mashupsdk.MashupContext //*FlumeWorldContext
	mashupDisplayContext         *mashupsdk.MashupDisplayContext
	WClientInitHandler           *WorldClientInitHandler
	DetailedElements             []*mashupsdk.MashupDetailedElement
	MashupDetailedElementLibrary map[int64]*mashupsdk.MashupDetailedElement
	ElementLoaderIndex           map[string]int64 // mashup indexes by Name
}

func (msdk *FlumeChat) OnDisplayChange(displayHint *mashupsdk.MashupDisplayHint) {
	return
}

func (msdk *FlumeChat) GetElements() (*mashupsdk.MashupDetailedElementBundle, error) {
	return &mashupsdk.MashupDetailedElementBundle{
		AuthToken:        client.GetServerAuthToken(),
		DetailedElements: flumeworld.DetailedElements,
	}, nil
}

func (msdk *FlumeChat) UpsertElements(detailedElementBundle *mashupsdk.MashupDetailedElementBundle) (*mashupsdk.MashupDetailedElementBundle, error) {
	log.Printf("Flume world received upsert elements: %v", detailedElementBundle)
	// flows.GetGChatQuery(detailedElementBundle)
	GetQuery(detailedElementBundle)
	return &mashupsdk.MashupDetailedElementBundle{
		AuthToken:        client.GetServerAuthToken(),
		DetailedElements: flumeworld.DetailedElements,
	}, nil
}

// func (msdk *FlumeChat) FlumeUpsertElements(detailedElementBundle *mashupsdk.MashupDetailedElementBundle) (*mashupsdk.MashupDetailedElementBundle, error) {
// 	return msdk.UpsertElements(detailedElementBundle)
// }

func (msdk *FlumeChat) ChatUpsertElements(detailedElementBundle *mashupsdk.MashupDetailedElementBundle) (*mashupsdk.MashupDetailedElementBundle, error) {
	log.Printf("Flume Chat world received upsert elements: %v", detailedElementBundle)
	// flumeworldhandler := trccontext.New(trccontext.FlumeWorld(*msdk))
	flumehandler := New(msdk)
	return flumehandler.FlumeChat.ChatUpsertElements(detailedElementBundle)
	// return &mashupsdk.MashupDetailedElementBundle{
	// 	AuthToken: client.GetServerAuthToken(),
	// 	// DetailedElements: chatworld.DetailedElements,
	// }, nil
}

func (msdk *FlumeChat) TweakStates(elementStateBundle *mashupsdk.MashupElementStateBundle) (*mashupsdk.MashupElementStateBundle, error) {
	return elementStateBundle, nil
}

func (msdk *FlumeChat) ResetStates() {
	return
}

func (msdk *FlumeChat) TweakStatesByMotiv(mashupsdk.Motiv) {
	return
}

func (msdk *FlumeChat) GetMashupElements() (*mashupsdk.MashupDetailedElementBundle, error) {
	return &mashupsdk.MashupDetailedElementBundle{
		AuthToken:        client.GetServerAuthToken(),
		DetailedElements: flumeworld.DetailedElements,
	}, nil
}

func (msdk *FlumeChat) OnResize(displayHint *mashupsdk.MashupDisplayHint) {
	return
}

func (msdk *FlumeChat) UpsertMashupElements(detailedElementBundle *mashupsdk.MashupDetailedElementBundle) (*mashupsdk.MashupDetailedElementBundle, error) {
	log.Printf("Flume world received upsert elements: %v", detailedElementBundle)
	return &mashupsdk.MashupDetailedElementBundle{
		AuthToken:        client.GetServerAuthToken(),
		DetailedElements: flumeworld.DetailedElements,
	}, nil
}

func (msdk *FlumeChat) UpsertMashupElementsState(elementStateBundle *mashupsdk.MashupElementStateBundle) (*mashupsdk.MashupElementStateBundle, error) {
	return elementStateBundle, nil
}

func (msdk *FlumeChat) ResetG3NDetailedElementStates() {
	return
}

func (w *WorldClientInitHandler) RegisterContext(context *mashupsdk.MashupContext) {
	flumeworld.FlumeWorldContext = context
}

func (w *FlumeWorldApp) InitServer(callerCreds string, insecure bool, maxMessageLength int) {
	if callerCreds != "" {
		server.InitServer(callerCreds, insecure, maxMessageLength, w.MashupSdkApiHandler, w.WClientInitHandler)
	}
}

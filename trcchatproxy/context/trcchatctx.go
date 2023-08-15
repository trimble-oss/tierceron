package trcchatproxy

// type igchathandler interface {
// 	OnDisplayChange(displayHint *mashupsdk.MashupDisplayHint)
// 	GetElements() (*mashupsdk.MashupDetailedElementBundle, error)
// 	UpsertElements(detailedElementBundle *mashupsdk.MashupDetailedElementBundle) (*mashupsdk.MashupDetailedElementBundle, error)
// 	TweakStates(elementStateBundle *mashupsdk.MashupElementStateBundle) (*mashupsdk.MashupElementStateBundle, error)
// 	ResetStates()
// 	TweakStatesByMotiv(mashupsdk.Motiv)
// 	ResetG3NDetailedElementStates()
// 	UpsertMashupElementsState(elementStateBundle *mashupsdk.MashupElementStateBundle)
// 	UpsertMashupElements(detailedElementBundle *mashupsdk.MashupDetailedElementBundle)
// 	OnResize(displayHint *mashupsdk.MashupDisplayHint)
// 	GetMashupElements()
// }

type FlumeWorldApiHandler struct {
}

type GChatHandler struct {
}

// trccontext "github.com/trimble-oss/tierceron/trcchatproxy"

// "github.com/trimble-oss/tierceron-nute/mashupsdk"

// var context *mashupsdk.MashupContext
// type GoogleChatContext struct {
// 	MashupContext *mashupsdk.MashupContext // Needed for callbacks to other mashups
// }

// type GChatApp struct {
// 	MashupSdkApiHandler          *trccontext.MashupSdkApiHandler
// 	GoogleChatContext            *GoogleChatContext //*FlumeWorldContext
// 	mashupDisplayContext         *mashupsdk.MashupDisplayContext
// 	WClientInitHandler           *trccontext.WorldClientInitHandler
// 	DetailedElements             []*mashupsdk.MashupDetailedElement
// 	MashupDetailedElementLibrary map[int64]*mashupsdk.MashupDetailedElement
// 	ElementLoaderIndex           map[string]int64 // mashup indexes by Name
// }

// func (w *GChatApp) InitServer(callerCreds string, insecure bool, maxMessageLength int) {
// 	if callerCreds != "" {
// 		server.InitServer(callerCreds, insecure, maxMessageLength, w.MashupSdkApiHandler, w.WClientInitHandler)
// 	}
// }

func InitFlumeWorld() {

}

// func SetContext(ctx *mashupsdk.MashupContext) {
// 	context = ctx
// }

// func GetContext() *mashupsdk.MashupContext {
// 	return context
// }

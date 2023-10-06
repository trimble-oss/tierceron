package pubsub

import "google.golang.org/api/chat/v1"

var manualInteraction bool = true
var chatEvents chan *chat.DeprecatedEvent
var chatAnswerEvents chan *chat.DeprecatedEvent

func CommonInit(manualInteract bool) {
	manualInteraction = manualInteract
	chatEvents = make(chan *chat.DeprecatedEvent, 30)
}

func IsManualInteractionEnabled() bool {
	return manualInteraction
}

func PubChatEvent(event *chat.DeprecatedEvent) {
	chatEvents <- event
}

func SubChatEvent() *chat.DeprecatedEvent {
	select {
	case event := <-chatEvents:
		return event
	}
}

func PubChatAnswerEvent(event *chat.DeprecatedEvent) {
	chatAnswerEvents <- event
}

func SubChatAnswerEvent() *chat.DeprecatedEvent {
	select {
	case event := <-chatAnswerEvents:
		return event
	}
}

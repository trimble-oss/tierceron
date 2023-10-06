package pubsub

import "google.golang.org/api/chat/v1"

var manualInteraction bool = true
var chatEvents chan *chat.DeprecatedEvent

func CommonInit(manualInteract bool) {
	manualInteraction = manualInteract
	chatEvents = make(chan *chat.DeprecatedEvent, 30)
}

func IsManualInteractionEnabled() bool {
	return manualInteraction
}

func PubEvent(event *chat.DeprecatedEvent) {
	chatEvents <- event
}

func SubEvent() *chat.DeprecatedEvent {
	select {
	case event := <-chatEvents:
		return event
	}
}

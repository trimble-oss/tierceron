package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/trimble-oss/tierceron/trcchatproxy/pubsub"
	"github.com/trimble-oss/tierceron/trcchatproxy/trcchat"
	"google.golang.org/api/chat/v1"
)

func get_port() string {
	port := ":8080"
	if val, ok := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT"); ok {
		port = ":" + val
	}
	return port
}

// Will be the google chat api implementation --> Send messages from googlechat.go to the specified server and port
// This is just an example of what the method should look like based on the above link
func main() {
	pubsub.CommonInit(false)
	trcchat.CommonInit()
	f := func(w http.ResponseWriter, r *http.Request) {

		switch r.Method {
		case http.MethodGet:
			w.Header().Add("Content-Type", "application/json")
			fmt.Fprintf(w, `{"text": "Ask Flume has been enabled."}`)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var event chat.DeprecatedEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err == nil {
			w.Header().Add("Content-Type", "application/json")
			switch event.Type {
			case "ADDED_TO_SPACE":
				if event.Space.Type != "ROOM" {
					break
				}
				fmt.Fprint(w, `{"text":"Google Chat App has been successfully added to this space!"}`)
			case "REMOVED_FROM_SPACE":
				if event.Space.Type != "ROOM" {
					break
				}
				fmt.Fprint(w, `{"text":"Google Chat App has been successfully removed from this space!"}`)
			case "MESSAGE":
				go pubsub.PubChatEvent(&event)
				responseEvent := pubsub.SubChatAnswerEvent()
				fmt.Fprintf(w, `{"text": "%s"}`, responseEvent.Message.Text)
			default:
				fmt.Fprintf(w, `{"text": "you said %s"}`, event.Message.Text)
			}
			return
		}

	}
	log.Fatal(http.ListenAndServe(get_port(), http.HandlerFunc(f)))
}

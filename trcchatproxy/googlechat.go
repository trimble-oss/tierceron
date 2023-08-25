package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	chat "google.golang.org/api/chat/v1"
)

func InitGoogleChat() {
	f := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var event chat.DeprecatedEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}

		switch event.Type {
		case "ADDED_TO_SPACE":
			if event.Space.Type != "ROOM" {
				break
			}
			fmt.Fprint(w, `{"text":"Google Chat App has been successfully added to this space!"}`)
		case "MESSAGE":
			// Send message to DialogFlow
			fmt.Fprintf(w, `{"text": "you said %s"}`, event.Message.Text)
		}
	}
	log.Fatal(http.ListenAndServe(":0", http.HandlerFunc(f)))
}

// https://medium.com/google-cloud/google-chat-bot-go-cc91c5311d7e

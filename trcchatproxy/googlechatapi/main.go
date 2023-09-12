package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"google.golang.org/api/chat/v1"
)

// https://medium.com/google-cloud/google-chat-bot-go-cc91c5311d7e

// Will be the google chat api implementation --> Send messages from googlechat.go to the specified server and port
// This is just an example of what the method should look like based on the above link
func main() {
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
			fmt.Fprintf(w, `{"text": "you said %s"}`, event.Message.Text)
		}
	}
	log.Fatal(http.ListenAndServe(":8080", http.HandlerFunc(f)))
}

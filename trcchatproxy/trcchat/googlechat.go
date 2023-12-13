package trcchat

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/trimble-oss/tierceron-nute/mashupsdk"
	"github.com/trimble-oss/tierceron/trcchatproxy/pubsub"
	"google.golang.org/api/chat/v1"
)

// Prints out answer to google chat user and sets up google chat app
// to ask another response
// Once the google chat api is set up, should send response to
// correct endpoint for google chat
func ProcessGChatAnswer(msg *mashupsdk.MashupDetailedElement) {
	log.Printf("Message is ready to send to Google Chat user")
	var infos [][]interface{}

	err := json.Unmarshal([]byte(msg.Data), &infos)
	if err != nil {
		log.Println("Error in decoding data in recursiveBuildArgosies")
	}

	for _, info := range infos {
		switch {
		case msg.Alias == "Active":
			tenant := info[0]
			enterpriseId := info[1]
			error_msg := info[2]
			err_time := info[3]
			// Stack trace was empty, so skipping index of 4
			snap_mode := info[5]
			msg.Data = fmt.Sprintf("The pipeline tenant %v with enterprise ID %v is running with last error message %v at %v and snapshot mode %v", tenant, enterpriseId, error_msg, err_time, snap_mode)
		case msg.Alias == "Error":
			tenant := info[0]
			enterpriseId := info[1]
			error_msg := info[2]
			err_time := info[3]
			// Stack trace was empty, so skipping index of 4
			snap_mode := info[5]
			msg.Data = fmt.Sprintf("The pipeline tenant %v with enterprise ID %v is failing with error message %v at %v with snapshot mode %v", tenant, enterpriseId, error_msg, err_time, snap_mode)

		case msg.Alias == "Tenant":
		case msg.Alias == "DataFlowState":
			tenant := info[0]
			flowName := info[1]
			//_ := info[2]
			stateName := info[3]
			lastTestedDate := info[4]
			msg.Data = fmt.Sprintf("The pipeline tenant %v and flow name %v last failed with error: %v on %v", tenant, flowName, stateName, lastTestedDate)

		}
		if pubsub.IsManualInteractionEnabled() {
			fmt.Println(msg.Data)
		} else {
			go pubsub.PubChatAnswerEvent(&chat.DeprecatedEvent{Message: &chat.Message{ClientAssignedMessageId: msg.Genre, Text: msg.Data}})
			log.Printf("Message published to answer channel for delivery to Google Chat user")
		}
	}

	element := mashupsdk.MashupDetailedElement{
		Id:   msg.Id,
		Name: "GChatAnswer",
		Data: msg.Data,
	}
	offset := GetId()
	if gchatApp.MashupDetailedElementLibrary == nil {
		gchatApp.MashupDetailedElementLibrary = make(map[int64]*mashupsdk.MashupDetailedElement)
	}
	gchatApp.MashupDetailedElementLibrary[msg.Id+offset] = gchatApp.DetailedElements[0]
	gchatApp.DetailedElements = []*mashupsdk.MashupDetailedElement{&element}
}

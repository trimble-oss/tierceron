package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/trimble-oss/tierceron-nute/mashupsdk"
)

// Prints out answer to google chat user and sets up google chat app
// to ask another response
// Once the google chat api is set up, should send response to
// correct endpoint for google chat
func ProcessGChatAnswer(msg *mashupsdk.MashupDetailedElement) {
	log.Printf("Message is ready to send to Google Chat user")
	var info [][]interface{}

	err := json.Unmarshal([]byte(msg.Data), &info)
	if err != nil {
		log.Println("Error in decoding data in recursiveBuildArgosies")
	}

	tenant := info[0][0]
	enterpriseId := info[0][1]
	error_msg := info[0][2]
	err_time := info[0][3]
	// Stack trace was empty, so skipping index of 4
	snap_mode := info[0][5]
	msg.Data = fmt.Sprintf("The tenant %v with enterprise ID %v is failing with error message %v at %v with snapshot mode %v", tenant, enterpriseId, error_msg, err_time, snap_mode)
	fmt.Println(msg.Data)
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

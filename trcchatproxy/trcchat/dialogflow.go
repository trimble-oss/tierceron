package trcchat

import (
	"log"
	"strings"

	"github.com/trimble-oss/tierceron-nute/mashupsdk"
)

// This is currently a stub version of processing a DialogFlow query
// When the dialogflow api has been implemented, send the msg provided
// in the parameter to the endpoint of the dialogflowapi
func ProcessDFQuery(msg *mashupsdk.MashupDetailedElement) {
	log.Printf("DialogFlow received message to process")

	// Map query to a function call
	if strings.Contains(strings.ToLower(msg.Data), "error") || strings.Contains(strings.ToLower(msg.Data), "fail") {
		msg.Data = "Error"
		msg.Alias = "120"
	} else if strings.Contains(strings.ToLower(msg.Data), "tenant") {
		msg.Data = "Tenant"
		msg.Alias = "plum-co"
	} else if strings.Contains(strings.ToLower(msg.Data), "active") {
		msg.Data = "Active"
		msg.Alias = "2"
	} else if strings.Contains(strings.ToLower(msg.Data), "ninja") || strings.Contains(strings.ToLower(msg.Data), "tests") {
		msg.Data = "DataFlowState"
		msg.Alias = "3"
	}

	// Change message type
	gchatApp.DetailedElements[msg.Id].Name = "DFQuery"
	gchatApp.DetailedElements[msg.Id].Data = msg.Data
}

// This is currently a stub version of processing a DialogFlow query
// When the dialogflow api has been implemented, send the msg provided
// in the parameter to the endpoint of the dialogflowapi
// This message will need to be interpreted and formated by dialogflow
// to make it understandable to user
func ProcessDFResponse(msg *mashupsdk.MashupDetailedElement) {
	log.Printf("DialogFlow received response from Flume to format")
	gchatApp.DetailedElements[msg.Id].Name = "DFAnswer"
	response_type := gchatApp.DetailedElements[msg.Id].Alias
	switch {
	case response_type == "Error":
		// Process which flows are erroring
		gchatApp.DetailedElements[msg.Id].Data = "Error message response"
	case response_type == "Tenant":
		// Report status of tenant
		gchatApp.DetailedElements[msg.Id].Data = "Tenant message response"
	case response_type == "Active":
		// Report activitiy status
		gchatApp.DetailedElements[msg.Id].Data = "Activity report message response"
	case response_type == "DataFlowState":
		// Report activitiy status
		gchatApp.DetailedElements[msg.Id].Data = "Ninja Test status"
	default:
		// Unable to process response... please try asking your question again error
		gchatApp.DetailedElements[msg.Id].Data = "Unable to process question"
	}
}

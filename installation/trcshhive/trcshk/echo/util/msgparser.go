package util

import (
	"fmt"
	"strings"
	"time"

	// Update package path as needed
	ttsdk "github.com/trimble-oss/tierceron/installation/trcshhive/trcshk/echo/trcshtalksdk"
	"golang.org/x/exp/rand"
)

var (
	interactions = map[string]ttsdk.Interactions{
		"HEALTH CHECK": ttsdk.Interactions_HEALTH_CHECK,
		"ALL":          ttsdk.Interactions_ALL,
	}

	acceptableTests = []string{
		"Query",
	}
)

func GenMsgId(env string) string {
	rand.Seed(uint64(time.Now().UnixNano()))
	randomNumber := rand.Intn(10000000) // Generate a number between 0 and 10000000
	return fmt.Sprintf("%s:%d", env, randomNumber)
}

func ParseInteractions(message string) []ttsdk.Interactions {
	var requestedInteractions []ttsdk.Interactions
	// find and add interactions
	upperMessage := strings.ToUpper(message)
	for interaction, protoValue := range interactions {
		if strings.Contains(upperMessage, interaction) {
			requestedInteractions = append(requestedInteractions, protoValue)
		}
	}
	// no interactions requested, default to all
	if len(requestedInteractions) == 0 {
		requestedInteractions = append(requestedInteractions, ttsdk.Interactions_ALL)
	}
	return requestedInteractions
}

func ParseTenantID(message string) string {
	msg_split := strings.Split(message, "tenantID:")
	if len(msg_split) == 2 {
		tenant_split := strings.Split(msg_split[1], ":")
		if len(tenant_split) > 0 {
			return tenant_split[0]
		}
	}
	return ""
}

func ParseData(message string) []string {
	requestedData := make([]string, 0)
	// find and add interaction data
	for _, requestedTest := range acceptableTests {
		if strings.Contains(strings.ToUpper(message), strings.ToUpper(requestedTest)) {
			requestedData = append(requestedData, requestedTest)
		}
	}
	return requestedData
}

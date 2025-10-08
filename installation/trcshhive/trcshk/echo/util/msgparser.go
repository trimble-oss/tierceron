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
	diagnostics = map[string]ttsdk.Diagnostics{
		"HEALTH CHECK": ttsdk.Diagnostics_HEALTH_CHECK,
		"ALL":          ttsdk.Diagnostics_ALL,
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

func ParseDiagnostics(message string) []ttsdk.Diagnostics {
	var requestedDiagnostics []ttsdk.Diagnostics
	// find and add diagnostics
	upperMessage := strings.ToUpper(message)
	for diagnostic, protoValue := range diagnostics {
		if strings.Contains(upperMessage, diagnostic) {
			requestedDiagnostics = append(requestedDiagnostics, protoValue)
		}
	}
	// no diagnostics requested, default to all
	if len(requestedDiagnostics) == 0 {
		requestedDiagnostics = append(requestedDiagnostics, ttsdk.Diagnostics_ALL)
	}
	return requestedDiagnostics
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
	// find and add diagnostics
	for _, requestedTest := range acceptableTests {
		if strings.Contains(strings.ToUpper(message), strings.ToUpper(requestedTest)) {
			requestedData = append(requestedData, requestedTest)
		}
	}
	return requestedData
}

package routinglog

import (
	"bytes"
	"log"
	"testing"
)

func TestRoutingLoggingDefaultsOff(test *testing.T) {
	if IsRoutingLoggingActive() {
		test.Fatal("expected routing logging to default to disabled")
	}
	var output bytes.Buffer
	LogRouting(log.New(&output, "", 0), "routing detail")
	if output.Len() != 0 {
		test.Fatalf("expected routing detail to be suppressed, got %q", output.String())
	}
}

func TestRoutingLoggingCanBeToggled(test *testing.T) {
	defer SetRoutingLoggingActive(false)
	var output bytes.Buffer
	logger := log.New(&output, "", 0)
	SetRoutingLoggingActive(true)
	LogRouting(logger, "Sending query to service: %s.", "test-service")
	if output.String() != "Sending query to service: test-service.\n" {
		test.Fatalf("expected enabled routing detail, got %q", output.String())
	}
	output.Reset()
	SetRoutingLoggingActive(false)
	LogRouting(logger, "routing detail")
	if IsRoutingLoggingActive() || output.Len() != 0 {
		test.Fatalf("expected routing logging to be disabled, got %q", output.String())
	}
	LogRouting(nil, "routing detail")
}

package hcore

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"
)

func TestProxyDirectiveQueuesOneDeliveryPerConnectedHost(t *testing.T) {
	broker := newProxyRequestBroker()
	trcdb := map[string]struct{}{"trcdb": {}}
	rainier := map[string]struct{}{"rainier": {}}
	broker.registerHost("trcdb-0", "trcdb", trcdb)
	broker.registerHost("trcdb-1", "trcdb", trcdb)
	broker.registerHost("rainier-0", "rainier", rainier)

	deliveries := broker.enqueueDirective("trcdb-log-state", &proxyRequestEnvelope{
		messageID:         "log-stop",
		targetPlugins:     []string{"trcdb"},
		serializedRequest: `{"data":["trcdb","log","stop"]}`,
	})
	if deliveries != 2 {
		t.Fatalf("expected two trcdb host deliveries, got %d", deliveries)
	}

	for _, hostID := range []string{"trcdb-0", "trcdb-1"} {
		request, err := broker.dequeue(context.Background(), hostID, "trcdb", trcdb)
		if err != nil || request.messageID != "log-stop" {
			t.Fatalf("expected %s to receive the directive, request=%#v err=%v", hostID, request, err)
		}
	}
	if len(broker.deliveryQueues[hubClientDeliveryKey("rainier", "rainier-0")]) != 0 {
		t.Fatal("unexpected directive delivery to a host that does not run trcdb")
	}

	broker.registerHost("trcdb-2", "trcdb", trcdb)
	request, err := broker.dequeue(context.Background(), "trcdb-2", "trcdb", trcdb)
	if err != nil || request.messageID != "log-stop" {
		t.Fatalf("expected newly registered host to receive the directive, request=%#v err=%v", request, err)
	}
}

func TestProxyDirectiveSurvivesWithoutConnectedPod(t *testing.T) {
	broker := newProxyRequestBroker()
	trcdb := map[string]struct{}{"trcdb": {}}

	deliveries := broker.enqueueDirective("trcdb-log-state", &proxyRequestEnvelope{
		messageID:         "log-start",
		targetPlugins:     []string{"trcdb"},
		serializedRequest: `{"data":["trcdb","log","start"]}`,
	})
	if deliveries != 0 {
		t.Fatalf("expected no immediate deliveries, got %d", deliveries)
	}

	broker.registerHost("trcdb-0", "trcdb", trcdb)
	request, err := broker.dequeue(context.Background(), "trcdb-0", "trcdb", trcdb)
	if err != nil || request.messageID != "log-start" {
		t.Fatalf("expected connected pod to receive the persisted directive, request=%#v err=%v", request, err)
	}
}

func TestOrdinaryRequestQueuesOneDeliveryPerConnectedHost(t *testing.T) {
	broker := newProxyRequestBroker()
	ninja := map[string]struct{}{"ninja": {}}
	rainier := map[string]struct{}{"rainier": {}}
	broker.registerHost("trcshk-ninja-0", "trcshk-ninja", ninja)
	broker.registerHost("trcshk-ninja-1", "trcshk-ninja", ninja)
	broker.registerHost("trcshk-rainier-0", "trcshk-rainier", rainier)
	request := &proxyRequestEnvelope{messageID: "ninja-log-stop", targetPlugins: []string{"ninja"}}

	if deliveries := broker.enqueueFanout(request); deliveries != 2 {
		t.Fatalf("expected two ninja host deliveries, got %d", deliveries)
	}
	for _, hostID := range []string{"trcshk-ninja-0", "trcshk-ninja-1"} {
		delivered, err := broker.dequeue(context.Background(), hostID, "trcshk-ninja", ninja)
		if err != nil || delivered != request {
			t.Fatalf("expected %s to receive the request, request=%#v err=%v", hostID, delivered, err)
		}
	}
	if len(broker.deliveryQueues[hubClientDeliveryKey("trcshk-rainier", "trcshk-rainier-0")]) != 0 {
		t.Fatal("unexpected delivery to a host that does not run ninja")
	}
}

func TestProxyResponseTrackerWaitsForEveryExpectedResponse(t *testing.T) {
	tracker := newProxyResponseTracker()
	tracker.setExpected(2)
	if !tracker.addResponse("first") {
		t.Fatal("expected first response to be accepted")
	}
	select {
	case <-tracker.complete:
		t.Fatal("tracker completed before every expected response arrived")
	default:
	}
	if !tracker.addResponse("second") {
		t.Fatal("expected second response to be accepted")
	}
	select {
	case <-tracker.complete:
	default:
		t.Fatal("tracker did not complete after every expected response arrived")
	}
	if response := tracker.firstResponse(); response != "first" {
		t.Fatalf("expected first response to be retained, got %q", response)
	}
}

func TestOrdinaryRequestWaitsForMatchingHubClient(t *testing.T) {
	broker := newProxyRequestBroker()
	request := &proxyRequestEnvelope{
		messageID:         "ninja-log-stop",
		targetPlugins:     []string{"ninja"},
		serializedRequest: `{"data":["ninja","log","stop"]}`,
	}
	if deliveries := broker.enqueueFanout(request); deliveries != 0 {
		t.Fatalf("expected no immediate deliveries, got %d", deliveries)
	}

	nonmatchingContext, cancel := context.WithCancel(context.Background())
	cancel()
	if delivered, err := broker.dequeue(nonmatchingContext, "rainier-0", "rainier", map[string]struct{}{"rainier": {}}); err == nil || delivered != nil {
		t.Fatalf("expected nonmatching poll to leave the request queued, request=%#v err=%v", delivered, err)
	}

	delivered, err := broker.dequeue(context.Background(), "trcshk-0", "trcshk", map[string]struct{}{"ninja": {}})
	if err != nil || delivered != request {
		t.Fatalf("expected matching ninja poll to claim the queued request, request=%#v err=%v", delivered, err)
	}
	if len(broker.queues) != 0 {
		t.Fatalf("expected request removal only after matching delivery, queues=%#v", broker.queues)
	}
}

func TestTargetedRequestIsDeliveredToOnlyTheRequestedPod(t *testing.T) {
	broker := newProxyRequestBroker()
	trcdb := map[string]struct{}{"trcdb": {}}
	broker.registerHost("trcdb-0", "trcdb", trcdb)
	broker.registerHost("trcdb-1", "trcdb", trcdb)
	request := &proxyRequestEnvelope{messageID: "trcdb-peer", targetPlugins: []string{"trcdb"}}

	if err := broker.enqueueToPod("1", request); err != nil {
		t.Fatalf("unexpected targeted delivery error: %v", err)
	}
	if len(broker.deliveryQueues[hubClientDeliveryKey("trcdb", "trcdb-0")]) != 0 {
		t.Fatal("unexpected delivery to a non-targeted pod")
	}
	delivered, err := broker.dequeue(context.Background(), "trcdb-1", "trcdb", trcdb)
	if err != nil || delivered.messageID != request.messageID {
		t.Fatalf("expected targeted pod to receive the request, request=%#v err=%v", delivered, err)
	}
}

func TestTargetedRequestKeepsHostWithActiveLongPoll(t *testing.T) {
	broker := newProxyRequestBroker()
	trcdb := map[string]struct{}{"trcdb": {}}
	deliveryKey := broker.beginHostPoll("trcdb-0", "trcdb", trcdb)
	if deliveryKey == "" {
		t.Fatal("expected valid poll metadata to register the host")
	}
	defer broker.endHostPoll(deliveryKey)

	host := broker.hosts[deliveryKey]
	host.lastSeen = time.Now().Add(-3 * hubClientRequestTimeout)
	broker.hosts[deliveryKey] = host

	request := &proxyRequestEnvelope{messageID: "trcdb-peer", targetPlugins: []string{"trcdb"}}
	if err := broker.enqueueToPod("0", request); err != nil {
		t.Fatalf("expected active long poll to remain targetable after idle TTL: %v", err)
	}
	if len(broker.deliveryQueues[deliveryKey]) != 1 {
		t.Fatal("expected targeted request to be queued for the active poll")
	}
}

func TestHubClientStatefulSetIdentityIsRecorded(t *testing.T) {
	contextWithMetadata := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		hubClientPodMetadataKey, "trcdb-west-2",
		hubClientStatefulSetMetadataKey, "trcdb-west",
	))
	broker := newProxyRequestBroker()
	trcdb := map[string]struct{}{"trcdb": {}}
	podID := hubClientPodFromIncomingContext(contextWithMetadata)
	statefulSetID := hubClientStatefulSetFromIncomingContext(contextWithMetadata)
	broker.registerHost(podID, statefulSetID, trcdb)

	host, ok := broker.hosts[hubClientDeliveryKey("trcdb-west", "trcdb-west-2")]
	if !ok || host.statefulSetID != "trcdb-west" {
		t.Fatalf("expected StatefulSet identity to be retained, host=%#v", host)
	}
}

func TestIsHubClientRequestRequiresCompleteRegistrationMetadata(t *testing.T) {
	if IsHubClientRequest(context.Background()) {
		t.Fatal("request without metadata identified as a hub client")
	}
	incompleteContext := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		hubClientPodMetadataKey, "trcdb-0",
		hubClientStatefulSetMetadataKey, "trcdb",
	))
	if IsHubClientRequest(incompleteContext) {
		t.Fatal("request without supported deployments identified as a hub client")
	}
	hubClientContext := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		hubClientPodMetadataKey, "trcdb-0",
		hubClientStatefulSetMetadataKey, "trcdb",
		hubClientPluginsMetadataKey, "trcdb",
	))
	if !IsHubClientRequest(hubClientContext) {
		t.Fatal("complete hub client metadata rejected")
	}
}

func TestDeliveryQueuesAreUniqueAcrossStatefulSets(t *testing.T) {
	broker := newProxyRequestBroker()
	trcdb := map[string]struct{}{"trcdb": {}}
	broker.registerHost("trcdb-0", "trcdb-east", trcdb)
	broker.registerHost("trcdb-0", "trcdb-west", trcdb)

	eastKey := hubClientDeliveryKey("trcdb-east", "trcdb-0")
	westKey := hubClientDeliveryKey("trcdb-west", "trcdb-0")
	broker.enqueueDirective("trcdb-log-state", &proxyRequestEnvelope{messageID: "fanout", targetPlugins: []string{"trcdb"}})
	if len(broker.deliveryQueues[eastKey]) != 1 || len(broker.deliveryQueues[westKey]) != 1 {
		t.Fatal("expected distinct delivery queues for pods in different StatefulSets")
	}
}

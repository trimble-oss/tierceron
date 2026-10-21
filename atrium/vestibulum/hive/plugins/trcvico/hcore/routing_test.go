package hcore

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestDefaultRouterTrainingFailsClosedWithoutTargetContracts(t *testing.T) {
	router := NewRouter(nil, nil)
	if err := router.Train(); err != nil {
		t.Fatalf("Train() error = %v", err)
	}
	if turns := router.BuildTurns("inspect worker"); len(turns) != 2 || turns[0].Role != "system" || turns[1].Content != "inspect worker" {
		t.Fatalf("unexpected default turns: %#v", turns)
	}
	plan, err := router.ParsePlan(`{"target":"worker","transport":"trcshtalk_diagnostic","supported":true}`)
	if err != nil {
		t.Fatalf("ParsePlan() error = %v", err)
	}
	if plan.Supported {
		t.Fatalf("default router accepted undeclared target: %#v", plan)
	}
}

func TestDefaultRouterResolvesExplicitLoggingDirectives(t *testing.T) {
	router := NewRouter(nil, nil)
	encoded, ok := router.ResolveLoggingPrompt("turn off logging for worker")
	if !ok {
		t.Fatal("expected explicit logging directive to resolve")
	}
	plan, err := router.ParsePlan(encoded)
	if err != nil {
		t.Fatalf("ParsePlan() error = %v", err)
	}
	if !plan.Supported || plan.Target != "worker" || plan.Transport != "logging_directive" || plan.LoggingAction != "stop" {
		t.Fatalf("unexpected logging plan: %#v", plan)
	}
	if _, ok := router.ResolveLoggingPrompt("turn off logging"); ok {
		t.Fatal("directive without explicit target resolved")
	}
}

func TestDefaultRouterCanonicalizesLoggingStatusAndFallback(t *testing.T) {
	router := NewRouter(nil, nil)
	plan, err := router.ParsePlan(`{"target":"worker","support_status":"off","diagnostic":"WORKER","supported":false}`)
	if err != nil {
		t.Fatalf("ParsePlan() error = %v", err)
	}
	if !plan.Supported || plan.Transport != "logging_directive" || plan.LoggingAction != "stop" || plan.Diagnostic != "" {
		t.Fatalf("unexpected canonical logging plan: %#v", plan)
	}
	normalized, err := router.NormalizeResponse("not json")
	if err != nil {
		t.Fatalf("NormalizeResponse() error = %v", err)
	}
	if normalized == "" {
		t.Fatal("fallback response was empty")
	}
}

func TestDefaultRouteAuthorizerRequiresHubClientForLogging(t *testing.T) {
	routingResults, ok := defaultRouter.ResolveLoggingPrompt("turn off logging for worker")
	if !ok {
		t.Fatal("expected deterministic logging directive")
	}
	if err := defaultLocalModelRouteAuthorizer(context.Background(), routingResults); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("error = %v, want permission denied", err)
	}
	hubClientContext := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		hubClientPodMetadataKey, "worker-0",
		hubClientStatefulSetMetadataKey, "worker",
		hubClientPluginsMetadataKey, "worker",
	))
	if err := defaultLocalModelRouteAuthorizer(hubClientContext, routingResults); err != nil {
		t.Fatalf("hub client logging directive rejected: %v", err)
	}
}

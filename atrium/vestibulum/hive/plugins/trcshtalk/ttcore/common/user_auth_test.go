package common

import (
	"context"
	"errors"
	"testing"

	"github.com/trimble-oss/tierceron-core/v2/identity"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestUserAndWorkloadAuthenticationAreIndependent(t *testing.T) {
	for _, scenario := range []struct {
		name          string
		tokens        []string
		userError     error
		code          codes.Code
		workloadCalls int
	}{
		{"workload", nil, nil, codes.OK, 1},
		{"user", []string{"oidc-id-token"}, nil, codes.OK, 0},
		{"invalid user cannot fall back", []string{"oidc-id-token"}, errors.New("denied"), codes.Unauthenticated, 0},
		{"expired user cannot fall back", []string{"oidc-id-token"}, identity.ErrOIDCIDTokenExpired, codes.Unauthenticated, 0},
		{"duplicate tokens", []string{"oidc-id-token", "oidc-id-token"}, nil, codes.Unauthenticated, 0},
		{"empty token", []string{""}, nil, codes.Unauthenticated, 0},
		{"missing token value", []string{}, nil, codes.Unauthenticated, 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			calls := 0
			md := metadata.MD{}
			if scenario.tokens != nil {
				md[identity.OIDCIDTokenMetadataKey] = scenario.tokens
			}
			ctx, err := AuthenticateIncoming(metadata.NewIncomingContext(context.Background(), md),
				func(context.Context) error { calls++; return nil },
				func(context.Context, string) error { return scenario.userError },
			)
			if status.Code(err) != scenario.code || calls != scenario.workloadCalls {
				t.Fatalf("error=%v workload calls=%d", err, calls)
			}
			if err == nil && identity.HasElevatedUserAuthorization(ctx) != (scenario.tokens != nil) {
				t.Fatal("incorrect server authorization")
			}
			if OIDCIDTokenExpired(err) != errors.Is(scenario.userError, identity.ErrOIDCIDTokenExpired) {
				t.Fatal("incorrect expiration signal")
			}
		})
	}
}

func TestMissingUserValidatorDoesNotUseWorkloadCredentials(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(identity.OIDCIDTokenMetadataKey, "oidc-id-token"))
	_, err := AuthenticateIncoming(ctx, func(context.Context) error {
		t.Fatal("user credentials fell back to workload authentication")
		return nil
	}, nil)
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("error=%v", err)
	}
}

func TestOutgoingOIDCIDTokenUsesOneMetadataKey(t *testing.T) {
	md, ok := metadata.FromOutgoingContext(withOIDCIDToken(context.Background(), "oidc-id-token"))
	if !ok || len(md) != 1 || len(md.Get("x-trc-oidc-id-token")) != 1 || md.Get("x-trc-oidc-id-token")[0] != "oidc-id-token" {
		t.Fatalf("unexpected outgoing user metadata: %v", md)
	}
}

func TestOIDCIDTokenExpirationReason(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(identity.OIDCIDTokenMetadataKey, "oidc-id-token"))
	_, err := AuthenticateIncoming(ctx, nil, func(context.Context, string) error {
		return identity.ErrOIDCIDTokenExpired
	})
	if status.Code(err) != codes.Unauthenticated || !OIDCIDTokenExpired(err) {
		t.Fatalf("expired credential was not recognized: %v", err)
	}
	for _, detail := range status.Convert(err).Details() {
		if info, ok := detail.(*errdetails.ErrorInfo); ok && info.Domain == "trcshtalk" && info.Reason == "OIDC_ID_TOKEN_EXPIRED" {
			return
		}
	}
	t.Fatal("credential did not receive the oidc-id-token expiration reason")
}

func TestSendHubCommandReauthenticatesOnlyExpirationOnce(t *testing.T) {
	preserveHubCommandHooks(t)
	for _, scenario := range []struct {
		name       string
		expired    bool
		retryError bool
	}{
		{"expired", true, false},
		{"invalid", false, false},
		{"still expired after retry", true, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			calls, refreshes := 0, 0
			SetOIDCIDTokenProvider(func(_ context.Context, force bool) (string, error) {
				if force {
					refreshes++
					return "new-token", nil
				}
				return "old-token", nil
			})
			SetHubCommandClient(func(ctx context.Context, id, prompt string) (string, error) {
				calls++
				md, _ := metadata.FromOutgoingContext(ctx)
				want := "old-token"
				if calls == 2 {
					want = "new-token"
				}
				if id != "original-id" || prompt != "run rainier" || len(md.Get(identity.OIDCIDTokenMetadataKey)) != 1 || md.Get(identity.OIDCIDTokenMetadataKey)[0] != want {
					t.Fatal("retry changed query, correlation, or credentials")
				}
				if !scenario.expired {
					return "", status.Error(codes.Unauthenticated, "invalid OIDC ID token")
				}
				if calls == 1 || scenario.retryError {
					_, err := AuthenticateIncoming(metadata.NewIncomingContext(context.Background(), metadata.Pairs(identity.OIDCIDTokenMetadataKey, want)),
						func(context.Context) error { t.Fatal("unexpected workload fallback"); return nil },
						func(context.Context, string) error { return identity.ErrOIDCIDTokenExpired },
					)
					return "", err
				}
				return "done", nil
			})
			result, err := SendHubCommand(context.Background(), "original-id", "run rainier")
			wantCalls := 1
			if scenario.expired {
				wantCalls = 2
			}
			if calls != wantCalls || refreshes != wantCalls-1 || (err == nil) != (scenario.expired && !scenario.retryError) {
				t.Fatalf("result=%q error=%v calls=%d refreshes=%d", result, err, calls, refreshes)
			}
		})
	}
}

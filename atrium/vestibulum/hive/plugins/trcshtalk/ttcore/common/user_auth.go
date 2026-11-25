package common

import (
	"context"
	"errors"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron-core/v2/identity"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const oidcIDTokenExpiredReason = "OIDC_ID_TOKEN_EXPIRED"

var oidcIDTokenProvider func(context.Context, bool) (string, error)

func SetOIDCIDTokenProvider(provider func(context.Context, bool) (string, error)) {
	hubCommandMu.Lock()
	oidcIDTokenProvider = provider
	hubCommandMu.Unlock()
}

// A successful oidcIDTokenValidator must verify elevated access, not just authenticate.
func AuthenticateIncoming(ctx context.Context, workload func(context.Context) error, oidcIDTokenValidator func(context.Context, string) error) (context.Context, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	tokens, hasOIDCIDToken := md[identity.OIDCIDTokenMetadataKey]
	if hasOIDCIDToken {
		if len(tokens) != 1 || tokens[0] == "" {
			return nil, status.Error(codes.Unauthenticated, "invalid user credentials")
		}
		if oidcIDTokenValidator == nil {
			return nil, status.Error(codes.Unavailable, "user authentication is not configured")
		}
		if err := oidcIDTokenValidator(ctx, tokens[0]); err != nil {
			if errors.Is(err, identity.ErrOIDCIDTokenExpired) {
				expired, detailErr := status.New(codes.Unauthenticated, "OIDC ID token expired").WithDetails(
					&errdetails.ErrorInfo{Reason: oidcIDTokenExpiredReason, Domain: "trcshtalk"},
				)
				if detailErr != nil {
					return nil, status.Error(codes.Internal, "unable to report OIDC ID token expiration")
				}
				return nil, expired.Err()
			}
			if errors.Is(err, identity.ErrUserAuthUnavailable) {
				return nil, status.Error(codes.Unavailable, "user authentication service unavailable")
			}
			return nil, status.Error(codes.Unauthenticated, "user authentication failed")
		}
		return identity.WithElevatedUserAuthorization(ctx), nil
	}
	if workload == nil {
		return nil, status.Error(codes.Unavailable, "workload authentication is not configured")
	}
	if err := workload(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid talkback token")
	}
	return ctx, nil
}

func OIDCIDTokenExpired(err error) bool {
	if status.Code(err) != codes.Unauthenticated {
		return false
	}
	for _, detail := range status.Convert(err).Details() {
		if info, ok := detail.(*errdetails.ErrorInfo); ok && info.Domain == "trcshtalk" && info.Reason == oidcIDTokenExpiredReason {
			return true
		}
	}
	return false
}

func RequireElevatedUserAuthorization(ctx context.Context) error {
	if !identity.HasElevatedUserAuthorization(ctx) {
		return status.Error(codes.PermissionDenied, "elevated queries require authorized user authentication")
	}
	return nil
}

func ConfiguredOIDCIDTokenValidator(config *tccore.ConfigContext) func(context.Context, string) error {
	if config == nil || config.Config == nil {
		return nil
	}
	validator, _ := identity.OIDCIDTokenValidatorFromProperties(*config.Config)
	return validator
}

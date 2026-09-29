package common

import (
	"context"
	"errors"
	"strings"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"google.golang.org/grpc/metadata"
)

const (
	SupportedDeploymentsMetadataKey = "x-trc-supported-deployments"
	TTBTokenMetadataKey             = "authorization"
)

func withOutgoingMetadata(ctx context.Context, key string, value string) context.Context {
	value = strings.TrimSpace(value)
	if value == "" {
		return ctx
	}
	md, _ := metadata.FromOutgoingContext(ctx)
	md = md.Copy()
	md.Set(key, value)
	return metadata.NewOutgoingContext(ctx, md)
}

func SupportedDeploymentsCSV(ctx *tccore.ConfigContext) string {
	if ctx == nil || ctx.Config == nil {
		return ""
	}
	if deployments, ok := (*ctx.Config)["deployments"].(string); ok {
		return deployments
	}
	return ""
}

func WithSupportedDeploymentsOutgoingContext(ctx context.Context, deployments string) context.Context {
	return withOutgoingMetadata(ctx, SupportedDeploymentsMetadataKey, deployments)
}

func WithTTBTokenOutgoingContext(ctx context.Context, token string) context.Context {
	return withOutgoingMetadata(ctx, TTBTokenMetadataKey, token)
}

func ExpectedTTBToken(ctx *tccore.ConfigContext) string {
	if ctx == nil || ctx.Config == nil {
		return ""
	}
	if token, ok := (*ctx.Config)[CfgTTBToken].(string); ok {
		return strings.TrimSpace(token)
	}
	return ""
}

func IncomingTTBToken(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	for _, value := range md.Get(TTBTokenMetadataKey) {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func ValidateIncomingTTBToken(configCtx *tccore.ConfigContext, incomingCtx context.Context) error {
	if configCtx == nil {
		return errors.New("missing config context")
	}
	mode := resolveTrcshTalkMode(configCtx.Config)
	if mode != ModeHub {
		return nil
	}
	expectedToken := ExpectedTTBToken(configCtx)
	if expectedToken == "" {
		return errors.New("missing configured ttb_token")
	}
	providedToken := IncomingTTBToken(incomingCtx)
	if providedToken == "" {
		return errors.New("missing talkback token")
	}
	if providedToken != expectedToken {
		return errors.New("invalid talkback token")
	}
	return nil
}

func SupportedDeploymentsSet(ctx *tccore.ConfigContext) map[string]struct{} {
	return SupportedDeploymentsMap(SupportedDeploymentsCSV(ctx))
}

func SupportedDeploymentsFromIncomingContext(ctx context.Context) map[string]struct{} {
	if ctx == nil {
		return nil
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil
	}
	values := md.Get(SupportedDeploymentsMetadataKey)
	if len(values) == 0 {
		return nil
	}
	deployments := map[string]struct{}{}
	for _, value := range values {
		for deployment := range SupportedDeploymentsMap(value) {
			deployments[deployment] = struct{}{}
		}
	}
	if len(deployments) == 0 {
		return nil
	}
	return deployments
}

func SupportedDeploymentsMap(deployments string) map[string]struct{} {
	if deployments == "" {
		return nil
	}
	deploymentSet := map[string]struct{}{}
	for _, deployment := range strings.Split(deployments, ",") {
		deployment = strings.TrimSpace(deployment)
		if deployment != "" {
			deploymentSet[deployment] = struct{}{}
		}
	}
	if len(deploymentSet) == 0 {
		return nil
	}
	return deploymentSet
}

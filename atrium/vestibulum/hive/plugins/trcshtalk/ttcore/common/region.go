package common

import (
	"context"
	"strings"
)

const HubClientRegionMetadataKey = "x-trc-region"

type targetRegionKey struct{}

func WithTargetRegion(ctx context.Context, region string) context.Context {
	return context.WithValue(ctx, targetRegionKey{}, region)
}

func TargetRegion(ctx context.Context) string {
	region, _ := ctx.Value(targetRegionKey{}).(string)
	return region
}

func NormalizeRegion(region string) string {
	return strings.ToLower(strings.TrimSpace(region))
}

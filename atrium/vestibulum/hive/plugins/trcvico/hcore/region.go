package hcore

import (
	"context"

	trcshtalkcommon "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcshtalk/ttcore/common"
)

var localRegionNormalizer = trcshtalkcommon.NormalizeRegion

func SetLocalRegionNormalizer(normalizer func(string) string) {
	if normalizer == nil {
		localRegionNormalizer = trcshtalkcommon.NormalizeRegion
		return
	}
	localRegionNormalizer = normalizer
}

func localDiagnosticHandler(ctx context.Context, targets []string) func(context.Context, string, string) (string, error) {
	region := trcshtalkcommon.TargetRegion(ctx)
	if region != "" && (configContext == nil || localRegionNormalizer(configContext.Region) != region) {
		return nil
	}
	return trcshtalkcommon.LocalDiagnosticHandler(targets)
}

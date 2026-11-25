package services

import (
	"context"
	"log/slog"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/registry"
)

// BackfillLegacyPrices rewrites the commodities stored before a price could
// be unknown, so that a zero current or converted price that meant "not set"
// reads as unknown instead of as a known zero (see
// registry.CommodityRegistry.BackfillLegacyPrices). It reports how many rows
// it rewrote. Rows written since are never touched, so running it again, or
// on several replicas at once, changes nothing.
func BackfillLegacyPrices(ctx context.Context, factorySet *registry.FactorySet) (int, error) {
	if factorySet == nil || factorySet.CommodityRegistryFactory == nil {
		return 0, nil
	}
	reg := factorySet.CommodityRegistryFactory.CreateServiceRegistry()
	rewritten, err := reg.BackfillLegacyPrices(ctx)
	if err != nil {
		return rewritten, errxtrace.Wrap("failed to backfill legacy prices", err)
	}
	return rewritten, nil
}

// RunLegacyPriceBackfill runs BackfillLegacyPrices and retries it every
// retryInterval until a pass succeeds or ctx ends.
func RunLegacyPriceBackfill(ctx context.Context, factorySet *registry.FactorySet, retryInterval time.Duration) {
	for {
		start := time.Now()
		rewritten, err := BackfillLegacyPrices(ctx, factorySet)
		if err == nil {
			if rewritten > 0 {
				slog.Info("legacy prices backfilled",
					"commodities", rewritten, "duration_ms", time.Since(start).Milliseconds())
			}
			return
		}
		if ctx.Err() != nil {
			return
		}
		slog.Warn("legacy price backfill failed; will retry",
			"error", err.Error(), "commodities", rewritten, "retry_in", retryInterval.String())
		if !sleepCtx(ctx, retryInterval) {
			return
		}
	}
}

package services

import (
	"context"
	"log/slog"
	"time"

	"go.5x5.cz/inventario/registry"
)

// BackfillRegisteredDates gives the day it was added to every commodity
// stored without one whose history records its creation (see
// registry.CommodityRegistry.BackfillRegisteredDates). Before the registries
// dated new items, only seed data carried the date, so items added through
// the app showed no "Date added" and sorted last by it. It runs once at boot
// through the service registry and runs another pass every retryInterval
// while a pass fails or leaves rows another transaction held locked, until
// none is left or ctx ends. The pass is idempotent, so several replicas
// running it at once change nothing twice.
func BackfillRegisteredDates(ctx context.Context, factorySet *registry.FactorySet, retryInterval time.Duration) {
	if factorySet == nil || factorySet.CommodityRegistryFactory == nil {
		return
	}
	reg := factorySet.CommodityRegistryFactory.CreateServiceRegistry()

	start := time.Now()
	total := 0
	for {
		dated, pending, err := reg.BackfillRegisteredDates(ctx)
		total += dated
		switch {
		case err == nil && !pending:
			if total > 0 {
				slog.InfoContext(ctx, "registered date backfill complete",
					"commodities", total, "duration_ms", time.Since(start).Milliseconds())
			}
			return
		case ctx.Err() != nil:
			return
		case err == nil:
			slog.InfoContext(ctx, "registered date backfill skipped locked commodities; will retry",
				"commodities", dated, "retry_in", retryInterval.String())
		default:
			slog.WarnContext(ctx, "registered date backfill failed; will retry",
				"error", err.Error(), "commodities", dated, "retry_in", retryInterval.String())
		}
		if !sleepCtx(ctx, retryInterval) {
			return
		}
	}
}

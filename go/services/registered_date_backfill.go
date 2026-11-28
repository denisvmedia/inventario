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
// through the service registry and retries every retryInterval until a pass
// succeeds or ctx ends. The pass is idempotent, so several replicas running
// it at once change nothing twice.
func BackfillRegisteredDates(ctx context.Context, factorySet *registry.FactorySet, retryInterval time.Duration) {
	if factorySet == nil || factorySet.CommodityRegistryFactory == nil {
		return
	}
	reg := factorySet.CommodityRegistryFactory.CreateServiceRegistry()

	for {
		start := time.Now()
		dated, err := reg.BackfillRegisteredDates(ctx)
		if err == nil {
			if dated > 0 {
				slog.InfoContext(ctx, "registered date backfill complete",
					"commodities", dated, "duration_ms", time.Since(start).Milliseconds())
			}
			return
		}
		if ctx.Err() != nil {
			return
		}
		slog.WarnContext(ctx, "registered date backfill failed; will retry",
			"error", err.Error(), "commodities", dated, "retry_in", retryInterval.String())
		if !sleepCtx(ctx, retryInterval) {
			return
		}
	}
}

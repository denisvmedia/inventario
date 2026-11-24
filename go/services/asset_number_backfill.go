package services

import (
	"context"
	"log/slog"
	"time"

	"go.5x5.cz/inventario/registry"
)

// BackfillAssetNumbers gives an asset number to every commodity that has
// none: rows created before the asset_number column existed. It runs once at
// boot through the service registry and retries every retryInterval until a
// pass succeeds or ctx ends, so a database that is briefly unavailable at
// startup does not leave items unnumbered until the next restart.
//
// The registry pass is idempotent and takes the same per-group lock as
// commodity creation, so several replicas running it at once, or users
// creating items meanwhile, cannot produce two items with one number.
func BackfillAssetNumbers(ctx context.Context, factorySet *registry.FactorySet, retryInterval time.Duration) {
	if factorySet == nil || factorySet.CommodityRegistryFactory == nil {
		return
	}
	reg := factorySet.CommodityRegistryFactory.CreateServiceRegistry()

	for {
		start := time.Now()
		numbered, err := reg.BackfillAssetNumbers(ctx)
		if err == nil {
			if numbered > 0 {
				slog.Info("asset number backfill complete",
					"numbered", numbered,
					"duration_ms", time.Since(start).Milliseconds())
			}
			return
		}
		if ctx.Err() != nil {
			return
		}
		slog.Warn("asset number backfill failed; will retry",
			"error", err.Error(), "numbered", numbered, "retry_in", retryInterval.String())

		timer := time.NewTimer(retryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

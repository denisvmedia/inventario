package services

import (
	"context"
	"log/slog"
	"time"

	"go.5x5.cz/inventario/registry"
)

// BackfillItemTypes seeds the built-in item types into every group that has
// none: the groups created before item types were a per-group list (#2802).
// It runs once at boot through the service registry and retries every
// retryInterval until a pass succeeds or ctx ends.
//
// The pass is idempotent and takes the same per-group lock as the lazy
// seeding a request does on first use, so several replicas running it at
// once, or a request seeding the same group meanwhile, cannot seed a group
// twice. A group a request reaches first is seeded by the request.
func BackfillItemTypes(ctx context.Context, factorySet *registry.FactorySet, retryInterval time.Duration) {
	if factorySet == nil || factorySet.ItemTypeRegistryFactory == nil {
		return
	}
	reg := factorySet.ItemTypeRegistryFactory.CreateServiceRegistry()

	for {
		start := time.Now()
		seeded, err := reg.BackfillBuiltins(ctx)
		if err == nil {
			if seeded > 0 {
				slog.Info("item type backfill complete",
					"item_types", seeded,
					"duration_ms", time.Since(start).Milliseconds())
			}
			return
		}
		if ctx.Err() != nil {
			return
		}
		slog.Warn("item type backfill failed; will retry",
			"error", err.Error(), "item_types", seeded, "retry_in", retryInterval.String())

		timer := time.NewTimer(retryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

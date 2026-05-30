package metrics

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// BusinessStats is a point-in-time snapshot of installation-wide
// entity counts and storage usage. It is produced by a
// BusinessStatsFunc and fed into the business gauges by
// BusinessCollector.
type BusinessStats struct {
	Tenants        int64
	Users          int64
	LocationGroups int64
	Locations      int64
	Areas          int64
	Commodities    int64
	Files          int64

	StorageImages    int64
	StorageDocuments int64
	StorageOther     int64
	StorageExports   int64
}

// BusinessStatsFunc returns a fresh BusinessStats snapshot. It is the
// only seam through which this leaf package reaches the database, so
// the registry/service code that knows how to count rows stays out of
// the metrics package's import graph.
type BusinessStatsFunc func(ctx context.Context) (BusinessStats, error)

// defaultBusinessCollectInterval is a sane fallback cadence; the
// caller normally passes an explicit interval.
const defaultBusinessCollectInterval = time.Minute

// BusinessCollector periodically calls a BusinessStatsFunc and pushes
// the result into the business gauges. It owns a single goroutine,
// mirroring the lifecycle of the storage-quota reminder worker:
// run-once-immediately, then tick, with a best-effort graceful stop.
type BusinessCollector struct {
	source   BusinessStatsFunc
	interval time.Duration
	logger   *slog.Logger

	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// NewBusinessCollector constructs a BusinessCollector. A non-positive
// interval falls back to defaultBusinessCollectInterval.
func NewBusinessCollector(source BusinessStatsFunc, interval time.Duration) *BusinessCollector {
	if interval <= 0 {
		interval = defaultBusinessCollectInterval
	}
	return &BusinessCollector{
		source:   source,
		interval: interval,
		logger:   slog.Default(),
		stopCh:   make(chan struct{}),
	}
}

// Start launches the collection goroutine. It collects once
// immediately so the gauges are populated right after a deploy rather
// than after the first full interval, then ticks every interval. The
// goroutine exits on ctx cancellation or Stop. No-op if no source is
// configured.
func (c *BusinessCollector) Start(ctx context.Context) {
	if c.source == nil {
		c.logger.Warn("BusinessCollector: no source configured, skipping startup")
		return
	}
	c.wg.Go(func() {
		c.run(ctx)
	})
	c.logger.Info("Business metrics collector started", "interval", c.interval)
}

// Stop signals the goroutine and waits for it to exit. Safe to call
// multiple times.
func (c *BusinessCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	c.wg.Wait()
	c.logger.Info("Business metrics collector stopped")
}

func (c *BusinessCollector) run(ctx context.Context) {
	// Collect once at startup; same rationale as the reminder workers.
	c.collectOnce(ctx)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.collectOnce(ctx)
		}
	}
}

// collectOnce runs a single collection sweep. On error it records the
// failure and returns WITHOUT touching the gauges, so a transient DB
// blip leaves the last good values in place rather than zeroing them.
func (c *BusinessCollector) collectOnce(ctx context.Context) {
	start := time.Now()
	defer func() {
		businessCollectDuration.Observe(time.Since(start).Seconds())
	}()

	stats, err := c.source(ctx)
	if err != nil {
		businessCollectErrorsTotal.Inc()
		c.logger.Warn("Business metrics collection failed", "error", err)
		return
	}

	businessTenants.Set(float64(stats.Tenants))
	businessUsers.Set(float64(stats.Users))
	businessLocationGroups.Set(float64(stats.LocationGroups))
	businessLocations.Set(float64(stats.Locations))
	businessAreas.Set(float64(stats.Areas))
	businessCommodities.Set(float64(stats.Commodities))
	businessFiles.Set(float64(stats.Files))

	businessFileStorageBytes.WithLabelValues(storageCategoryImages).Set(float64(stats.StorageImages))
	businessFileStorageBytes.WithLabelValues(storageCategoryDocuments).Set(float64(stats.StorageDocuments))
	businessFileStorageBytes.WithLabelValues(storageCategoryOther).Set(float64(stats.StorageOther))
	businessFileStorageBytes.WithLabelValues(storageCategoryExports).Set(float64(stats.StorageExports))
}

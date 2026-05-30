package metrics

import (
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// PoolStatProvider is the slice of *pgxpool.Pool the PoolCollector
// needs: a way to snapshot pool statistics on demand.
type PoolStatProvider interface {
	Stat() *pgxpool.Stat
}

// compile-time assertion that *pgxpool.Pool satisfies PoolStatProvider.
var _ PoolStatProvider = (*pgxpool.Pool)(nil)

// Descriptors for the const metrics the PoolCollector emits. Defined
// as package vars so every collector instance shares one set.
var (
	poolConnectionsDesc = prometheus.NewDesc(
		"inventario_db_pool_connections",
		"Current number of pool connections, partitioned by state.",
		[]string{"state"}, nil,
	)
	poolMaxConnectionsDesc = prometheus.NewDesc(
		"inventario_db_pool_max_connections",
		"Maximum number of connections the pool is configured to allow.",
		nil, nil,
	)
	poolAcquireTotalDesc = prometheus.NewDesc(
		"inventario_db_pool_acquire_total",
		"Cumulative number of successful connection acquisitions.",
		nil, nil,
	)
	poolEmptyAcquireTotalDesc = prometheus.NewDesc(
		"inventario_db_pool_empty_acquire_total",
		"Cumulative number of acquisitions that had to wait for a connection because the pool was empty.",
		nil, nil,
	)
	poolCanceledAcquireTotalDesc = prometheus.NewDesc(
		"inventario_db_pool_canceled_acquire_total",
		"Cumulative number of acquisitions canceled by context before completing.",
		nil, nil,
	)
)

// PoolCollector is a prometheus.Collector that snapshots a pgx pool's
// statistics on each scrape and emits them as const metrics. Using
// const metrics (rather than promauto gauges) keeps the snapshot
// consistent and avoids a background sampling goroutine.
type PoolCollector struct {
	provider PoolStatProvider
}

// compile-time assertion that *PoolCollector satisfies the Collector
// interface.
var _ prometheus.Collector = (*PoolCollector)(nil)

// NewPoolCollector constructs a PoolCollector backed by the given
// stat provider (typically a *pgxpool.Pool).
func NewPoolCollector(p PoolStatProvider) *PoolCollector {
	return &PoolCollector{provider: p}
}

// Describe sends the descriptors of all metrics the collector may emit.
func (*PoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- poolConnectionsDesc
	ch <- poolMaxConnectionsDesc
	ch <- poolAcquireTotalDesc
	ch <- poolEmptyAcquireTotalDesc
	ch <- poolCanceledAcquireTotalDesc
}

// Collect snapshots the pool once and emits the const metrics.
func (c *PoolCollector) Collect(ch chan<- prometheus.Metric) {
	stat := c.provider.Stat()

	ch <- prometheus.MustNewConstMetric(poolConnectionsDesc, prometheus.GaugeValue,
		float64(stat.AcquiredConns()), "acquired")
	ch <- prometheus.MustNewConstMetric(poolConnectionsDesc, prometheus.GaugeValue,
		float64(stat.IdleConns()), "idle")
	ch <- prometheus.MustNewConstMetric(poolConnectionsDesc, prometheus.GaugeValue,
		float64(stat.ConstructingConns()), "constructing")

	ch <- prometheus.MustNewConstMetric(poolMaxConnectionsDesc, prometheus.GaugeValue,
		float64(stat.MaxConns()))

	ch <- prometheus.MustNewConstMetric(poolAcquireTotalDesc, prometheus.CounterValue,
		float64(stat.AcquireCount()))
	ch <- prometheus.MustNewConstMetric(poolEmptyAcquireTotalDesc, prometheus.CounterValue,
		float64(stat.EmptyAcquireCount()))
	ch <- prometheus.MustNewConstMetric(poolCanceledAcquireTotalDesc, prometheus.CounterValue,
		float64(stat.CanceledAcquireCount()))
}

// RegisterPoolCollector registers a PoolCollector for p against the
// default registerer and returns a function that unregisters it.
//
// An AlreadyRegisteredError (an identical collector is already
// registered, e.g. by a previous pool or a parallel test) is tolerated
// and yields a no-op unregister, so callers in postgres.go can blindly
// register on pool creation and unregister on cleanup without ever
// panicking.
//
// LIMITATION: all PoolCollectors share one set of descriptors, so the
// registry derives the same collector identity for every pool. Only the
// FIRST live pool's stats are exported; a SECOND concurrently-live pool
// (e.g. a future read-replica or a second registry set) collides and is
// dropped. That is correct for the single-pool deployment today, and the
// collision is logged (below) rather than swallowed silently so the gap
// is visible if it ever arises. Supporting multiple pools would require a
// distinguishing const label on the descriptors.
func RegisterPoolCollector(p PoolStatProvider) (unregister func()) {
	coll := NewPoolCollector(p)
	if err := prometheus.DefaultRegisterer.Register(coll); err != nil {
		var already prometheus.AlreadyRegisteredError
		if errors.As(err, &already) {
			// Identical collector already present — its stats, not this
			// pool's, will be exported. Surface it; see the LIMITATION note.
			slog.Warn("db pool metrics collector already registered; " +
				"this pool's stats will not be exported (multiple pools are not supported)")
			return func() {}
		}
		// Any other registration error is non-fatal for the caller's
		// happy path, but should not pass unnoticed.
		slog.Warn("failed to register db pool metrics collector", "error", err)
		return func() {}
	}
	return func() {
		prometheus.DefaultRegisterer.Unregister(coll)
	}
}

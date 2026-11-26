package metrics

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Background-job failure metrics (#1307). A job is dead-lettered when its
// worker gives up on it and marks it failed for good; the depth is how many
// such jobs are waiting for an operator to retry or discard them.
var (
	jobsDeadLetteredTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "inventario_jobs_dead_lettered_total",
		Help: "Total number of background jobs this process marked failed for good, partitioned by job type.",
	}, []string{"type"})

	jobsDeadLetterDepth = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "inventario_jobs_dead_letter_depth",
		Help: "Current number of failed background jobs that are neither retried nor discarded, partitioned by job type. Installation-wide.",
	}, []string{"type"})

	jobsDeadLetterCollectErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "inventario_jobs_dead_letter_collect_errors_total",
		Help: "Total number of failed-job depth collections that failed.",
	})
)

// InitJobTypes exports a zero-valued dead-letter counter series for each job
// type, so rate() has a baseline before the first failure. The type names
// come from the caller: this leaf package does not import the models.
func InitJobTypes(jobTypes ...string) {
	for _, t := range jobTypes {
		jobsDeadLetteredTotal.WithLabelValues(t).Add(0)
	}
}

// RecordJobDeadLettered counts one job of the type marked failed for good.
func RecordJobDeadLettered(jobType string) {
	jobsDeadLetteredTotal.WithLabelValues(jobType).Inc()
}

// JobDepthFunc returns the number of failed, undiscarded jobs per job type.
type JobDepthFunc func(ctx context.Context) (map[string]int, error)

// JobDepthCollector periodically reads the failed-job depth per type into
// the inventario_jobs_dead_letter_depth gauge. The counts span every
// tenant, so exactly one process should run it, as with BusinessCollector.
type JobDepthCollector struct {
	source   JobDepthFunc
	interval time.Duration
	logger   *slog.Logger

	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// NewJobDepthCollector constructs a JobDepthCollector. A non-positive
// interval falls back to one minute.
func NewJobDepthCollector(source JobDepthFunc, interval time.Duration) *JobDepthCollector {
	if interval <= 0 {
		interval = defaultBusinessCollectInterval
	}
	return &JobDepthCollector{
		source:   source,
		interval: interval,
		logger:   slog.Default(),
		stopCh:   make(chan struct{}),
	}
}

// Start collects once, then every interval, until ctx is done or Stop is
// called. No-op without a source.
func (c *JobDepthCollector) Start(ctx context.Context) {
	if c.source == nil {
		c.logger.Warn("JobDepthCollector: no source configured, skipping startup")
		return
	}
	c.wg.Go(func() {
		c.CollectOnce(ctx)
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.stopCh:
				return
			case <-ticker.C:
				c.CollectOnce(ctx)
			}
		}
	})
	c.logger.Info("Failed-job depth collector started", "interval", c.interval)
}

// Stop signals the goroutine and waits for it to exit. Safe to call more
// than once.
func (c *JobDepthCollector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	c.wg.Wait()
}

// CollectOnce reads the depths once. On error it keeps the last values, so a
// database blip does not report an empty queue.
func (c *JobDepthCollector) CollectOnce(ctx context.Context) {
	depths, err := c.source(ctx)
	if err != nil {
		jobsDeadLetterCollectErrorsTotal.Inc()
		c.logger.Warn("Failed-job depth collection failed", "error", err)
		return
	}
	for jobType, n := range depths {
		jobsDeadLetterDepth.WithLabelValues(jobType).Set(float64(n))
	}
}

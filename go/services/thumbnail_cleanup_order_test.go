package services

import (
	"context"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// cleanupOrderRecorder records, in call order, which cleanup step ran. It is
// shared by the slot- and job-registry decorators so the test can assert that
// performCleanup deletes expired concurrency slots BEFORE it deletes completed
// jobs (#2122 F5).
type cleanupOrderRecorder struct {
	mu    sync.Mutex
	order []string
}

func (r *cleanupOrderRecorder) record(step string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.order = append(r.order, step)
}

func (r *cleanupOrderRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// recordingSlotRegistry wraps a real UserConcurrencySlotRegistry and records
// when CleanupExpiredSlots runs, delegating everything else to the embedded
// registry.
type recordingSlotRegistry struct {
	registry.UserConcurrencySlotRegistry
	rec *cleanupOrderRecorder
}

func (r *recordingSlotRegistry) CleanupExpiredSlots(ctx context.Context) error {
	r.rec.record("slots")
	return r.UserConcurrencySlotRegistry.CleanupExpiredSlots(ctx)
}

// recordingSlotFactory decorates a UserConcurrencySlotRegistryFactory so every
// registry it hands out records its cleanup calls.
type recordingSlotFactory struct {
	inner registry.UserConcurrencySlotRegistryFactory
	rec   *cleanupOrderRecorder
}

func (f *recordingSlotFactory) CreateUserRegistry(ctx context.Context) (registry.UserConcurrencySlotRegistry, error) {
	r, err := f.inner.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}
	return &recordingSlotRegistry{UserConcurrencySlotRegistry: r, rec: f.rec}, nil
}

func (f *recordingSlotFactory) MustCreateUserRegistry(ctx context.Context) registry.UserConcurrencySlotRegistry {
	return &recordingSlotRegistry{UserConcurrencySlotRegistry: f.inner.MustCreateUserRegistry(ctx), rec: f.rec}
}

func (f *recordingSlotFactory) CreateServiceRegistry() registry.UserConcurrencySlotRegistry {
	return &recordingSlotRegistry{UserConcurrencySlotRegistry: f.inner.CreateServiceRegistry(), rec: f.rec}
}

// recordingJobRegistry wraps a real ThumbnailGenerationJobRegistry and records
// when CleanupCompletedJobs runs, delegating everything else to the embedded
// registry.
type recordingJobRegistry struct {
	registry.ThumbnailGenerationJobRegistry
	rec *cleanupOrderRecorder
}

func (r *recordingJobRegistry) CleanupCompletedJobs(ctx context.Context, olderThan time.Duration) error {
	r.rec.record("jobs")
	return r.ThumbnailGenerationJobRegistry.CleanupCompletedJobs(ctx, olderThan)
}

// recordingJobFactory decorates a ThumbnailGenerationJobRegistryFactory so
// every registry it hands out records its cleanup calls.
type recordingJobFactory struct {
	inner registry.ThumbnailGenerationJobRegistryFactory
	rec   *cleanupOrderRecorder
}

func (f *recordingJobFactory) CreateUserRegistry(ctx context.Context) (registry.ThumbnailGenerationJobRegistry, error) {
	r, err := f.inner.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}
	return &recordingJobRegistry{ThumbnailGenerationJobRegistry: r, rec: f.rec}, nil
}

func (f *recordingJobFactory) MustCreateUserRegistry(ctx context.Context) registry.ThumbnailGenerationJobRegistry {
	return &recordingJobRegistry{ThumbnailGenerationJobRegistry: f.inner.MustCreateUserRegistry(ctx), rec: f.rec}
}

func (f *recordingJobFactory) CreateServiceRegistry() registry.ThumbnailGenerationJobRegistry {
	return &recordingJobRegistry{ThumbnailGenerationJobRegistry: f.inner.CreateServiceRegistry(), rec: f.rec}
}

// TestThumbnailWorker_PerformCleanupOrder locks the #2122 F5 invariant:
// performCleanup must delete expired concurrency slots BEFORE it deletes
// completed jobs, so the NO ACTION user_concurrency_slots.job_id ->
// thumbnail_generation_jobs(id) FK is already broken before the job rows are
// removed. The two run in separate transactions, so the wrong order would let
// an orphan slot still reference a completed job and block its deletion.
func TestThumbnailWorker_PerformCleanupOrder(t *testing.T) {
	c := qt.New(t)

	rec := &cleanupOrderRecorder{}

	factorySet := memory.NewFactorySet()
	factorySet.UserConcurrencySlotRegistryFactory = &recordingSlotFactory{
		inner: factorySet.UserConcurrencySlotRegistryFactory,
		rec:   rec,
	}
	factorySet.ThumbnailGenerationJobRegistryFactory = &recordingJobFactory{
		inner: factorySet.ThumbnailGenerationJobRegistryFactory,
		rec:   rec,
	}

	config := ThumbnailGenerationConfig{
		MaxConcurrentPerUser: 5,
		RateLimitPerMinute:   50,
		SlotDuration:         5 * time.Minute,
	}
	worker := NewThumbnailGenerationWorker(factorySet, "memory://", config)

	worker.performCleanup(context.Background())

	c.Assert(rec.snapshot(), qt.DeepEquals, []string{"slots", "jobs"})
}

var (
	_ registry.UserConcurrencySlotRegistryFactory    = (*recordingSlotFactory)(nil)
	_ registry.ThumbnailGenerationJobRegistryFactory = (*recordingJobFactory)(nil)
)

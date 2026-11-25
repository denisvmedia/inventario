package services

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"go.5x5.cz/inventario/models"
)

// defaultTrashPurgeInterval is how often the purge worker looks for expired
// Trash entries.
const defaultTrashPurgeInterval = time.Hour

// trashPurgeBatch is how many items and how many files one round of a sweep
// deletes.
const trashPurgeBatch = 100

// TrashPurger deletes expired Trash entries; TrashService implements it.
type TrashPurger interface {
	PurgeExpired(ctx context.Context, cutoff time.Time, batch int) (items, files int, err error)
}

// TrashPurgeWorker deletes, across every group, the items and files that have
// been in Trash longer than the retention. Every step re-reads its state, so
// a sweep that stops halfway is finished by the next one.
type TrashPurgeWorker struct {
	purger    TrashPurger
	retention time.Duration
	interval  time.Duration
	pause     PauseChecker
	clock     func() time.Time
	stopCh    chan struct{}
	stopOnce  sync.Once
	wg        sync.WaitGroup
}

// TrashPurgeOption customizes a TrashPurgeWorker.
type TrashPurgeOption func(*TrashPurgeWorker)

// WithTrashRetention sets how long entries stay in Trash. Non-positive values
// are ignored.
func WithTrashRetention(d time.Duration) TrashPurgeOption {
	return func(w *TrashPurgeWorker) {
		if d > 0 {
			w.retention = d
		}
	}
}

// WithTrashPurgeInterval sets the sweep cadence. Non-positive values are
// ignored.
func WithTrashPurgeInterval(d time.Duration) TrashPurgeOption {
	return func(w *TrashPurgeWorker) {
		if d > 0 {
			w.interval = d
		}
	}
}

// WithTrashPurgeClock replaces the wall clock. Test-only.
func WithTrashPurgeClock(fn func() time.Time) TrashPurgeOption {
	return func(w *TrashPurgeWorker) {
		if fn != nil {
			w.clock = fn
		}
	}
}

// WithTrashPurgePauseController skips sweeps while the trash-purge worker
// type is paused. A nil checker leaves the worker unpaused.
func WithTrashPurgePauseController(pc PauseChecker) TrashPurgeOption {
	return func(w *TrashPurgeWorker) {
		if pc != nil {
			w.pause = pc
		}
	}
}

// NewTrashPurgeWorker builds the worker. A nil purger makes Start a no-op.
func NewTrashPurgeWorker(purger TrashPurger, opts ...TrashPurgeOption) *TrashPurgeWorker {
	w := &TrashPurgeWorker{
		purger:    purger,
		retention: DefaultTrashRetention,
		interval:  defaultTrashPurgeInterval,
		clock:     time.Now,
		stopCh:    make(chan struct{}),
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Start launches the background goroutine.
func (w *TrashPurgeWorker) Start(ctx context.Context) {
	if w.purger == nil {
		slog.Warn("TrashPurgeWorker: no purger configured, skipping startup")
		return
	}
	w.wg.Go(func() {
		w.run(ctx)
	})
	slog.Info("Trash purge worker started", "retention", w.retention, "interval", w.interval)
}

// Stop signals the worker to stop and waits for it to finish.
func (w *TrashPurgeWorker) Stop() {
	w.stopOnce.Do(func() { close(w.stopCh) })
	w.wg.Wait()
	slog.Info("Trash purge worker stopped")
}

func (w *TrashPurgeWorker) run(ctx context.Context) {
	w.Sweep(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.Sweep(ctx)
		}
	}
}

// Sweep deletes every entry that expired by now, in rounds of trashPurgeBatch.
// A round without errors that deletes less than a full batch is the last one.
// A round with errors logs them, and the sweep goes on while rounds still
// delete something, so one entry that keeps failing does not hold back the
// rest; it is retried on the next sweep.
func (w *TrashPurgeWorker) Sweep(ctx context.Context) {
	if w.pause != nil && w.pause.IsPaused(models.WorkerTypeTrashPurge) {
		return
	}
	cutoff := w.clock().Add(-w.retention)
	var totalItems, totalFiles int
	for {
		if ctx.Err() != nil {
			return
		}
		select {
		case <-w.stopCh:
			return
		default:
		}
		items, files, err := w.purger.PurgeExpired(ctx, cutoff, trashPurgeBatch)
		totalItems += items
		totalFiles += files
		if err != nil {
			slog.Error("Failed to purge expired Trash entries", "error", err, "cutoff", cutoff)
			if items+files == 0 {
				break
			}
			continue
		}
		if items < trashPurgeBatch && files < trashPurgeBatch {
			break
		}
	}
	if totalItems > 0 || totalFiles > 0 {
		slog.Info("Purged expired Trash entries", "items", totalItems, "files", totalFiles, "cutoff", cutoff)
	}
}

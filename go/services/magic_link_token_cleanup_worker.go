// MagicLinkTokenCleanupWorker mirrors EmailVerificationCleanupWorker by
// design — same Start/Stop/runCleanup/cleanupOnce lifecycle and the same
// soft-pause skip (#1308). Each worker still owns its own registry type,
// worker-type pause key, and log wording, so a shared generic base would
// erase those per-worker specifics for negligible LOC savings.
//
//nolint:dupl // intentional symmetry with the email-verification cleanup worker
package services

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

const defaultMagicLinkTokenCleanupInterval = 1 * time.Hour

// MagicLinkTokenCleanupWorker periodically deletes expired magic-link
// sign-in tokens from the registry. For PostgreSQL this keeps the
// magic_link_tokens table from growing unbounded; for the in-memory
// registry it reclaims memory during long-running test servers.
type MagicLinkTokenCleanupWorker struct {
	registry        registry.MagicLinkTokenRegistry
	cleanupInterval time.Duration
	pause           PauseChecker
	stopCh          chan struct{}
	stopOnce        sync.Once
	wg              sync.WaitGroup
}

// MagicLinkTokenCleanupOption customizes a MagicLinkTokenCleanupWorker created by NewMagicLinkTokenCleanupWorker.
type MagicLinkTokenCleanupOption func(*magicLinkTokenCleanupOptions)

type magicLinkTokenCleanupOptions struct {
	cleanupInterval time.Duration
	pause           PauseChecker
}

// WithMagicLinkTokenCleanupInterval overrides the default cleanup interval.
// Non-positive values are ignored.
func WithMagicLinkTokenCleanupInterval(d time.Duration) MagicLinkTokenCleanupOption {
	return func(o *magicLinkTokenCleanupOptions) {
		if d > 0 {
			o.cleanupInterval = d
		}
	}
}

// WithMagicLinkTokenCleanupPauseController wires the soft-pause controller
// so the worker skips its cleanup while the magic-link-token-cleanup worker
// type is paused (#1308). A nil checker leaves the worker unpaused.
func WithMagicLinkTokenCleanupPauseController(pc PauseChecker) MagicLinkTokenCleanupOption {
	return func(o *magicLinkTokenCleanupOptions) {
		if pc != nil {
			o.pause = pc
		}
	}
}

// NewMagicLinkTokenCleanupWorker creates a cleanup worker with the default one-hour interval,
// overridable via MagicLinkTokenCleanupOption values (e.g., WithMagicLinkTokenCleanupInterval).
func NewMagicLinkTokenCleanupWorker(r registry.MagicLinkTokenRegistry, opts ...MagicLinkTokenCleanupOption) *MagicLinkTokenCleanupWorker {
	options := magicLinkTokenCleanupOptions{
		cleanupInterval: defaultMagicLinkTokenCleanupInterval,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return &MagicLinkTokenCleanupWorker{
		registry:        r,
		cleanupInterval: options.cleanupInterval,
		pause:           options.pause,
		stopCh:          make(chan struct{}),
	}
}

// Start launches the background cleanup goroutine. It is a no-op if the registry is nil.
func (w *MagicLinkTokenCleanupWorker) Start(ctx context.Context) {
	if w.registry == nil {
		slog.Warn("MagicLinkTokenCleanupWorker: no registry configured, skipping startup")
		return
	}
	w.wg.Go(func() {
		w.runCleanup(ctx)
	})
	slog.Info("Magic-link token cleanup worker started", "interval", w.cleanupInterval)
}

// Stop signals the worker to stop and waits for it to finish.
func (w *MagicLinkTokenCleanupWorker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.wg.Wait()
	slog.Info("Magic-link token cleanup worker stopped")
}

func (w *MagicLinkTokenCleanupWorker) runCleanup(ctx context.Context) {
	ticker := time.NewTicker(w.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.cleanupOnce(ctx)
		}
	}
}

// cleanupOnce runs a single expired-token sweep.
func (w *MagicLinkTokenCleanupWorker) cleanupOnce(ctx context.Context) {
	// Soft-pause (#1308): skip the cleanup while paused. The ticker keeps
	// running so resuming takes effect on the next tick without a restart.
	if w.pause != nil && w.pause.IsPaused(models.WorkerTypeMagicLinkTokenCleanup) {
		return
	}

	if err := w.registry.DeleteExpired(ctx); err != nil {
		slog.Error("Failed to delete expired magic-link tokens", "error", err)
	} else {
		slog.Debug("Expired magic-link tokens cleaned up")
	}
}

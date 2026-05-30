// EmailVerificationCleanupWorker mirrors RefreshTokenCleanupWorker by
// design — same Start/Stop/runCleanup/cleanupOnce lifecycle and the same
// soft-pause skip (#1308). Each worker still owns its own registry type,
// worker-type pause key, and log wording, so a shared generic base would
// erase those per-worker specifics for negligible LOC savings.
//
//nolint:dupl // intentional symmetry with the refresh-token cleanup worker
package services

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

const defaultEmailVerificationCleanupInterval = 1 * time.Hour

// EmailVerificationCleanupWorker periodically deletes expired email
// verification tokens from the registry. For PostgreSQL this keeps the
// email_verifications table from growing unbounded; for the in-memory
// registry it reclaims memory during long-running test servers.
type EmailVerificationCleanupWorker struct {
	registry        registry.EmailVerificationRegistry
	cleanupInterval time.Duration
	pause           PauseChecker
	stopCh          chan struct{}
	stopOnce        sync.Once
	wg              sync.WaitGroup
}

// EmailVerificationCleanupOption customizes an EmailVerificationCleanupWorker created by NewEmailVerificationCleanupWorker.
type EmailVerificationCleanupOption func(*emailVerificationCleanupOptions)

type emailVerificationCleanupOptions struct {
	cleanupInterval time.Duration
	pause           PauseChecker
}

// WithEmailVerificationCleanupInterval overrides the default cleanup interval.
// Non-positive values are ignored.
func WithEmailVerificationCleanupInterval(d time.Duration) EmailVerificationCleanupOption {
	return func(o *emailVerificationCleanupOptions) {
		if d > 0 {
			o.cleanupInterval = d
		}
	}
}

// WithEmailVerificationCleanupPauseController wires the soft-pause controller
// so the worker skips its cleanup while the email-verification-cleanup worker
// type is paused (#1308). A nil checker leaves the worker unpaused.
func WithEmailVerificationCleanupPauseController(pc PauseChecker) EmailVerificationCleanupOption {
	return func(o *emailVerificationCleanupOptions) {
		if pc != nil {
			o.pause = pc
		}
	}
}

// NewEmailVerificationCleanupWorker creates a cleanup worker with the default one-hour interval,
// overridable via EmailVerificationCleanupOption values (e.g., WithEmailVerificationCleanupInterval).
func NewEmailVerificationCleanupWorker(r registry.EmailVerificationRegistry, opts ...EmailVerificationCleanupOption) *EmailVerificationCleanupWorker {
	options := emailVerificationCleanupOptions{
		cleanupInterval: defaultEmailVerificationCleanupInterval,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return &EmailVerificationCleanupWorker{
		registry:        r,
		cleanupInterval: options.cleanupInterval,
		pause:           options.pause,
		stopCh:          make(chan struct{}),
	}
}

// Start launches the background cleanup goroutine. It is a no-op if the registry is nil.
func (w *EmailVerificationCleanupWorker) Start(ctx context.Context) {
	if w.registry == nil {
		slog.Warn("EmailVerificationCleanupWorker: no registry configured, skipping startup")
		return
	}
	w.wg.Go(func() {
		w.runCleanup(ctx)
	})
	slog.Info("Email verification cleanup worker started", "interval", w.cleanupInterval)
}

// Stop signals the worker to stop and waits for it to finish.
func (w *EmailVerificationCleanupWorker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.wg.Wait()
	slog.Info("Email verification cleanup worker stopped")
}

func (w *EmailVerificationCleanupWorker) runCleanup(ctx context.Context) {
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
func (w *EmailVerificationCleanupWorker) cleanupOnce(ctx context.Context) {
	// Soft-pause (#1308): skip the cleanup while paused. The ticker keeps
	// running so resuming takes effect on the next tick without a restart.
	if w.pause != nil && w.pause.IsPaused(models.WorkerTypeEmailVerificationCleanup) {
		return
	}

	if err := w.registry.DeleteExpired(ctx); err != nil {
		slog.Error("Failed to delete expired email verification tokens", "error", err)
	} else {
		slog.Debug("Expired email verification tokens cleaned up")
	}
}

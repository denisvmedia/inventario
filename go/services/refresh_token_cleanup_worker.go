package services

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/denisvmedia/inventario/registry"
)

const defaultRefreshTokenCleanupInterval = 1 * time.Hour

// RefreshTokenCleanupWorker periodically deletes expired refresh tokens from the registry.
// For PostgreSQL this keeps the refresh_tokens table from growing unbounded;
// for the in-memory registry it reclaims memory during long-running test servers.
type RefreshTokenCleanupWorker struct {
	registry        registry.RefreshTokenRegistry
	cleanupInterval time.Duration
	stopCh          chan struct{}
	stopOnce        sync.Once
	wg              sync.WaitGroup
}

// NewRefreshTokenCleanupWorker creates a cleanup worker with a default interval of 1 hour.
func NewRefreshTokenCleanupWorker(r registry.RefreshTokenRegistry) *RefreshTokenCleanupWorker {
	return &RefreshTokenCleanupWorker{
		registry:        r,
		cleanupInterval: defaultRefreshTokenCleanupInterval,
		stopCh:          make(chan struct{}),
	}
}

// Start launches the background cleanup goroutine. It is a no-op if the registry is nil.
func (w *RefreshTokenCleanupWorker) Start(ctx context.Context) {
	if w.registry == nil {
		slog.Warn("RefreshTokenCleanupWorker: no registry configured, skipping startup")
		return
	}
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.runCleanup(ctx)
	}()
	slog.Info("Refresh token cleanup worker started", "interval", w.cleanupInterval)
}

// Stop signals the worker to stop and waits for it to finish.
func (w *RefreshTokenCleanupWorker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.wg.Wait()
	slog.Info("Refresh token cleanup worker stopped")
}

func (w *RefreshTokenCleanupWorker) runCleanup(ctx context.Context) {
	ticker := time.NewTicker(w.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			if err := w.registry.DeleteExpired(ctx); err != nil {
				slog.Error("Failed to delete expired refresh tokens", "error", err)
			} else {
				slog.Debug("Expired refresh tokens cleaned up")
			}
		}
	}
}

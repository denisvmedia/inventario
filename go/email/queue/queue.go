package queue

import (
	"context"
	"time"
)

// Queue is a delayed-retry-capable payload queue.
//
// Implementations maintain two logical buckets:
//   - ready payloads that workers can consume immediately,
//   - delayed retry payloads promoted to ready when due.
//
// Payloads are opaque to the queue; callers define payload schema.
type Queue interface {
	// Enqueue adds a payload to the ready queue.
	Enqueue(ctx context.Context, payload []byte) error
	// Dequeue blocks up to timeout waiting for a ready payload.
	// It returns (nil, nil) when timeout elapses without data.
	Dequeue(ctx context.Context, timeout time.Duration) ([]byte, error)
	// ScheduleRetry stores payload for future promotion at readyAt.
	ScheduleRetry(ctx context.Context, payload []byte, readyAt time.Time) error
	// PromoteDueRetries moves due retry payloads to ready queue.
	// Returns number of promoted payloads.
	PromoteDueRetries(ctx context.Context, now time.Time, limit int) (int, error)
}

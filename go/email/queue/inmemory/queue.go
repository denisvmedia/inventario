package inmemory

import (
	"context"
	"sync"
	"time"

	"github.com/denisvmedia/inventario/email/queue"
)

type scheduledPayload struct {
	readyAt time.Time
	payload []byte
}

// Queue is an in-memory queue backend that implements queue.Queue.
type Queue struct {
	ready chan []byte

	mu      sync.Mutex
	retries []scheduledPayload
}

// New creates an in-memory queue with the given ready-channel buffer size.
func New(buffer int) *Queue {
	if buffer <= 0 {
		buffer = 256
	}
	return &Queue{
		ready: make(chan []byte, buffer),
	}
}

var _ queue.Queue = (*Queue)(nil)

// Enqueue adds a payload to the ready queue.
func (q *Queue) Enqueue(ctx context.Context, payload []byte) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case q.ready <- clone(payload):
		return nil
	}
}

// Dequeue waits up to timeout for a ready payload.
func (q *Queue) Dequeue(ctx context.Context, timeout time.Duration) ([]byte, error) {
	if timeout <= 0 {
		timeout = 100 * time.Millisecond
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, nil
	case payload := <-q.ready:
		return clone(payload), nil
	}
}

// ScheduleRetry stores payload for future promotion at readyAt.
func (q *Queue) ScheduleRetry(ctx context.Context, payload []byte, readyAt time.Time) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	q.mu.Lock()
	q.retries = append(q.retries, scheduledPayload{
		readyAt: readyAt,
		payload: clone(payload),
	})
	q.mu.Unlock()
	return nil
}

// PromoteDueRetries moves due payloads to ready queue.
func (q *Queue) PromoteDueRetries(ctx context.Context, now time.Time, _ int) (int, error) {
	q.mu.Lock()
	due := make([][]byte, 0)
	remaining := make([]scheduledPayload, 0, len(q.retries))
	for _, pending := range q.retries {
		if pending.readyAt.After(now) {
			remaining = append(remaining, pending)
			continue
		}
		due = append(due, pending.payload)
	}
	q.retries = remaining
	q.mu.Unlock()

	moved := 0
	for _, payload := range due {
		select {
		case <-ctx.Done():
			return moved, ctx.Err()
		case q.ready <- clone(payload):
			moved++
		default:
			_ = q.ScheduleRetry(context.Background(), payload, now.Add(time.Second))
		}
	}
	return moved, nil
}

func clone(in []byte) []byte {
	if in == nil {
		return nil
	}
	out := make([]byte, len(in))
	copy(out, in)
	return out
}

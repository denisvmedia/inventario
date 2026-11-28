package services

// White-box: the dequeue loop is the unexported runWorker. Driving it directly
// with a scripted queue shows each pause it takes without starting the worker
// pool and the retry promoter.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

var errQueueDown = errors.New("queue is unavailable")

// scriptedQueue answers Dequeue from a script of errors, where nil is a
// successful call that found no job. Once the script runs out it cancels the
// worker's context, so the test ends without timing.
type scriptedQueue struct {
	erroringQueue
	mu     sync.Mutex
	script []error
	cancel context.CancelFunc
}

func (q *scriptedQueue) Dequeue(context.Context, time.Duration) ([]byte, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.script) == 0 {
		q.cancel()
		return nil, context.Canceled
	}
	err := q.script[0]
	q.script = q.script[1:]
	return nil, err
}

func TestAsyncEmailService_RunWorker_BacksOffOnDequeueFailures(t *testing.T) {
	c := qt.New(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	queue := &scriptedQueue{
		script: []error{errQueueDown, errQueueDown, errQueueDown, errQueueDown, nil, errQueueDown},
		cancel: cancel,
	}
	svc := &AsyncEmailService{
		queue:              queue,
		dequeueBackoffBase: time.Millisecond,
		dequeueBackoffMax:  4 * time.Millisecond,
		stopCh:             make(chan struct{}),
	}

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	svc.runWorker(ctx, 1)
	slog.SetDefault(prev)

	var pauses []any
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		var rec map[string]any
		c.Assert(json.Unmarshal([]byte(line), &rec), qt.IsNil)
		if rec["msg"] == "Email worker dequeue failed" {
			pauses = append(pauses, rec["retry_in"])
		}
	}
	// Doubles up to the cap, and starts over after a successful call.
	c.Assert(pauses, qt.DeepEquals, []any{"1ms", "2ms", "4ms", "4ms", "1ms"})
}

func TestAsyncEmailService_RunWorker_DoesNotSpinOnDequeueFailure(t *testing.T) {
	c := qt.New(t)
	queue := &erroringQueue{dequeueErr: errQueueDown}
	svc := &AsyncEmailService{
		queue:              queue,
		dequeueBackoffBase: 50 * time.Millisecond,
		dequeueBackoffMax:  time.Second,
		stopCh:             make(chan struct{}),
	}
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()

	svc.runWorker(ctx, 1)

	// Calls at 0, 50 and 150 ms, the next at 350 ms. Without a pause the
	// worker calls Dequeue as fast as it fails.
	c.Assert(queue.DequeueCalls() <= 5, qt.IsTrue, qt.Commentf("Dequeue calls: %d", queue.DequeueCalls()))
}

func TestAsyncEmailService_RunWorker_StopsDuringDequeueBackoff(t *testing.T) {
	c := qt.New(t)
	queue := &erroringQueue{dequeueErr: errQueueDown}
	svc := &AsyncEmailService{
		queue:              queue,
		dequeueBackoffBase: time.Hour,
		stopCh:             make(chan struct{}),
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		svc.runWorker(ctx, 1)
	}()
	for deadline := time.Now().Add(2 * time.Second); queue.DequeueCalls() == 0 && time.Now().Before(deadline); {
		time.Sleep(time.Millisecond)
	}

	cancel()

	c.Assert(waitForDone(done, time.Second), qt.IsTrue)
}

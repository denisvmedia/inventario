package inmemory

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

func TestQueue_EnqueueDequeue(t *testing.T) {
	c := qt.New(t)

	q := New(4)
	payload := []byte(`{"type":"verification","to":"user@example.com"}`)

	err := q.Enqueue(context.Background(), payload)
	c.Assert(err, qt.IsNil)

	// Mutate the original slice to verify queue stores its own copy.
	payload[0] = 'X'

	got, err := q.Dequeue(context.Background(), 20*time.Millisecond)
	c.Assert(err, qt.IsNil)
	c.Assert(string(got), qt.Equals, `{"type":"verification","to":"user@example.com"}`)
}

func TestQueue_DequeueTimeoutReturnsNil(t *testing.T) {
	c := qt.New(t)

	q := New(1)
	got, err := q.Dequeue(context.Background(), 5*time.Millisecond)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.IsNil)
}

func TestQueue_ScheduleRetryAndPromoteDueRetries(t *testing.T) {
	c := qt.New(t)

	q := New(4)
	now := time.Unix(1700000000, 0)

	duePayload := []byte("due")
	futurePayload := []byte("future")

	err := q.ScheduleRetry(context.Background(), duePayload, now.Add(-time.Second))
	c.Assert(err, qt.IsNil)
	err = q.ScheduleRetry(context.Background(), futurePayload, now.Add(2*time.Second))
	c.Assert(err, qt.IsNil)

	moved, err := q.PromoteDueRetries(context.Background(), now, 100)
	c.Assert(err, qt.IsNil)
	c.Assert(moved, qt.Equals, 1)

	got, err := q.Dequeue(context.Background(), 20*time.Millisecond)
	c.Assert(err, qt.IsNil)
	c.Assert(string(got), qt.Equals, "due")

	got, err = q.Dequeue(context.Background(), 5*time.Millisecond)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.IsNil)

	moved, err = q.PromoteDueRetries(context.Background(), now.Add(3*time.Second), 100)
	c.Assert(err, qt.IsNil)
	c.Assert(moved, qt.Equals, 1)

	got, err = q.Dequeue(context.Background(), 20*time.Millisecond)
	c.Assert(err, qt.IsNil)
	c.Assert(string(got), qt.Equals, "future")
}

package redis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	qt "github.com/frankban/quicktest"
)

func TestNewFromConfig_InvalidURL(t *testing.T) {
	c := qt.New(t)

	_, err := NewFromConfig(Config{
		RedisURL: "://bad-url",
	})
	c.Assert(err, qt.IsNotNil)
}

func TestQueue_EnqueueDequeue(t *testing.T) {
	c := qt.New(t)

	mr, err := miniredis.Run()
	c.Assert(err, qt.IsNil)
	defer mr.Close()

	q, err := NewFromConfig(Config{
		RedisURL: fmt.Sprintf("redis://%s/0", mr.Addr()),
	})
	c.Assert(err, qt.IsNil)

	payload := []byte(`{"id":"job-1"}`)
	err = q.Enqueue(context.Background(), payload)
	c.Assert(err, qt.IsNil)

	got, err := q.Dequeue(context.Background(), 20*time.Millisecond)
	c.Assert(err, qt.IsNil)
	c.Assert(string(got), qt.Equals, `{"id":"job-1"}`)
}

func TestQueue_DequeueTimeoutReturnsNil(t *testing.T) {
	c := qt.New(t)

	mr, err := miniredis.Run()
	c.Assert(err, qt.IsNil)
	defer mr.Close()

	q, err := NewFromConfig(Config{
		RedisURL: fmt.Sprintf("redis://%s/0", mr.Addr()),
	})
	c.Assert(err, qt.IsNil)

	got, err := q.Dequeue(context.Background(), 5*time.Millisecond)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.IsNil)
}

func TestQueue_ScheduleRetryAndPromoteDueRetries(t *testing.T) {
	c := qt.New(t)

	mr, err := miniredis.Run()
	c.Assert(err, qt.IsNil)
	defer mr.Close()

	q, err := NewFromConfig(Config{
		RedisURL: fmt.Sprintf("redis://%s/0", mr.Addr()),
	})
	c.Assert(err, qt.IsNil)

	now := time.Unix(1700000000, 0)
	err = q.ScheduleRetry(context.Background(), []byte("due"), now.Add(-time.Second))
	c.Assert(err, qt.IsNil)
	err = q.ScheduleRetry(context.Background(), []byte("future"), now.Add(2*time.Second))
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

func TestQueue_CustomKeysIsolation(t *testing.T) {
	c := qt.New(t)

	mr, err := miniredis.Run()
	c.Assert(err, qt.IsNil)
	defer mr.Close()

	q1, err := NewFromConfig(Config{
		RedisURL: fmt.Sprintf("redis://%s/0", mr.Addr()),
		ReadyKey: "emails:one:ready",
		RetryKey: "emails:one:retry",
	})
	c.Assert(err, qt.IsNil)

	q2, err := NewFromConfig(Config{
		RedisURL: fmt.Sprintf("redis://%s/0", mr.Addr()),
		ReadyKey: "emails:two:ready",
		RetryKey: "emails:two:retry",
	})
	c.Assert(err, qt.IsNil)

	err = q1.Enqueue(context.Background(), []byte("only-q1"))
	c.Assert(err, qt.IsNil)

	got, err := q2.Dequeue(context.Background(), 5*time.Millisecond)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.IsNil)

	got, err = q1.Dequeue(context.Background(), 20*time.Millisecond)
	c.Assert(err, qt.IsNil)
	c.Assert(string(got), qt.Equals, "only-q1")
}

func TestQueue_PromoteDueRetries_RespectsLimit(t *testing.T) {
	c := qt.New(t)

	mr, err := miniredis.Run()
	c.Assert(err, qt.IsNil)
	defer mr.Close()

	q, err := NewFromConfig(Config{
		RedisURL: fmt.Sprintf("redis://%s/0", mr.Addr()),
	})
	c.Assert(err, qt.IsNil)

	now := time.Unix(1700000000, 0)
	err = q.ScheduleRetry(context.Background(), []byte("a"), now.Add(-time.Second))
	c.Assert(err, qt.IsNil)
	err = q.ScheduleRetry(context.Background(), []byte("b"), now.Add(-time.Second))
	c.Assert(err, qt.IsNil)
	err = q.ScheduleRetry(context.Background(), []byte("c"), now.Add(-time.Second))
	c.Assert(err, qt.IsNil)

	moved, err := q.PromoteDueRetries(context.Background(), now, 2)
	c.Assert(err, qt.IsNil)
	c.Assert(moved, qt.Equals, 2)

	got1, err := q.Dequeue(context.Background(), 20*time.Millisecond)
	c.Assert(err, qt.IsNil)
	c.Assert(got1, qt.IsNotNil)
	got2, err := q.Dequeue(context.Background(), 20*time.Millisecond)
	c.Assert(err, qt.IsNil)
	c.Assert(got2, qt.IsNotNil)

	got3, err := q.Dequeue(context.Background(), 5*time.Millisecond)
	c.Assert(err, qt.IsNil)
	c.Assert(got3, qt.IsNil)

	moved, err = q.PromoteDueRetries(context.Background(), now, 2)
	c.Assert(err, qt.IsNil)
	c.Assert(moved, qt.Equals, 1)

	got3, err = q.Dequeue(context.Background(), 20*time.Millisecond)
	c.Assert(err, qt.IsNil)
	c.Assert(got3, qt.IsNotNil)
}

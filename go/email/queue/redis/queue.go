package redis

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	redisv9 "github.com/redis/go-redis/v9"

	"github.com/denisvmedia/inventario/email/queue"
)

const (
	defaultReadyKey    = "inventario:email:queue:ready"
	defaultRetryKey    = "inventario:email:queue:retry"
	startupPingTimeout = 2 * time.Second
)

// Config configures the Redis queue backend.
type Config struct {
	// RedisURL is required and used to construct the Redis client.
	RedisURL string
	// ReadyKey optionally overrides the list key for ready payloads.
	ReadyKey string
	// RetryKey optionally overrides the sorted-set key for delayed payloads.
	RetryKey string
}

// Queue is a Redis-backed queue.Queue implementation.
type Queue struct {
	client   *redisv9.Client
	readyKey string
	retryKey string
}

// NewFromConfig builds a Redis queue from configuration.
func NewFromConfig(cfg Config) (*Queue, error) {
	opts, err := redisv9.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}
	client := redisv9.NewClient(opts)
	pingCtx, pingCancel := context.WithTimeout(context.Background(), startupPingTimeout)
	defer pingCancel()
	if pingErr := client.Ping(pingCtx).Err(); pingErr != nil {
		slog.Warn("Redis email queue unreachable at startup; queue operations may fail until Redis becomes available", "error", pingErr)
	}

	readyKey := cfg.ReadyKey
	if readyKey == "" {
		readyKey = defaultReadyKey
	}
	retryKey := cfg.RetryKey
	if retryKey == "" {
		retryKey = defaultRetryKey
	}

	return &Queue{
		client:   client,
		readyKey: readyKey,
		retryKey: retryKey,
	}, nil
}

var _ queue.Queue = (*Queue)(nil)

// Enqueue adds payload to the ready list.
func (q *Queue) Enqueue(ctx context.Context, payload []byte) error {
	return q.client.RPush(ctx, q.readyKey, payload).Err()
}

// Dequeue blocks up to timeout waiting for payload from ready list.
func (q *Queue) Dequeue(ctx context.Context, timeout time.Duration) ([]byte, error) {
	res, err := q.client.BLPop(ctx, timeout, q.readyKey).Result()
	if err != nil {
		if err == redisv9.Nil {
			return nil, nil
		}
		return nil, err
	}
	if len(res) < 2 {
		return nil, nil
	}
	return []byte(res[1]), nil
}

// ScheduleRetry stores payload in retry sorted set until readyAt.
func (q *Queue) ScheduleRetry(ctx context.Context, payload []byte, readyAt time.Time) error {
	score := float64(readyAt.UnixMilli())
	return q.client.ZAdd(ctx, q.retryKey, redisv9.Z{
		Score:  score,
		Member: payload,
	}).Err()
}

// PromoteDueRetries moves due delayed payloads into ready list.
func (q *Queue) PromoteDueRetries(ctx context.Context, now time.Time, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}

	maxScore := fmt.Sprintf("%d", now.UnixMilli())
	duePayloads, err := q.client.ZRangeArgs(ctx, redisv9.ZRangeArgs{
		Key:     q.retryKey,
		Start:   "-inf",
		Stop:    maxScore,
		ByScore: true,
		Offset:  0,
		Count:   int64(limit),
	}).Result()
	if err != nil {
		return 0, err
	}
	if len(duePayloads) == 0 {
		return 0, nil
	}

	pipe := q.client.TxPipeline()
	for _, payload := range duePayloads {
		pipe.RPush(ctx, q.readyKey, payload)
		pipe.ZRem(ctx, q.retryKey, payload)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return len(duePayloads), nil
}

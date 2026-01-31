package services

import (
	"context"
	"sync"
	"time"

	"github.com/go-extras/errx"
	"github.com/go-extras/errx/stacktrace"
)

// RateLimiter provides per-user rate limiting functionality
type RateLimiter struct {
	mu           sync.RWMutex
	userBuckets  map[string]*TokenBucket
	limit        int           // requests per minute
	window       time.Duration // time window (1 minute)
	cleanupTimer *time.Timer
}

// TokenBucket represents a token bucket for rate limiting
type TokenBucket struct {
	tokens     int
	lastRefill time.Time
	limit      int
	window     time.Duration
}

// NewRateLimiter creates a new rate limiter with the specified limit per minute
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	rl := &RateLimiter{
		userBuckets: make(map[string]*TokenBucket),
		limit:       requestsPerMinute,
		window:      time.Minute,
	}

	// Start cleanup routine to remove old buckets
	rl.startCleanup()
	return rl
}

// Allow checks if a request is allowed for the given user
func (rl *RateLimiter) Allow(userID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.userBuckets[userID]
	if !exists {
		bucket = &TokenBucket{
			tokens:     rl.limit,
			lastRefill: time.Now(),
			limit:      rl.limit,
			window:     rl.window,
		}
		rl.userBuckets[userID] = bucket
	}

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill)
	if elapsed >= bucket.window {
		// Full refill after a complete window
		bucket.tokens = bucket.limit
		bucket.lastRefill = now
	} else {
		// Partial refill based on elapsed time
		tokensToAdd := int(float64(bucket.limit) * elapsed.Seconds() / bucket.window.Seconds())
		bucket.tokens = minInt(bucket.limit, bucket.tokens+tokensToAdd)
		if tokensToAdd > 0 {
			bucket.lastRefill = now
		}
	}

	// Check if we have tokens available
	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}

	return false
}

// GetRemainingTokens returns the number of remaining tokens for a user
func (rl *RateLimiter) GetRemainingTokens(userID string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	bucket, exists := rl.userBuckets[userID]
	if !exists {
		return rl.limit
	}

	// Calculate current tokens (similar to Allow but without consuming)
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill)
	if elapsed >= bucket.window {
		return bucket.limit
	}

	tokensToAdd := int(float64(bucket.limit) * elapsed.Seconds() / bucket.window.Seconds())
	return minInt(bucket.limit, bucket.tokens+tokensToAdd)
}

// startCleanup starts a background routine to clean up old buckets
func (rl *RateLimiter) startCleanup() {
	rl.cleanupTimer = time.AfterFunc(5*time.Minute, func() {
		rl.cleanup()
		rl.startCleanup() // Reschedule
	})
}

// cleanup removes buckets that haven't been used recently
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for userID, bucket := range rl.userBuckets {
		// Remove buckets that haven't been used in the last 10 minutes
		if now.Sub(bucket.lastRefill) > 10*time.Minute {
			delete(rl.userBuckets, userID)
		}
	}
}

// Stop stops the cleanup routine
func (rl *RateLimiter) Stop() {
	if rl.cleanupTimer != nil {
		rl.cleanupTimer.Stop()
	}
}

// min returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ThumbnailRateLimitService provides rate limiting for thumbnail generation requests
type ThumbnailRateLimitService struct {
	rateLimiter *RateLimiter
}

// NewThumbnailRateLimitService creates a new thumbnail rate limit service
func NewThumbnailRateLimitService(requestsPerMinute int) *ThumbnailRateLimitService {
	return &ThumbnailRateLimitService{
		rateLimiter: NewRateLimiter(requestsPerMinute),
	}
}

// CheckRateLimit checks if a thumbnail generation request is allowed for the user
func (s *ThumbnailRateLimitService) CheckRateLimit(ctx context.Context, userID string) error {
	if !s.rateLimiter.Allow(userID) {
		remaining := s.rateLimiter.GetRemainingTokens(userID)
		return stacktrace.Wrap("thumbnail generation rate limit exceeded", ErrRateLimitExceeded, errx.Attrs("user_id", userID,
			"remaining_tokens", remaining))
	}
	return nil
}

// GetRemainingRequests returns the number of remaining requests for a user
func (s *ThumbnailRateLimitService) GetRemainingRequests(ctx context.Context, userID string) int {
	return s.rateLimiter.GetRemainingTokens(userID)
}

// Stop stops the rate limiter cleanup routine
func (s *ThumbnailRateLimitService) Stop() {
	s.rateLimiter.Stop()
}

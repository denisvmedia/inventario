package services

import "github.com/go-extras/errx"

var (
	// ErrInvalidThumbnailSize is returned when an invalid thumbnail size is requested
	ErrInvalidThumbnailSize = errx.NewSentinel("invalid thumbnail size")

	// ErrRateLimitExceeded is returned when a user exceeds their rate limit
	ErrRateLimitExceeded = errx.NewSentinel("rate limit exceeded")
)

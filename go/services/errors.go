package services

import "errors"

var (
	// ErrInvalidThumbnailSize is returned when an invalid thumbnail size is requested
	ErrInvalidThumbnailSize = errors.New("invalid thumbnail size")

	// ErrRateLimitExceeded is returned when a user exceeds their rate limit
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

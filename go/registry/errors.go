package registry

import (
	"github.com/go-extras/errx"
)

var (
	ErrNotFound           = errx.NewSentinel("not found")
	ErrCannotDelete       = errx.NewSentinel("cannot delete")
	ErrInvalidConfig      = errx.NewSentinel("invalid config")
	ErrInvalidInput       = errx.NewSentinel("invalid input")
	ErrFieldRequired      = errx.NewSentinel("field required")
	ErrAlreadyExists      = errx.NewSentinel("already exists")
	ErrEmailAlreadyExists = errx.NewSentinel("email already exists")
	ErrSlugAlreadyExists  = errx.NewSentinel("slug already exists")
	ErrBadDataStructure   = errx.NewSentinel("bad data structure")
	ErrDeleted            = errx.NewSentinel("deleted", ErrNotFound)

	ErrMainCurrencyNotSet       = errx.NewSentinel("main currency not set")
	ErrMainCurrencyAlreadySet   = errx.NewSentinel("main currency already set and cannot be changed")
	ErrUserContextRequired      = errx.NewSentinel("user context required")
	ErrResourceLimitExceeded    = errx.NewSentinel("resource limit exceeded")
	ErrConcurrencyLimitExceeded = errx.NewSentinel("concurrency limit exceeded")
	ErrTooManyRequests          = errx.NewSentinel("too many requests")
)

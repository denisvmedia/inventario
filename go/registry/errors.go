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
	ErrInvalidSettingName       = errx.NewSentinel("invalid setting name")
	ErrUserContextRequired      = errx.NewSentinel("user context required")
	ErrResourceLimitExceeded    = errx.NewSentinel("resource limit exceeded")
	ErrConcurrencyLimitExceeded = errx.NewSentinel("concurrency limit exceeded")
	ErrTooManyRequests          = errx.NewSentinel("too many requests")

	// ErrTagInUse signals that a tag still has commodity / file
	// references and DeleteAtomic was invoked with force=false. Returned
	// alongside the populated TagUsage so callers can render the
	// breakdown without a second round-trip. Defined here (not in
	// services) so registry implementations can return it directly from
	// inside the lock-protected delete path.
	ErrTagInUse = errx.NewSentinel("tag is in use")

	// ErrLoanAlreadyOpen signals that a commodity already has an open
	// (returned_at IS NULL) loan and the service refused to create a
	// second one. The handler maps it to 409 Conflict so the FE can
	// surface the existing loan instead of stacking duplicates.
	ErrLoanAlreadyOpen = errx.NewSentinel("commodity already has an open loan")

	// ErrLoanAlreadyReturned signals that a return-loan call hit a row
	// whose returned_at is already set. Idempotent return is intentionally
	// NOT supported — the FE should refresh and stop offering the
	// "Mark returned" button when the loan closes.
	ErrLoanAlreadyReturned = errx.NewSentinel("loan already returned")
)

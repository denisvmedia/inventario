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

	ErrGroupCurrencyNotSet      = errx.NewSentinel("group currency not set")
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

	// ErrServiceAlreadyOpen signals that a commodity already has an open
	// (returned_at IS NULL) commodity_services row and the service refused
	// to create a second one. Mirrors ErrLoanAlreadyOpen for the service
	// (#1508) sibling feature.
	ErrServiceAlreadyOpen = errx.NewSentinel("commodity already has an open service")

	// ErrServiceAlreadyReturned signals that a return-service call hit a
	// row whose returned_at is already set. Same idempotency stance as
	// ErrLoanAlreadyReturned.
	ErrServiceAlreadyReturned = errx.NewSentinel("service already returned")

	// ErrCommodityAlreadyOut signals that a holding-creating call (start
	// loan or send for service) found a different OPEN holding kind on
	// the same commodity. The service layer enforces this cross-kind
	// invariant so the FE can render a meaningful 409 ("already at Apple
	// Service since 2026-03-12" / "already lent to X since Y") instead of
	// stacking a parallel hold. The companion error message includes the
	// existing holding's kind so the apiserver can hand back the right
	// payload shape.
	ErrCommodityAlreadyOut = errx.NewSentinel("commodity is already out (open loan or service)")

	// ErrMigrationInFlight signals that a currency migration row in
	// pending or running state already exists for the target group, and
	// CurrencyMigrationRegistry.Create refused to insert a second one.
	// At the schema level this is the partial unique index
	// idx_currency_migrations_group_in_flight; at the API layer the
	// apiserver maps this to 409 currency_migration.migration_in_progress.
	ErrMigrationInFlight = errx.NewSentinel("currency migration already in flight for group")

	// ErrAcquisitionAlreadySet signals that the migrationops.SetAcquisition
	// runtime guard refused to overwrite a row whose acquisition_price /
	// acquisition_currency were already set. Indicates a programming
	// error (the worker should only fill on the first Case-A migration);
	// surfaced as a 5xx with this sentinel for diagnosis.
	ErrAcquisitionAlreadySet = errx.NewSentinel("commodity acquisition columns already set; refusing to overwrite")

	// ErrPreviewTokenInvalid signals that VerifyPreviewToken could not
	// validate the HMAC signature (forged, tampered, or signed with a
	// different key). Maps to 422 currency_migration.token_invalid.
	ErrPreviewTokenInvalid = errx.NewSentinel("preview token signature invalid")

	// ErrPreviewTokenExpired signals that the preview token's embedded
	// expiry is in the past. Maps to 409 currency_migration.preview_expired.
	ErrPreviewTokenExpired = errx.NewSentinel("preview token expired")

	// ErrLastOwner signals that DeleteWithMemberInvariants refused to
	// remove the only owner row in a group. Mirrors the user-facing
	// invariant in services.ErrLastOwner (which wraps this on its way
	// to the handler). Living at the registry layer lets the
	// transactional delete path return it directly from under the
	// per-group lock without re-classifying through the service.
	// #1652.
	ErrLastOwner = errx.NewSentinel("cannot remove the last owner from a group")

	// ErrLastMember signals that DeleteWithMemberInvariants refused to
	// drop the group to zero memberships. Defense-in-depth companion
	// to ErrLastOwner: even if the role taxonomy ever drifts and the
	// owner check passes vacuously (e.g. the leaving user lands in a
	// non-owner role on a single-member group), the member-count
	// invariant still blocks the leave. #1652.
	ErrLastMember = errx.NewSentinel("cannot remove the last member from a group")

	// ErrLastSystemAdmin signals that RevokeSystemAdminAtomic refused to
	// drop the platform's system-admin count to zero. Defined at the
	// registry layer so both backends can return the same sentinel from
	// inside the lock-protected revoke path, and so the CLI / future HTTP
	// callers branch on a single identity. The CLI exposes the override
	// via `--allow-zero`; the HTTP path will never expose it. #1745.
	ErrLastSystemAdmin = errx.NewSentinel("cannot revoke the last system administrator")

	// ErrBackofficeUserNotFound is returned by BackofficeUserRegistry
	// lookups when no row matches the supplied id / email. Distinct
	// sentinel from ErrNotFound so callers in the back-office plane
	// (issue #1785) can render messages targeted at platform operators
	// without conflating with regular user/tenant misses.
	ErrBackofficeUserNotFound = errx.NewSentinel("backoffice user not found", ErrNotFound)

	// ErrBackofficeEmailAlreadyExists is returned by
	// BackofficeUserRegistry.Create when the lowercased email collides
	// with an existing row. Email is unique platform-wide for back-office
	// identities (no tenant_id to partition on), so this is the canonical
	// duplicate-create error for that table.
	ErrBackofficeEmailAlreadyExists = errx.NewSentinel("backoffice user email already exists", ErrEmailAlreadyExists)

	// ErrInvalidBackofficeRole is returned by BackofficeUserRegistry.Create
	// / Update when the supplied role is not one of the closed-set values
	// declared on models.BackofficeRole. The model's ValidateWithContext
	// catches this on the happy path; the registry-level check is a
	// defense-in-depth guard for callers that skip model validation.
	ErrInvalidBackofficeRole = errx.NewSentinel("invalid backoffice role")
)

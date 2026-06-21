package apiserver

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/render"
	"github.com/go-extras/errx"

	"github.com/denisvmedia/inventario/internal/errormarshal"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

var (
	ErrUnknownContentType     = errx.NewSentinel("render: unable to automatically decode the request content type")
	ErrInvalidContentType     = errx.NewSentinel("invalid content type")
	ErrNoFilesUploaded        = errx.NewSentinel("no files uploaded")
	ErrEntityNotFound         = errx.NewSentinel("entity not found")
	ErrTenantNotFound         = errx.NewSentinel("tenant not found")
	ErrUnknownThumbnailStatus = errx.NewSentinel("unknown thumbnail generation status")
	ErrMissingUploadSlot      = errx.NewSentinel("missing X-Upload-Slot header")
	ErrInvalidUploadSlot      = errx.NewSentinel("invalid or expired upload slot")
	ErrNotFound               = errx.NewSentinel("not found", registry.ErrNotFound)
	// errInvalidDeleteStrategy is returned when a DELETE request carries an
	// unknown `?strategy=` value (#2137). The handler answers 422 directly via
	// unprocessableEntityError, so it does not need a toJSONAPIError mapping.
	errInvalidDeleteStrategy = errx.NewSentinel("invalid delete strategy: must be one of cascade, unlink")
	// errImportSourceForeignTenant rejects a backup-import request whose
	// SourceFilePath points outside the caller's own tenant namespace
	// (`t/<callerTenant>/...`). A signed `.inb` archive is verified against a
	// tenant-AGNOSTIC server key, so without this guard a user could import
	// another tenant's backup by naming its blob key — cross-tenant data
	// exfiltration. The handler answers 422 directly via
	// unprocessableEntityError, so it does not need a toJSONAPIError mapping.
	errImportSourceForeignTenant = errx.NewSentinel("import source path must be within your tenant namespace")
	// ErrNotSystemAdmin is returned by RequireSystemAdmin when the caller's
	// user is not flagged as a system administrator. Surfaces as a 403 with
	// JSON:API code "admin.forbidden" so the FE can render specific copy
	// instead of the generic "permission denied" toast (#1745).
	ErrNotSystemAdmin = errx.NewSentinel("system administrator privileges required")
	// ErrPlatformAdminRequired is returned by RequirePlatformAdmin when a
	// back-office user authenticates successfully but their role is not
	// platform_admin (e.g. a support_agent attempting to start an
	// impersonation session). Surfaces as a 403 with JSON:API code
	// AdminRoleRequiredCode so the FE can render "ask a platform admin to
	// do this" instead of the generic "forbidden" toast (#1785, Phase 5).
	ErrPlatformAdminRequired = errx.NewSentinel("platform administrator role required")
	// ErrMissingUserContext fires when a handler runs without an
	// authenticated user in context — almost always a middleware-wiring
	// bug (JWTMiddleware was bypassed). Distinct from ErrNotSystemAdmin
	// because the right diagnosis is "not authenticated", not "not
	// authorized for admin"; using the auth sentinel makes the 401 path
	// readable in logs and avoids misleading clients with admin copy.
	ErrMissingUserContext = errx.NewSentinel("authenticated user context required")

	// ErrAdminCannotBlockSelf rejects POST /admin/users/{id}/block when
	// the caller would deactivate their own account. Without this guard
	// an operator can lock themselves out of every admin surface in a
	// single request — the recovery path is hand-flipping is_active in
	// the database, which is exactly the kind of lockout #1745 was
	// designed to prevent (mirrors the "last system admin" invariant on
	// revoke). 422 + JSON:API code "admin.block.self_blocked" lets the
	// FE render specific copy and lets e2e assertions branch on the
	// code rather than a generic 422.
	ErrAdminCannotBlockSelf = errx.NewSentinel("system administrators cannot block their own account")
	// ErrAdminCannotBlockAdminWithoutForce rejects blocking another
	// system admin when the request body's `force` flag is absent or
	// false. Symmetric with the "cannot impersonate another system
	// admin without force" rule that lands with the impersonation
	// primitive (#1750): blocking a peer admin is a sensitive action
	// that demands an explicit override so a typo on a username can't
	// quietly disable a fellow operator. 422 + JSON:API code
	// "admin.block.admin_requires_force" makes the FE branch trivial.
	ErrAdminCannotBlockAdminWithoutForce = errx.NewSentinel("blocking another system administrator requires force=true")

	// ErrCannotImpersonateAdmin rejects POST /admin/users/{id}/impersonate
	// when the target user is itself a system administrator. Impersonating
	// a peer admin would let an operator borrow another operator's
	// platform-admin authority while the audit trail records only the
	// borrowed identity — a privilege-escalation footgun the primitive
	// (#1750) refuses outright (no `force` override, unlike block).
	// 422 + JSON:API code "admin.impersonate.target_is_admin".
	ErrCannotImpersonateAdmin = errx.NewSentinel("system administrators cannot be impersonated")
	// ErrTargetBlocked rejects impersonation of a user whose account is
	// blocked (IsActive=false). Impersonating a deactivated user would
	// resurrect a session the block was meant to tear down. 422 +
	// JSON:API code "admin.impersonate.target_blocked".
	ErrTargetBlocked = errx.NewSentinel("cannot impersonate a blocked user")
	// ErrNestedImpersonation rejects an impersonate request made through
	// an already-impersonated session (the caller's access token carries
	// `imp=true`). Nesting impersonation would make the audit chain
	// ambiguous about who the operator-of-record is. 422 + JSON:API code
	// "admin.impersonate.nested".
	ErrNestedImpersonation = errx.NewSentinel("cannot start impersonation from an impersonated session")
	// ErrNotImpersonating is returned by POST /admin/impersonation/end and
	// GET /admin/impersonation/current when the caller holds a validly
	// signed impersonation token but no live server-side session backs it
	// (the return slot is missing or its operator-of-record disagrees with
	// the token). This is a business-rule outcome — the request
	// authenticated fine, there is just nothing to end. 422 + JSON:API
	// code "admin.impersonate.not_active".
	ErrNotImpersonating = errx.NewSentinel("no active impersonation session")
	// ErrImpersonationTokenInvalid is returned by POST /admin/impersonation/end
	// when the Authorization header is missing/malformed, the token's
	// signature or algorithm is invalid, or the token is not an
	// impersonation token. The `end` route is mounted WITHOUT JWTMiddleware,
	// so this sentinel re-introduces the authentication failure the
	// middleware would otherwise raise: it is an AUTHENTICATION failure,
	// not a business-rule violation, and maps to 401 — distinct from
	// ErrNotImpersonating's 422. Surfaced via NewUnauthorizedError.
	ErrImpersonationTokenInvalid = errx.NewSentinel("invalid or missing impersonation token")
	// ErrImpersonationTokenCannotRefresh is returned by POST /auth/refresh
	// when the caller presents an impersonation access token. Impersonation
	// sessions are deliberately non-refreshable so they expire hard at the
	// TTL. Surfaces as 401 — the refresh endpoint already returns plain-text
	// 401s, so this sentinel only documents the rejection in one place.
	ErrImpersonationTokenCannotRefresh = errx.NewSentinel("impersonation tokens cannot be refreshed")
)

// JSON:API error codes returned by the impersonation endpoints (#1750).
// Kept as constants so the swagger annotations, the FE branch table, and
// the handler tests reference the same literals. Codes follow the
// "admin.impersonate.*" family.
const (
	// AdminImpersonateTargetIsAdminCode signals "the impersonation target
	// is a system administrator". Maps to a 422.
	AdminImpersonateTargetIsAdminCode = "admin.impersonate.target_is_admin"
	// AdminImpersonateTargetBlockedCode signals "the impersonation target
	// account is blocked". Maps to a 422.
	AdminImpersonateTargetBlockedCode = "admin.impersonate.target_blocked"
	// AdminImpersonateNestedCode signals "the caller is already inside an
	// impersonation session". Maps to a 422.
	AdminImpersonateNestedCode = "admin.impersonate.nested"
	// AdminImpersonateNotActiveCode signals "the caller is not in an
	// impersonation session" — returned by end / current. Maps to a 422.
	AdminImpersonateNotActiveCode = "admin.impersonate.not_active"
	// AdminImpersonateRateLimitedCode signals "the per-admin impersonation
	// start rate limit was exceeded". Maps to a 429.
	AdminImpersonateRateLimitedCode = "admin.impersonate.rate_limited"
	// AdminImpersonateReasonTooLongCode signals "the supplied impersonation
	// reason exceeds the 500-character cap". Maps to a 422 — mirrors the
	// admin.block.reason_too_long contract so the FE handles an over-long
	// reason identically across the block and impersonate forms.
	AdminImpersonateReasonTooLongCode = "admin.impersonate.reason_too_long"
)

func NewNotFoundError(err error) jsonapi.Error {
	return jsonapi.Error{
		Err:            err,
		UserError:      errormarshal.Marshal(err),
		HTTPStatusCode: http.StatusNotFound,
		StatusText:     "Not Found",
	}
}

// NewMaskedNotFoundError returns a 404 whose JSON body is the generic
// ErrNotFound message, while preserving the original error in the Err field
// for server-side logging. Use it when disclosing why a resource is "not
// found" would leak information that breaks a security/isolation boundary
// (e.g. confirming that an invite ID exists but belongs to a different group).
func NewMaskedNotFoundError(err error) jsonapi.Error {
	return jsonapi.Error{
		Err:            err,
		UserError:      errormarshal.Marshal(ErrNotFound),
		HTTPStatusCode: http.StatusNotFound,
		StatusText:     "Not Found",
	}
}

func NewUnprocessableEntityError(err error) jsonapi.Error {
	return jsonapi.Error{
		Err:            err,
		UserError:      errormarshal.Marshal(err),
		HTTPStatusCode: http.StatusUnprocessableEntity,
		StatusText:     "Unprocessable Entity",
	}
}

func NewInternalServerError(err error) jsonapi.Error {
	return jsonapi.Error{
		Err:            err,
		HTTPStatusCode: http.StatusInternalServerError,
		StatusText:     "Internal Server Error",
	}
}

func NewUnauthorizedError(err error) jsonapi.Error {
	return jsonapi.Error{
		Err:            err,
		HTTPStatusCode: http.StatusUnauthorized,
		StatusText:     "Unauthorized",
	}
}

func NewBadRequestError(err error) jsonapi.Error {
	return jsonapi.Error{
		Err:            err,
		UserError:      errormarshal.Marshal(err),
		HTTPStatusCode: http.StatusBadRequest,
		StatusText:     "Bad Request",
	}
}

func NewTooManyRequestsError(err error) jsonapi.Error {
	return jsonapi.Error{
		Err:            err,
		UserError:      errormarshal.Marshal(err),
		HTTPStatusCode: http.StatusTooManyRequests,
		StatusText:     "Too Many Requests",
	}
}

func internalServerError(w http.ResponseWriter, r *http.Request, err error) error {
	slog.Error("internal server error", "error", err)
	return render.Render(w, r, jsonapi.NewErrors(NewInternalServerError(err)))
}

func unauthorizedError(w http.ResponseWriter, r *http.Request, err error) error {
	return render.Render(w, r, jsonapi.NewErrors(NewUnauthorizedError(err)))
}

// codedForbiddenError renders a 403 with a JSON:API error code so the FE
// can branch on the code rather than the bare status. Used by
// RequireSystemAdmin (#1745); shape mirrors codedNotFoundError /
// codedConflictError.
func codedForbiddenError(w http.ResponseWriter, r *http.Request, err error, code string) error {
	jsErr := jsonapi.Error{
		Err:            err,
		UserError:      errormarshal.Marshal(err),
		HTTPStatusCode: http.StatusForbidden,
		StatusText:     "Forbidden",
		Code:           code,
	}
	return render.Render(w, r, jsonapi.NewErrors(jsErr))
}

func unprocessableEntityError(w http.ResponseWriter, r *http.Request, err error) error {
	return render.Render(w, r, jsonapi.NewErrors(NewUnprocessableEntityError(err)))
}

// requestEntityTooLargeError renders a 413 Payload Too Large with the
// JSON:API envelope. Used by the upload path (#2101) when a streamed file
// exceeds the configured per-file size cap.
func requestEntityTooLargeError(w http.ResponseWriter, r *http.Request, err error) error {
	jsErr := jsonapi.Error{
		Err:            err,
		UserError:      errormarshal.Marshal(err),
		HTTPStatusCode: http.StatusRequestEntityTooLarge,
		StatusText:     "Request Entity Too Large",
	}
	return render.Render(w, r, jsonapi.NewErrors(jsErr))
}

// adminBlockGuardError maps the two #1747 admin-block guard sentinels
// (self-block, admin-on-admin without force) to their 422 + code wire
// shape. Extracted so the toJSONAPIError switch stays under the
// gocyclo budget while keeping the mapping explicit and grep-friendly.
func adminBlockGuardError(err error) jsonapi.Error {
	code := AdminBlockSelfBlockedCode
	if errors.Is(err, ErrAdminCannotBlockAdminWithoutForce) {
		code = AdminBlockAdminRequiresForceCode
	}
	return jsonapi.Error{
		Err:            err,
		UserError:      errormarshal.Marshal(err),
		HTTPStatusCode: http.StatusUnprocessableEntity,
		StatusText:     "Unprocessable Entity",
		Code:           code,
	}
}

// adminMemberTenantMismatchError maps the #1749 cross-tenant
// add-member rejection (services.ErrTenantMismatch) to its 422 + code
// wire shape. Extracted as a free function — like adminBlockGuardError
// — so the toJSONAPIError switch stays under the gocyclo budget while
// keeping the mapping explicit and grep-friendly.
func adminMemberTenantMismatchError(err error) jsonapi.Error {
	return jsonapi.Error{
		Err:            err,
		UserError:      errormarshal.Marshal(err),
		HTTPStatusCode: http.StatusUnprocessableEntity,
		StatusText:     "Unprocessable Entity",
		Code:           adminMemberTenantMismatchCode,
	}
}

// inviteAcceptRejectionError maps the invite-accept business-rule
// sentinels (expired, already used, already a member, #1221 email
// mismatch) to their 422 wire shape. The email-mismatch case carries
// the JSON:API code `invite.email_mismatch` so the FE can distinguish
// it from generic invite-state errors; the other three keep the bare
// 422 they had pre-#1221. Extracted as a free function so the
// toJSONAPIError switch stays under the gocyclo budget while keeping
// the mapping explicit and grep-friendly.
func inviteAcceptRejectionError(err error) jsonapi.Error {
	if errors.Is(err, services.ErrInviteEmailMismatch) {
		return jsonapi.Error{
			Err:            err,
			UserError:      errormarshal.Marshal(err),
			HTTPStatusCode: http.StatusUnprocessableEntity,
			StatusText:     "Unprocessable Entity",
			Code:           "invite.email_mismatch",
		}
	}
	return NewUnprocessableEntityError(err)
}

// adminImpersonationGuardError maps the #1750 impersonation guard
// sentinels (target-is-admin, target-blocked, nested, not-active) to
// their 422 + code wire shape. Extracted as a free function — like
// adminBlockGuardError — so the toJSONAPIError switch stays under the
// gocyclo budget while keeping the mapping explicit and grep-friendly.
func adminImpersonationGuardError(err error) jsonapi.Error {
	code := AdminImpersonateTargetIsAdminCode
	switch {
	case errors.Is(err, ErrTargetBlocked):
		code = AdminImpersonateTargetBlockedCode
	case errors.Is(err, ErrNestedImpersonation):
		code = AdminImpersonateNestedCode
	case errors.Is(err, ErrNotImpersonating):
		code = AdminImpersonateNotActiveCode
	}
	return jsonapi.Error{
		Err:            err,
		UserError:      errormarshal.Marshal(err),
		HTTPStatusCode: http.StatusUnprocessableEntity,
		StatusText:     "Unprocessable Entity",
		Code:           code,
	}
}

// adminSentinelJSONAPIError maps the admin-only guard sentinels
// (RequireSystemAdmin, the #1747 block guards, the #1750 impersonation
// guards) to their JSON:API wire shape. Extracted as a single
// early-return helper called before the toJSONAPIError switch so the
// switch stays under the gocyclo budget; ok=false when err is not an
// admin sentinel, leaving the switch to handle it.
func adminSentinelJSONAPIError(err error) (jsonapi.Error, bool) {
	switch {
	case errors.Is(err, ErrNotSystemAdmin):
		// Reached via renderEntityError if a handler returns the sentinel
		// instead of relying on RequireSystemAdmin (the middleware writes
		// its own response). 403 + admin.forbidden code keeps the wire
		// shape identical so the FE doesn't need a second branch.
		return jsonapi.Error{
			Err:            err,
			UserError:      errormarshal.Marshal(err),
			HTTPStatusCode: http.StatusForbidden,
			StatusText:     "Forbidden",
			Code:           adminForbiddenCode,
		}, true
	case errors.Is(err, ErrPlatformAdminRequired):
		// #1785 Phase 5: RequirePlatformAdmin emits this via codedForbiddenError
		// already; the toJSONAPIError branch matters when a handler chooses to
		// return the sentinel directly (e.g. a deeper role check after a wider
		// gate). 403 + admin.role_required keeps the FE branch identical.
		return jsonapi.Error{
			Err:            err,
			UserError:      errormarshal.Marshal(err),
			HTTPStatusCode: http.StatusForbidden,
			StatusText:     "Forbidden",
			Code:           AdminRoleRequiredCode,
		}, true
	case errors.Is(err, ErrAdminCannotBlockSelf),
		errors.Is(err, ErrAdminCannotBlockAdminWithoutForce):
		// Admin block-guard rejections (#1747). Self-lockout and
		// admin-on-admin without `force: true` both surface as 422 with
		// a sentinel-specific JSON:API code so the FE can render
		// targeted copy instead of a generic 422 toast.
		return adminBlockGuardError(err), true
	case errors.Is(err, ErrCannotImpersonateAdmin),
		errors.Is(err, ErrTargetBlocked),
		errors.Is(err, ErrNestedImpersonation),
		errors.Is(err, ErrNotImpersonating):
		// Impersonation guard rejections (#1750). Target-is-admin,
		// target-blocked, nested-impersonation and not-active all surface
		// as 422 with a sentinel-specific JSON:API code so the FE can
		// render targeted copy. Extracted into a helper so the switch
		// stays under the gocyclo budget.
		return adminImpersonationGuardError(err), true
	default:
		return jsonapi.Error{}, false
	}
}

func toJSONAPIError(err error) jsonapi.Error {
	if jsErr, ok := adminSentinelJSONAPIError(err); ok {
		return jsErr
	}
	switch {
	case errors.Is(err, registry.ErrCannotDelete):
		return NewUnprocessableEntityError(err)
	case errors.Is(err, registry.ErrNotFound):
		return NewNotFoundError(err)
	case errors.Is(err, services.ErrInviteNotInGroup):
		return NewMaskedNotFoundError(err)
	case errors.Is(err, services.ErrLastAdmin),
		errors.Is(err, services.ErrLastOwner):
		// Both invariants surface as 422 business-rule violations. After
		// the #1533 role-taxonomy expansion the live sentinel is
		// ErrLastOwner; ErrLastAdmin stays for backwards compatibility
		// with any caller that hasn't migrated. The `group.last_owner`
		// JSON:API code is symmetric to `group.last_member` below — it
		// lets the admin FE (#1756) surface distinct copy ("transfer
		// ownership first") instead of a generic 422. The `code` field
		// is purely additive — message-based consumers are unaffected.
		return jsonapi.Error{
			Err:            err,
			UserError:      errormarshal.Marshal(err),
			HTTPStatusCode: http.StatusUnprocessableEntity,
			StatusText:     "Unprocessable Entity",
			Code:           "group.last_owner",
		}
	case errors.Is(err, services.ErrLastMember):
		// #1652 defense-in-depth: removing the last member of any role
		// is rejected even when the owner check would pass vacuously.
		// 422 mirrors ErrLastOwner; the JSON:API `code` lets the FE
		// surface distinct copy ("delete the group instead") instead
		// of muddling it with the "last owner — transfer first" path.
		return jsonapi.Error{
			Err:            err,
			UserError:      errormarshal.Marshal(err),
			HTTPStatusCode: http.StatusUnprocessableEntity,
			StatusText:     "Unprocessable Entity",
			Code:           "group.last_member",
		}
	case errors.Is(err, services.ErrInviteNotByEmail):
		// Resending a legacy token-only invite (no captured email)
		// is a business-rule violation: 422, not 500.
		return NewUnprocessableEntityError(err)
	case errors.Is(err, services.ErrCommodityNotTrackable),
		errors.Is(err, services.ErrClosedLoanFieldImmutable):
		// #1554: a bundle commodity (count > 1) cannot carry a per-
		// instance event (lend / service / warranty) — the FE renders
		// the "split into separate items" hint the create-form banner
		// uses. #1511: due_back_at / returned_at are frozen on closed
		// loans (date-of-record after the loan ends). Both are
		// business-rule violations: 422, not 500.
		return NewUnprocessableEntityError(err)
	case errors.Is(err, services.ErrInvalidConfirmation),
		errors.Is(err, services.ErrInvalidPassword):
		return NewUnprocessableEntityError(err)
	case errors.Is(err, ErrImpersonationTokenInvalid):
		// #1750: POST /admin/impersonation/end self-validates the imp token
		// off the Authorization header (the route is mounted without
		// JWTMiddleware). A missing/malformed/forged/non-impersonation
		// token is an authentication failure — 401, not the 422 reserved
		// for a validly-signed token with no active session.
		return NewUnauthorizedError(err)
	case errors.Is(err, services.ErrTenantMismatch):
		// #1749: an admin add-member request named a user whose tenant
		// differs from the group's tenant. A membership crosses no
		// tenant boundary, so this is a business-rule violation, not a
		// server bug. 422 + the JSON:API code lets the admin FE render
		// targeted copy ("that user belongs to a different tenant")
		// instead of a generic 422 toast. Extracted into a helper so
		// the switch stays under the gocyclo budget.
		return adminMemberTenantMismatchError(err)
	case errors.Is(err, services.ErrTooManyGroupMemberships):
		// Per-user group-membership cap reached (#1388). Same shape as
		// the other invite/membership business-rule violations below —
		// 422 with the sentinel message so the FE can render specific
		// copy and e2e assertions can match on status, not server bug
		// noise.
		return NewUnprocessableEntityError(err)
	case errors.Is(err, services.ErrInviteExpired),
		errors.Is(err, services.ErrInviteAlreadyUsed),
		errors.Is(err, services.ErrAlreadyMember),
		errors.Is(err, services.ErrInviteEmailMismatch):
		// Business-rule violations on the invite accept path: the token
		// is syntactically valid but cannot be redeemed right now.
		// Swagger on POST /invites/{token}/accept advertises 422 for
		// exactly these conditions; without this mapping they fall into
		// the default branch and surface as 500, which would mislead
		// clients (and e2e assertions) into treating them as server bugs.
		// The #1221 email-mismatch sentinel carries an extra JSON:API
		// `code` so the FE can render targeted copy ("this invite is
		// for a different email address") — handled by a helper that
		// keeps the wire shape sentinel-specific while collapsing the
		// switch arm so gocyclo stays under budget.
		return inviteAcceptRejectionError(err)
	case errors.Is(err, registry.ErrGroupCurrencyNotSet):
		return NewBadRequestError(err)
	case errors.Is(err, services.ErrRateLimitExceeded):
		return NewTooManyRequestsError(err)
	case errors.Is(err, services.ErrInvalidThumbnailSize):
		return NewBadRequestError(err)
	case errors.Is(err, registry.ErrResourceLimitExceeded):
		return NewTooManyRequestsError(err)
	case errors.Is(err, ErrMissingUploadSlot):
		return NewBadRequestError(err)
	case errors.Is(err, ErrInvalidUploadSlot):
		return NewBadRequestError(err)
	default:
		slog.Error("internal server error", "error", err)
		return NewInternalServerError(err)
	}
}

func renderEntityError(w http.ResponseWriter, r *http.Request, err error) error {
	return render.Render(w, r, jsonapi.NewErrors(toJSONAPIError(err)))
}

func badRequest(w http.ResponseWriter, r *http.Request, err error) error {
	badRequestError := jsonapi.Error{
		Err:            err,
		UserError:      errormarshal.Marshal(err),
		HTTPStatusCode: http.StatusBadRequest,
		StatusText:     "Bad Request",
	}
	return render.Render(w, r, jsonapi.NewErrors(badRequestError))
}

func notFound(w http.ResponseWriter, r *http.Request) error {
	notFoundError := jsonapi.Error{
		Err:            ErrEntityNotFound,
		UserError:      errormarshal.Marshal(ErrEntityNotFound),
		HTTPStatusCode: http.StatusNotFound,
		StatusText:     "Not Found",
	}
	return render.Render(w, r, jsonapi.NewErrors(notFoundError))
}

// codedNotFoundError renders a 404 with a JSON:API error code so the FE
// can distinguish "feature gated off in this deployment" from a generic
// missing resource. Used by the currency-migration featureGate when
// FeatureCurrencyMigration is false (#1616).
func codedNotFoundError(w http.ResponseWriter, r *http.Request, err error, code string) error {
	jsErr := jsonapi.Error{
		Err:            err,
		UserError:      errormarshal.Marshal(err),
		HTTPStatusCode: http.StatusNotFound,
		StatusText:     "Not Found",
		Code:           code,
	}
	return render.Render(w, r, jsonapi.NewErrors(jsErr))
}

func conflictError(w http.ResponseWriter, r *http.Request, err, userErr error) error {
	conflictErr := jsonapi.Error{
		Err:            err,
		UserError:      errormarshal.Marshal(userErr),
		HTTPStatusCode: http.StatusConflict,
		StatusText:     "Conflict",
	}
	return render.Render(w, r, jsonapi.NewErrors(conflictErr))
}

// codedConflictError renders a 409 with the given JSON:API error code
// and (optional) meta. Used by the currency-migration endpoints to
// distinguish migration_in_progress / restore_in_progress / preview_expired
// / state_changed at the wire level so the FE can render specific
// toasts.
func codedConflictError(w http.ResponseWriter, r *http.Request, err error, code string, meta map[string]any) error {
	jsErr := jsonapi.Error{
		Err:            err,
		UserError:      errormarshal.Marshal(err),
		HTTPStatusCode: http.StatusConflict,
		StatusText:     "Conflict",
		Code:           code,
		Meta:           meta,
	}
	return render.Render(w, r, jsonapi.NewErrors(jsErr))
}

// codedUnprocessableEntityError renders a 422 with a JSON:API error
// code. Used for currency_migration.token_invalid and same-currency
// rejections so the FE can branch on the code rather than the status
// alone.
func codedUnprocessableEntityError(w http.ResponseWriter, r *http.Request, err error, code string) error {
	jsErr := jsonapi.Error{
		Err:            err,
		UserError:      errormarshal.Marshal(err),
		HTTPStatusCode: http.StatusUnprocessableEntity,
		StatusText:     "Unprocessable Entity",
		Code:           code,
	}
	return render.Render(w, r, jsonapi.NewErrors(jsErr))
}

// codedTooManyRequestsError renders a 429 with a JSON:API error code
// and meta (typically {"retry_after_seconds": N}). Used for the
// currency-migration daily-cap rejections (#202 §3.5).
func codedTooManyRequestsError(w http.ResponseWriter, r *http.Request, err error, code string, meta map[string]any) error {
	jsErr := jsonapi.Error{
		Err:            err,
		UserError:      errormarshal.Marshal(err),
		HTTPStatusCode: http.StatusTooManyRequests,
		StatusText:     "Too Many Requests",
		Code:           code,
		Meta:           meta,
	}
	return render.Render(w, r, jsonapi.NewErrors(jsErr))
}

// lockedError renders a 423 Locked with a JSON:API error code and
// meta. Used by requireGroupNotMigrating to surface in-flight currency
// migrations on commodity write paths and on the restore-start
// endpoint (#202 §3.2).
func lockedError(w http.ResponseWriter, r *http.Request, err error, code string, meta map[string]any) error {
	jsErr := jsonapi.Error{
		Err:            err,
		UserError:      errormarshal.Marshal(err),
		HTTPStatusCode: http.StatusLocked,
		StatusText:     "Locked",
		Code:           code,
		Meta:           meta,
	}
	return render.Render(w, r, jsonapi.NewErrors(jsErr))
}

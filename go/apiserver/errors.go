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
		StatusText:     "Internal Server UserError",
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

func unprocessableEntityError(w http.ResponseWriter, r *http.Request, err error) error {
	return render.Render(w, r, jsonapi.NewErrors(NewUnprocessableEntityError(err)))
}

func toJSONAPIError(err error) jsonapi.Error {
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
		// with any caller that hasn't migrated.
		return NewUnprocessableEntityError(err)
	case errors.Is(err, services.ErrInviteNotByEmail):
		// Resending a legacy token-only invite (no captured email)
		// is a business-rule violation: 422, not 500.
		return NewUnprocessableEntityError(err)
	case errors.Is(err, services.ErrCommodityNotTrackable):
		// #1554: a bundle commodity (count > 1) cannot carry a per-
		// instance event (lend / service / warranty). Surface as 422
		// so the FE renders the same "split into separate items" hint
		// the create-form banner uses.
		return NewUnprocessableEntityError(err)
	case errors.Is(err, services.ErrInvalidConfirmation),
		errors.Is(err, services.ErrInvalidPassword):
		return NewUnprocessableEntityError(err)
	case errors.Is(err, services.ErrTooManyGroupMemberships):
		// Per-user group-membership cap reached (#1388). Same shape as
		// the other invite/membership business-rule violations below —
		// 422 with the sentinel message so the FE can render specific
		// copy and e2e assertions can match on status, not server bug
		// noise.
		return NewUnprocessableEntityError(err)
	case errors.Is(err, services.ErrInviteExpired),
		errors.Is(err, services.ErrInviteAlreadyUsed),
		errors.Is(err, services.ErrAlreadyMember):
		// Business-rule violations on the invite accept path: the token
		// is syntactically valid but cannot be redeemed right now.
		// Swagger on POST /invites/{token}/accept advertises 422 for
		// exactly these conditions; without this mapping they fall into
		// the default branch and surface as 500, which would mislead
		// clients (and e2e assertions) into treating them as server bugs.
		return NewUnprocessableEntityError(err)
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

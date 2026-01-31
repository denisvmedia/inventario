package apiserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/render"
	"github.com/go-extras/errx"
	errxjson "github.com/go-extras/errx/json"

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
	ErrNotFound               = registry.ErrNotFound
)

// marshalError marshals an error to JSON, ignoring any marshaling errors
func marshalError(err error) (result json.RawMessage) {
	defer func() {
		if r := recover(); r != nil {
			// If marshaling panics, return a simple error message
			result = json.RawMessage(`"error marshaling failed"`)
		}
	}()

	if data, e := errxjson.Marshal(err); e == nil {
		return data
	}
	// Fallback to simple error message if marshaling fails
	return json.RawMessage(`"error marshaling failed"`)
}

func NewNotFoundError(err error) jsonapi.Error {
	return jsonapi.Error{
		Err:            err,
		UserError:      marshalError(err),
		HTTPStatusCode: http.StatusNotFound,
		StatusText:     "Not Found",
	}
}

func NewUnprocessableEntityError(err error) jsonapi.Error {
	return jsonapi.Error{
		Err:            err,
		UserError:      marshalError(err),
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
		UserError:      marshalError(err),
		HTTPStatusCode: http.StatusBadRequest,
		StatusText:     "Bad Request",
	}
}

func NewTooManyRequestsError(err error) jsonapi.Error {
	return jsonapi.Error{
		Err:            err,
		UserError:      marshalError(err),
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
	case errors.Is(err, ErrNotFound):
		return NewNotFoundError(err)
	case errors.Is(err, registry.ErrMainCurrencyNotSet):
		return NewBadRequestError(err)
	case errors.Is(err, registry.ErrMainCurrencyAlreadySet):
		return NewUnprocessableEntityError(err)
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
		UserError:      marshalError(err),
		HTTPStatusCode: http.StatusBadRequest,
		StatusText:     "Bad Request",
	}
	return render.Render(w, r, jsonapi.NewErrors(badRequestError))
}

func notFound(w http.ResponseWriter, r *http.Request) error {
	notFoundError := jsonapi.Error{
		Err:            ErrEntityNotFound,
		UserError:      marshalError(ErrEntityNotFound),
		HTTPStatusCode: http.StatusNotFound,
		StatusText:     "Not Found",
	}
	return render.Render(w, r, jsonapi.NewErrors(notFoundError))
}

func conflictError(w http.ResponseWriter, r *http.Request, err, userErr error) error {
	conflictErr := jsonapi.Error{
		Err:            err,
		UserError:      marshalError(userErr),
		HTTPStatusCode: http.StatusConflict,
		StatusText:     "Conflict",
	}
	return render.Render(w, r, jsonapi.NewErrors(conflictErr))
}

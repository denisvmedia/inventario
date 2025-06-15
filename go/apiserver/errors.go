package apiserver

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/internal/log"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/registry"
)

var (
	ErrUnknownContentType = errors.New("render: unable to automatically decode the request content type")
	ErrInvalidContentType = errors.New("invalid content type")
	ErrNoFilesUploaded    = errors.New("no files uploaded")
	ErrEntityNotFound     = errors.New("entity not found")
)

func NewNotFoundError(err error) jsonapi.Error {
	return jsonapi.Error{
		Err:            err,
		UserError:      errkit.ForceMarshalError(err),
		HTTPStatusCode: http.StatusNotFound,
		StatusText:     "Not Found",
	}
}

func NewUnprocessableEntityError(err error) jsonapi.Error {
	return jsonapi.Error{
		Err:            err,
		UserError:      errkit.ForceMarshalError(err),
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

func internalServerError(w http.ResponseWriter, r *http.Request, err error) error {
	log.WithError(err).Error("internal server error")
	return render.Render(w, r, jsonapi.NewErrors(NewInternalServerError(err)))
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
	case errors.Is(err, registry.ErrMainCurrencyAlreadySet):
		return NewUnprocessableEntityError(err)
	default:
		log.WithError(err).Error("internal server error")
		return NewInternalServerError(err)
	}
}

func renderEntityError(w http.ResponseWriter, r *http.Request, err error) error {
	return render.Render(w, r, jsonapi.NewErrors(toJSONAPIError(err)))
}

func badRequest(w http.ResponseWriter, r *http.Request, err error) error {
	badRequestError := jsonapi.Error{
		Err:            err,
		UserError:      errkit.ForceMarshalError(err),
		HTTPStatusCode: http.StatusBadRequest,
		StatusText:     "Bad Request",
	}
	return render.Render(w, r, jsonapi.NewErrors(badRequestError))
}

func notFound(w http.ResponseWriter, r *http.Request) error {
	notFoundError := jsonapi.Error{
		Err:            ErrEntityNotFound,
		UserError:      errkit.ForceMarshalError(ErrEntityNotFound),
		HTTPStatusCode: http.StatusNotFound,
		StatusText:     "Not Found",
	}
	return render.Render(w, r, jsonapi.NewErrors(notFoundError))
}

func conflictError(w http.ResponseWriter, r *http.Request, err error) error {
	conflictErr := jsonapi.Error{
		Err:            err,
		UserError:      errkit.ForceMarshalError(err),
		HTTPStatusCode: http.StatusConflict,
		StatusText:     "Conflict",
	}
	return render.Render(w, r, jsonapi.NewErrors(conflictErr))
}

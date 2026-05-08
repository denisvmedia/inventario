package jsonapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/render"
)

// Error renderer type for handling all sorts of errors.
//
// In the best case scenario, the excellent github.com/pkg/errors package
// helps reveal information on the error, setting it on Err, and in the Render()
// method, using it to set the application-specific error code in AppCode.
type Error struct {
	Err            error `json:"-"` // low-level runtime error
	HTTPStatusCode int   `json:"-"` // http response status code

	StatusText string          `json:"status"`                               // user-level status message
	UserError  json.RawMessage `json:"error,omitempty" swaggertype:"object"` // user-level error message

	// Code is the application-specific error code (e.g.
	// "currency_migration.daily_cap_reached"). Optional; when present,
	// the FE branches on this string before falling back to the generic
	// HTTP status. Stable across versions.
	Code string `json:"code,omitempty"`

	// Meta is a free-form JSON object the handler may attach for
	// machine-readable context (e.g. {"retry_after_seconds": 3600}).
	// Optional; serialised only when non-empty.
	//
	// Swagger annotation note: `swaggertype:"object,string"` tells swag
	// to emit `additionalProperties: { type: string }`. Without an
	// explicit additionalProperties, openapi-typescript renders the
	// empty `{type: object}` schema as `Record<string, never>` — a
	// closed, indexable-but-empty object — which makes
	// `meta.retry_after_seconds` uncallable in TS without a cast. With
	// the override the FE gets a `{ [key: string]: string }` index
	// signature; consumers parse known keys (`retry_after_seconds:
	// number`, `migration_id: string`, `status: string`) into their
	// real types — we control both sides of that cast.
	Meta map[string]any `json:"meta,omitempty" swaggertype:"object,string"`
	// ErrorText string `json:"error,omitempty"` // application-level error message, for debugging
}

type Errors struct {
	HTTPStatusCode int     `json:"-"` // http response status code
	Errors         []Error `json:"errors"`
}

func NewErrors(errs ...Error) *Errors {
	return &Errors{
		Errors: errs,
	}
}

func (e *Errors) Render(_w http.ResponseWriter, r *http.Request) error {
	statusCode := e.HTTPStatusCode
	if e.HTTPStatusCode == 0 && len(e.Errors) != 0 {
		statusCode = e.Errors[0].HTTPStatusCode
	}

	render.Status(r, statusCodeDef(statusCode, http.StatusInternalServerError))
	return nil
}

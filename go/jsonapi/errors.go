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
	// AppCode    int64  `json:"code,omitempty"`  // application-specific error code
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

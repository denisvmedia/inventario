package apiserver_test

import (
	"errors"
	"net/http"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
)

// TestNewInternalServerError_StatusText guards against the typo fixed in
// #1655 ("Internal Server UserError" → "Internal Server Error") reappearing.
// The StatusText shows up in the JSON:API error envelope on every 500, gets
// logged in operator dashboards, and is also asserted by the frontend's
// global error toast — keeping it correct is a contract with both humans and
// machines.
func TestNewInternalServerError_StatusText(t *testing.T) {
	c := qt.New(t)

	e := apiserver.NewInternalServerError(errors.New("boom"))

	c.Assert(e.HTTPStatusCode, qt.Equals, http.StatusInternalServerError)
	c.Assert(e.StatusText, qt.Equals, "Internal Server Error")
}

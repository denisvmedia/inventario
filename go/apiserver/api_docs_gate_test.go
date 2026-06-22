package apiserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
)

// The GET /swagger/* API documentation UI is gated behind EnableAPIDocs
// (issue #2113, L-5): default on for dev/e2e, production sets it false so the
// API surface is not served publicly.

func TestAPIDocs_MountedWhenEnabled(t *testing.T) {
	c := qt.New(t)

	params, _, _ := newParams()
	params.EnableAPIDocs = true
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// The swagger handler serves an index redirect at /swagger/index.html.
	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Anything other than 404 proves the route is mounted (the swagger UI
	// returns 200/301/302 depending on the asset).
	c.Assert(rr.Code, qt.Not(qt.Equals), http.StatusNotFound)
}

func TestAPIDocs_NotMountedWhenDisabled(t *testing.T) {
	c := qt.New(t)

	params, _, _ := newParams()
	params.EnableAPIDocs = false
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestAPIDocs_DocJSONNotServedWhenDisabled(t *testing.T) {
	c := qt.New(t)

	params, _, _ := newParams()
	params.EnableAPIDocs = false
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

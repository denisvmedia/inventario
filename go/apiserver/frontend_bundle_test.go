//go:build with_frontend

package apiserver_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
)

// TestFrontendHandler_ServesLegacyByDefault verifies that the "legacy" bundle
// returns the Vue index.html (root mount point id="app") when requested.
//
// Like the other apiserver_test.* embed tests, this one needs the Vue bundle
// built first (npm run build in frontend/) and runs in CI under
// frontend-embed-smoke-test.
func TestFrontendHandler_ServesLegacyByDefault(t *testing.T) {
	c := qt.New(t)

	rec := getRoot(c, apiserver.FrontendHandler("legacy"))
	body := rec.Body.String()

	c.Assert(rec.Code, qt.Equals, http.StatusOK)
	c.Assert(strings.Contains(body, `id="app"`), qt.IsTrue,
		qt.Commentf(`legacy index.html should mount at <div id="app">; got: %s`, snippet(body)))
	c.Assert(strings.Contains(body, `id="root"`), qt.IsFalse,
		qt.Commentf("legacy bundle leaked the React mount id"))
}

// TestFrontendHandler_ServesReactBundle verifies that the "new" bundle
// returns the React index.html (mount point id="root").
//
// Needs the React bundle built first (npm run build in frontend-react/) and
// runs in CI under frontend-react-embed-smoke-test.
func TestFrontendHandler_ServesReactBundle(t *testing.T) {
	c := qt.New(t)

	rec := getRoot(c, apiserver.FrontendHandler("new"))
	body := rec.Body.String()

	c.Assert(rec.Code, qt.Equals, http.StatusOK)
	c.Assert(strings.Contains(body, `id="root"`), qt.IsTrue,
		qt.Commentf(`React index.html should mount at <div id="root">; got: %s`, snippet(body)))
	c.Assert(strings.Contains(body, `id="app"`), qt.IsFalse,
		qt.Commentf("React bundle leaked the Vue mount id"))
}

// TestFrontendHandler_UnknownBundleFallsBackToLegacy verifies the safety net
// in selectBundle: if validation is bypassed somehow, the handler still
// serves something rather than 500-ing.
func TestFrontendHandler_UnknownBundleFallsBackToLegacy(t *testing.T) {
	c := qt.New(t)

	rec := getRoot(c, apiserver.FrontendHandler("preact"))
	body := rec.Body.String()

	c.Assert(rec.Code, qt.Equals, http.StatusOK)
	c.Assert(strings.Contains(body, `id="app"`), qt.IsTrue,
		qt.Commentf("unknown bundle should fall back to legacy index.html"))
}

func getRoot(c *qt.C, h http.Handler) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rec, req)
	c.Logf("status=%d, content-type=%s", rec.Code, rec.Header().Get("Content-Type"))
	return rec
}

func snippet(s string) string {
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}

package apiserver_test

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/apiserver"
)

// The headers have to be on the router, not only on a handler: the SPA and
// every API route share it, and a middleware that is written but never
// registered is the failure this test exists to catch.
func TestSecurityHeaders_AreSetOnEveryResponse(t *testing.T) {
	c := qt.New(t)
	params, _, _ := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// An unauthenticated request: a 401 carries these headers too, which is
	// the case that matters — the browser applies them to whatever comes back.
	for _, path := range []string{"/api/v1/groups", "/some/spa/route"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		c.Assert(rr.Header().Get("X-Content-Type-Options"), qt.Equals, "nosniff",
			qt.Commentf("path %s", path))
		c.Assert(rr.Header().Get("X-Frame-Options"), qt.Equals, "DENY",
			qt.Commentf("path %s", path))
		c.Assert(rr.Header().Get("Referrer-Policy"), qt.Equals, "strict-origin-when-cross-origin",
			qt.Commentf("path %s", path))
	}
}

func TestSecurityHeaders_HSTSOnlyOverTLS(t *testing.T) {
	c := qt.New(t)
	handler := apiserver.SecurityHeaders()(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) },
	))

	// Plain HTTP: browsers would ignore the header anyway, and sending it
	// would pin localhost for anyone running the stack over HTTP.
	plain := httptest.NewRecorder()
	handler.ServeHTTP(plain, httptest.NewRequest(http.MethodGet, "/", nil))
	c.Assert(plain.Header().Get("Strict-Transport-Security"), qt.Equals, "")

	// Behind a TLS-terminating proxy.
	proxied := httptest.NewRequest(http.MethodGet, "/", nil)
	proxied.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, proxied)
	c.Assert(rr.Header().Get("Strict-Transport-Security"), qt.Equals,
		"max-age=31536000; includeSubDomains")

	// Direct TLS.
	direct := httptest.NewRequest(http.MethodGet, "https://example.test/", nil)
	direct.TLS = &tls.ConnectionState{}
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, direct)
	c.Assert(rr.Header().Get("Strict-Transport-Security"), qt.Equals,
		"max-age=31536000; includeSubDomains")
}

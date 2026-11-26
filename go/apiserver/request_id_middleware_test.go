package apiserver_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/appctx"
)

// requestIDResult is what one request through RequestIDMiddleware produced:
// the response header, and the id each context key carried to the handler.
type requestIDResult struct {
	header  string
	appctx  string
	chiCtx  string
	handled bool
}

// serveRequestID sends one request through the middleware. A nil supplied
// sends no X-Request-ID header at all.
func serveRequestID(supplied *string) requestIDResult {
	var res requestIDResult
	handler := apiserver.RequestIDMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res.handled = true
		res.appctx = appctx.RequestIDFromContext(r.Context())
		res.chiCtx = middleware.GetReqID(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
	if supplied != nil {
		req.Header.Set("X-Request-ID", *supplied)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	res.header = rr.Header().Get("X-Request-ID")
	return res
}

// assertOneID checks that the response header and both context keys agree,
// which is what lets the id a client reads back find the request's log lines
// and its audit rows.
func assertOneID(c *qt.C, res requestIDResult) {
	c.Helper()
	c.Assert(res.handled, qt.IsTrue)
	c.Assert(res.header, qt.Not(qt.Equals), "")
	c.Assert(res.appctx, qt.Equals, res.header)
	c.Assert(res.chiCtx, qt.Equals, res.header)
}

func TestRequestIDMiddleware_KeepsAValidIncomingID(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   string
	}{
		{"a plain token", "ticket-4417-attempt-2"},
		{"a UUID", "3f8a1c62-9d4e-4b17-8c2a-0f5b6d7e8a91"},
		{"chi's own format", "web-7d9f.example.com/Xb3kQ9pLmZ-000042"},
		{"base64 with padding", "q83vEjRWeJA+/w=="},
		{"a colon-separated trace id", "1-67891233:abcdef012345678912345678"},
		{"exactly at the cap", strings.Repeat("a", 128)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			res := serveRequestID(&tc.id)
			assertOneID(c, res)
			c.Assert(res.header, qt.Equals, tc.id)
		})
	}
}

func TestRequestIDMiddleware_ReplacesAnUnusableIncomingID(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   string
	}{
		{"one byte over the cap", strings.Repeat("a", 129)},
		{"a kilobyte", strings.Repeat("a", 1024)},
		{"a space", "abc def"},
		{"a newline", "abc\ndef"},
		{"a NUL", "abc\x00def"},
		{"a double quote", `abc"def`},
		{"markup", "<script>alert(1)</script>"},
		{"a header list separator", "abc,def"},
		{"a semicolon", "abc;def"},
		{"non-ASCII", "заявка-4417"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			res := serveRequestID(&tc.id)
			assertOneID(c, res)
			c.Assert(res.header, qt.Not(qt.Equals), tc.id)
			_, err := uuid.Parse(res.header)
			c.Assert(err, qt.IsNil, qt.Commentf("replacement %q is not a UUID", res.header))
		})
	}
}

func TestRequestIDMiddleware_GeneratesAnIDWhenNoneIsSent(t *testing.T) {
	c := qt.New(t)
	empty := ""

	first := serveRequestID(nil)
	assertOneID(c, first)
	_, err := uuid.Parse(first.header)
	c.Assert(err, qt.IsNil, qt.Commentf("generated %q is not a UUID", first.header))

	// An empty header is the same as none.
	second := serveRequestID(&empty)
	assertOneID(c, second)
	_, err = uuid.Parse(second.header)
	c.Assert(err, qt.IsNil, qt.Commentf("generated %q is not a UUID", second.header))

	c.Assert(first.header, qt.Not(qt.Equals), second.header)
}

// The middleware has to be on the router, ahead of everything that can answer
// on its own: a request refused before it reaches a handler is exactly the one
// whose id the caller needs for a report.
func TestAPIServer_ReturnsTheRequestIDOnEveryResponse(t *testing.T) {
	c := qt.New(t)
	params, _, _ := newParams()
	params.CORSConfig = apiserver.DefaultCORSConfig()
	params.CORSConfig.AllowedOrigins = []string{"https://inventario.example.com"}
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	for _, tc := range []struct {
		name   string
		method string
		path   string
		header map[string]string
		status int
	}{
		{"an unauthenticated request", http.MethodGet, "/api/v1/groups", nil, http.StatusUnauthorized},
		{"a request the tenant-header guard refuses", http.MethodGet, "/api/v1/groups",
			map[string]string{"X-Tenant-ID": "someone-else"}, http.StatusForbidden},
		{"a CORS preflight", http.MethodOptions, "/api/v1/groups", map[string]string{
			"Origin":                        "https://inventario.example.com",
			"Access-Control-Request-Method": http.MethodGet,
		}, http.StatusNoContent},
		{"the SPA", http.MethodGet, "/some/spa/route", nil, 0},
	} {
		c.Run(tc.name, func(c *qt.C) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			for k, v := range tc.header {
				req.Header.Set(k, v)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if tc.status != 0 {
				c.Assert(rr.Code, qt.Equals, tc.status, qt.Commentf(rr.Body.String()))
			}
			_, err := uuid.Parse(rr.Header().Get("X-Request-ID"))
			c.Assert(err, qt.IsNil, qt.Commentf("X-Request-ID %q", rr.Header().Get("X-Request-ID")))
		})
	}

	// A cross-origin frontend can read the header only if CORS exposes it.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups", nil)
	req.Header.Set("Origin", "https://inventario.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	exposed := strings.ToLower(rr.Header().Get("Access-Control-Expose-Headers"))
	c.Assert(exposed, qt.Contains, "x-request-id")
}

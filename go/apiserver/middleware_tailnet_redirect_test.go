package apiserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
)

func TestTailnetHostRedirect(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		suffix           string
		requestHost      string
		requestPath      string
		expectStatus     int
		expectLocation   string
		expectPassThru   bool // when true, the wrapped handler should run
	}{
		{
			name:           "no-op when suffix is empty",
			suffix:         "",
			requestHost:    "inv-vcl01-pr1881",
			requestPath:    "/",
			expectPassThru: true,
			expectStatus:   http.StatusOK,
		},
		{
			name:           "redirect short host to FQDN with HTTPS",
			suffix:         "<TAILNET>.ts.net",
			requestHost:    "inv-vcl01-pr1881",
			requestPath:    "/",
			expectStatus:   http.StatusMovedPermanently,
			expectLocation: "https://inv-vcl01-pr1881.<TAILNET>.ts.net/",
		},
		{
			name:           "preserve path + query when redirecting",
			suffix:         "<TAILNET>.ts.net",
			requestHost:    "inv-vcl01-pr1881",
			requestPath:    "/admin/users?page=2&sort=email",
			expectStatus:   http.StatusMovedPermanently,
			expectLocation: "https://inv-vcl01-pr1881.<TAILNET>.ts.net/admin/users?page=2&sort=email",
		},
		{
			name:           "host with port — strip port for match, redirect uses host only",
			suffix:         "<TAILNET>.ts.net",
			requestHost:    "inv-vcl01-pr1881:80",
			requestPath:    "/healthz",
			expectStatus:   http.StatusMovedPermanently,
			expectLocation: "https://inv-vcl01-pr1881.<TAILNET>.ts.net/healthz",
		},
		{
			name:           "pass-through when host already has the suffix",
			suffix:         "<TAILNET>.ts.net",
			requestHost:    "inv-vcl01-pr1881.<TAILNET>.ts.net",
			requestPath:    "/",
			expectPassThru: true,
			expectStatus:   http.StatusOK,
		},
		{
			name:           "pass-through when suffix match is case-insensitive",
			suffix:         "GIRAFFA-DUCK.TS.NET",
			requestHost:    "inv-vcl01-pr1881.<TAILNET>.ts.net",
			requestPath:    "/",
			expectPassThru: true,
			expectStatus:   http.StatusOK,
		},
		{
			name:           "pass-through on IPv4 host",
			suffix:         "<TAILNET>.ts.net",
			requestHost:    "100.85.83.119:443",
			requestPath:    "/",
			expectPassThru: true,
			expectStatus:   http.StatusOK,
		},
		{
			name:           "pass-through on IPv6 host",
			suffix:         "<TAILNET>.ts.net",
			requestHost:    "[fd7a:115c:a1e0::1]:443",
			requestPath:    "/",
			expectPassThru: true,
			expectStatus:   http.StatusOK,
		},
		{
			name:           "suffix with leading dot is normalized",
			suffix:         ".<TAILNET>.ts.net",
			requestHost:    "inv-vcl01-pr1881",
			requestPath:    "/",
			expectStatus:   http.StatusMovedPermanently,
			expectLocation: "https://inv-vcl01-pr1881.<TAILNET>.ts.net/",
		},
		{
			name:           "different short host gets redirected",
			suffix:         "<TAILNET>.ts.net",
			requestHost:    "inv-vcl01-master",
			requestPath:    "/api/v1/healthz",
			expectStatus:   http.StatusMovedPermanently,
			expectLocation: "https://inv-vcl01-master.<TAILNET>.ts.net/api/v1/healthz",
		},
		{
			name:           "empty host pass-through",
			suffix:         "<TAILNET>.ts.net",
			requestHost:    "",
			requestPath:    "/",
			expectPassThru: true,
			expectStatus:   http.StatusOK,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			passedThru := false
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				passedThru = true
				w.WriteHeader(http.StatusOK)
			})

			h := apiserver.TailnetHostRedirect(tc.suffix)(next)

			req := httptest.NewRequest(http.MethodGet, "http://example.invalid"+tc.requestPath, nil)
			req.Host = tc.requestHost
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			c.Assert(rec.Code, qt.Equals, tc.expectStatus,
				qt.Commentf("status code mismatch"))
			c.Assert(passedThru, qt.Equals, tc.expectPassThru,
				qt.Commentf("passThru expectation mismatch"))
			if tc.expectLocation != "" {
				c.Assert(rec.Header().Get("Location"), qt.Equals, tc.expectLocation,
					qt.Commentf("redirect Location mismatch"))
			}
		})
	}
}

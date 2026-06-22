package apiserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/services"
)

// GET /api/v1/invites/{token} is public (the invitee is usually
// unauthenticated). Issue #2113 (L-3) moved it inside the global per-IP
// rate-limit group so the token-lookup leg is rate-limited as defence-in-depth
// against invite-token enumeration.

func TestAPIServer_InvitesGoThroughGlobalRateLimit(t *testing.T) {
	c := qt.New(t)

	params, _, _ := newParamsAreaRegistryOnly()

	// Budget of 1 per hour: a single globally-limited request exhausts the IP.
	limiter := services.NewInMemoryGlobalRateLimiter(1, time.Hour)
	t.Cleanup(limiter.Stop)
	params.GlobalRateLimiter = limiter

	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	const clientIP = "203.0.113.77:1234"
	get := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = clientIP
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}

	// First invites GET consumes the single-request budget. The token is
	// unknown, so the handler returns a non-429 status (typically 404) — what
	// matters is that it is NOT throttled yet.
	first := get("/api/v1/invites/some-token")
	c.Assert(first.Code, qt.Not(qt.Equals), http.StatusTooManyRequests)

	// Second invites GET from the same IP: the global budget is exhausted, so
	// it must be throttled — proving the route sits inside the global limiter.
	second := get("/api/v1/invites/another-token")
	c.Assert(second.Code, qt.Equals, http.StatusTooManyRequests)
}

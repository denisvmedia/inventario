package apiserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// POST /backoffice/auth/refresh runs on the refresh cookie alone and bypasses
// the bearer-authenticated CSRF middleware. When the public host is configured
// it must reject cross-origin requests (issue #2113, L-1) as defence-in-depth
// on top of the cookie's SameSite=Strict attribute.

const backofficeL1PublicURL = "https://admin.example.com"

// newBackofficeAuthRouterWithPublicURL wires a back-office auth router with
// PublicURL set so the L-1 Origin/Referer check is active, and returns the
// router plus a valid refresh cookie obtained via login.
func newBackofficeAuthRouterWithPublicURL(t *testing.T) (http.Handler, *http.Cookie) {
	t.Helper()
	c := qt.New(t)

	bo := memory.NewBackofficeUserRegistry()
	rt := memory.NewBackofficeRefreshTokenRegistry()
	auditSvc := services.NewAuditService(memory.NewAuditLogRegistry())

	r := chi.NewRouter()
	r.Route("/", apiserver.BackofficeAuth(apiserver.BackofficeAuthParams{
		BackofficeUserRegistry:         bo,
		BackofficeRefreshTokenRegistry: rt,
		BlacklistService:               services.NewInMemoryTokenBlacklister(),
		RateLimiter:                    services.NewNoOpAuthRateLimiter(),
		AuditService:                   auditSvc,
		JWTSecret:                      backofficeTestSecret,
		PublicURL:                      backofficeL1PublicURL,
	}))

	seedBackofficeUser(t, bo, "ops@example.com", "S3cretPass!")

	body, _ := json.Marshal(apiserver.BackofficeLoginRequest{Email: "ops@example.com", Password: "S3cretPass!"})
	loginReq := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	r.ServeHTTP(loginRec, loginReq)
	c.Assert(loginRec.Code, qt.Equals, http.StatusOK)

	var refreshCookie *http.Cookie
	for _, cookie := range loginRec.Result().Cookies() {
		if cookie.Name == "backoffice_refresh_token" {
			refreshCookie = cookie
			break
		}
	}
	c.Assert(refreshCookie, qt.IsNotNil)
	return r, refreshCookie
}

func TestBackofficeAuth_Refresh_RejectsMissingOrigin(t *testing.T) {
	c := qt.New(t)
	router, cookie := newBackofficeAuthRouterWithPublicURL(t)

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req.AddCookie(cookie)
	// No Origin and no Referer header.
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusForbidden)
}

func TestBackofficeAuth_Refresh_RejectsForeignOrigin(t *testing.T) {
	c := qt.New(t)
	router, cookie := newBackofficeAuthRouterWithPublicURL(t)

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req.AddCookie(cookie)
	req.Header.Set("Origin", "https://evil.example.net")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusForbidden)
}

func TestBackofficeAuth_Refresh_AcceptsMatchingOrigin(t *testing.T) {
	c := qt.New(t)
	router, cookie := newBackofficeAuthRouterWithPublicURL(t)

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req.AddCookie(cookie)
	req.Header.Set("Origin", backofficeL1PublicURL)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusOK)
}

func TestBackofficeAuth_Refresh_AcceptsMatchingRefererFallback(t *testing.T) {
	c := qt.New(t)
	router, cookie := newBackofficeAuthRouterWithPublicURL(t)

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req.AddCookie(cookie)
	// No Origin — the handler falls back to Referer.
	req.Header.Set("Referer", backofficeL1PublicURL+"/admin/dashboard")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusOK)
}

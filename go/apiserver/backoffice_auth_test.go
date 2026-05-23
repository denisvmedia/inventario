package apiserver_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// backofficeTestSecret is the 32-byte signing secret every back-office
// auth test uses. Stays constant so a forged token in one test can be
// validated by another.
var backofficeTestSecret = []byte("backoffice-test-secret-32-bytes!")

// hashCookie hashes a raw refresh-token cookie value the same way the
// production code does so tests can assert against persisted rows by
// hash without depending on apiserver internals.
func hashCookie(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// newBackofficeAuthRouter wires a chi router with /login, /refresh,
// /logout and /me mounted at the root for tests. Uses the in-memory
// registries so tests exercise the real registry surface (including the
// FK + lowercase invariants Phase 1 covers).
//
// Returns the router, the back-office user registry, the refresh token
// registry, and the audit registry so tests can introspect side-effects.
func newBackofficeAuthRouter(t *testing.T) (http.Handler, *memory.BackofficeUserRegistry, *memory.BackofficeRefreshTokenRegistry, *memory.AuditLogRegistry) {
	t.Helper()

	bo := memory.NewBackofficeUserRegistry()
	rt := memory.NewBackofficeRefreshTokenRegistry()
	audit := memory.NewAuditLogRegistry()
	auditSvc := services.NewAuditService(audit)
	rateLimiter := services.NewNoOpAuthRateLimiter()
	blacklist := services.NewInMemoryTokenBlacklister()

	r := chi.NewRouter()
	r.Route("/", apiserver.BackofficeAuth(apiserver.BackofficeAuthParams{
		BackofficeUserRegistry:         bo,
		BackofficeRefreshTokenRegistry: rt,
		BlacklistService:               blacklist,
		RateLimiter:                    rateLimiter,
		AuditService:                   auditSvc,
		JWTSecret:                      backofficeTestSecret,
	}))

	return r, bo, rt, audit
}

// seedBackofficeUser inserts a back-office user with the supplied
// password (bcrypt-hashed). Returns the persisted row.
func seedBackofficeUser(t *testing.T, bo *memory.BackofficeUserRegistry, email, password string, opts ...func(*models.BackofficeUser)) *models.BackofficeUser {
	t.Helper()
	c := qt.New(t)

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	c.Assert(err, qt.IsNil)

	u := models.BackofficeUser{
		Email:        email,
		Name:         "Operator",
		PasswordHash: string(hash),
		Role:         models.BackofficeRolePlatformAdmin,
		IsActive:     true,
		MFAEnforced:  false,
	}
	for _, opt := range opts {
		opt(&u)
	}
	created, err := bo.Create(context.Background(), u)
	c.Assert(err, qt.IsNil)
	return created
}

func TestBackofficeAuth_Login_HappyPath(t *testing.T) {
	c := qt.New(t)
	router, bo, rt, audit := newBackofficeAuthRouter(t)
	user := seedBackofficeUser(t, bo, "ops@example.com", "S3cretPass!")

	body, _ := json.Marshal(apiserver.BackofficeLoginRequest{
		Email:    "ops@example.com",
		Password: "S3cretPass!",
	})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusOK)

	var resp apiserver.BackofficeLoginResponse
	c.Assert(json.Unmarshal(rec.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.AccessToken, qt.Not(qt.Equals), "")
	c.Assert(resp.TokenType, qt.Equals, "Bearer")
	c.Assert(resp.ExpiresIn > 0, qt.IsTrue)
	c.Assert(resp.User, qt.IsNotNil)
	c.Assert(resp.User.Email, qt.Equals, "ops@example.com")
	c.Assert(resp.User.Role, qt.Equals, string(models.BackofficeRolePlatformAdmin))

	// JWT claims should carry admin_id (not user_id) and aud=backoffice.
	parsed, err := jwt.Parse(resp.AccessToken, func(t *jwt.Token) (any, error) {
		return backofficeTestSecret, nil
	})
	c.Assert(err, qt.IsNil)
	c.Assert(parsed.Valid, qt.IsTrue)
	claims := parsed.Claims.(jwt.MapClaims)
	c.Assert(claims["admin_id"], qt.Equals, user.ID)
	c.Assert(claims["aud"], qt.Equals, "backoffice")
	c.Assert(claims["role"], qt.Equals, string(models.BackofficeRolePlatformAdmin))
	_, hasUserID := claims["user_id"]
	c.Assert(hasUserID, qt.IsFalse)

	// Refresh cookie set at the back-office path.
	var refreshCookie *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "backoffice_refresh_token" {
			refreshCookie = cookie
			break
		}
	}
	c.Assert(refreshCookie, qt.IsNotNil)
	c.Assert(refreshCookie.Path, qt.Equals, "/api/v1/backoffice")
	c.Assert(refreshCookie.HttpOnly, qt.IsTrue)
	c.Assert(refreshCookie.SameSite, qt.Equals, http.SameSiteStrictMode)
	c.Assert(refreshCookie.Value, qt.Not(qt.Equals), "")

	// Refresh-token row persisted with matching hash.
	stored, err := rt.GetByHash(context.Background(), hashCookie(refreshCookie.Value))
	c.Assert(err, qt.IsNil)
	c.Assert(stored.BackofficeUserID, qt.Equals, user.ID)

	// LastLoginAt stamped on the back-office user.
	fetched, err := bo.Get(context.Background(), user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.LastLoginAt, qt.IsNotNil)

	// Audit log records backoffice.login success.
	logs, err := audit.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(logs, qt.HasLen, 1)
	c.Assert(logs[0].Action, qt.Equals, "backoffice.login")
	c.Assert(logs[0].Success, qt.IsTrue)
	c.Assert(logs[0].UserID, qt.IsNotNil)
	c.Assert(*logs[0].UserID, qt.Equals, user.ID)
}

func TestBackofficeAuth_Login_WrongPassword(t *testing.T) {
	c := qt.New(t)
	router, bo, _, audit := newBackofficeAuthRouter(t)
	seedBackofficeUser(t, bo, "ops@example.com", "S3cretPass!")

	body, _ := json.Marshal(apiserver.BackofficeLoginRequest{
		Email:    "ops@example.com",
		Password: "nope",
	})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusUnauthorized)

	logs, err := audit.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(logs, qt.HasLen, 1)
	c.Assert(logs[0].Action, qt.Equals, "backoffice.login_failed")
	c.Assert(logs[0].Success, qt.IsFalse)
}

func TestBackofficeAuth_Login_NonexistentUser(t *testing.T) {
	c := qt.New(t)
	router, _, _, audit := newBackofficeAuthRouter(t)

	body, _ := json.Marshal(apiserver.BackofficeLoginRequest{
		Email:    "nobody@example.com",
		Password: "doesntmatter",
	})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusUnauthorized)

	logs, err := audit.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(logs, qt.HasLen, 1)
	c.Assert(logs[0].Action, qt.Equals, "backoffice.login_failed")
}

// TestBackofficeAuth_Login_InactiveAccount asserts the design-choice
// collapse: a disabled account returns the SAME 401 "Invalid credentials"
// body as a wrong-password attempt, so an attacker cannot enumerate
// which operator emails exist by probing the status code. The audit log
// still records the disabled-account distinction (action remains
// `backoffice.login_failed`, error message carries the
// ErrBackofficeAccountDisabled sentinel text) so platform admins can
// observe the real reason out-of-band.
func TestBackofficeAuth_Login_InactiveAccount(t *testing.T) {
	c := qt.New(t)
	router, bo, _, audit := newBackofficeAuthRouter(t)
	seedBackofficeUser(t, bo, "ops@example.com", "S3cretPass!", func(u *models.BackofficeUser) {
		u.IsActive = false
	})

	body, _ := json.Marshal(apiserver.BackofficeLoginRequest{
		Email:    "ops@example.com",
		Password: "S3cretPass!",
	})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusUnauthorized)

	logs, err := audit.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(logs, qt.HasLen, 1)
	c.Assert(logs[0].Action, qt.Equals, "backoffice.login_failed")
	c.Assert(logs[0].ErrorMessage, qt.IsNotNil)
	c.Assert(*logs[0].ErrorMessage, qt.Equals, apiserver.ErrBackofficeAccountDisabled.Error())
}

func TestBackofficeAuth_Login_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		body map[string]string
	}{
		{"missing password", map[string]string{"email": "ops@example.com"}},
		{"missing email", map[string]string{"password": "x"}},
		{"empty body", map[string]string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			router, _, _, _ := newBackofficeAuthRouter(t)

			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			c.Assert(rec.Code, qt.Equals, http.StatusBadRequest)
		})
	}
}

func TestBackofficeAuth_Refresh_HappyPath(t *testing.T) {
	c := qt.New(t)
	router, bo, _, _ := newBackofficeAuthRouter(t)
	seedBackofficeUser(t, bo, "ops@example.com", "S3cretPass!")

	// Login to obtain a refresh cookie.
	body, _ := json.Marshal(apiserver.BackofficeLoginRequest{Email: "ops@example.com", Password: "S3cretPass!"})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	c.Assert(rec.Code, qt.Equals, http.StatusOK)
	var refreshCookie *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "backoffice_refresh_token" {
			refreshCookie = cookie
			break
		}
	}
	c.Assert(refreshCookie, qt.IsNotNil)

	// Now refresh.
	refreshReq := httptest.NewRequest("POST", "/refresh", nil)
	refreshReq.AddCookie(refreshCookie)
	refreshRec := httptest.NewRecorder()
	router.ServeHTTP(refreshRec, refreshReq)

	c.Assert(refreshRec.Code, qt.Equals, http.StatusOK)
	var resp apiserver.BackofficeLoginResponse
	c.Assert(json.Unmarshal(refreshRec.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.AccessToken, qt.Not(qt.Equals), "")
	c.Assert(resp.User.Email, qt.Equals, "ops@example.com")
}

func TestBackofficeAuth_Refresh_NoCookie(t *testing.T) {
	c := qt.New(t)
	router, _, _, _ := newBackofficeAuthRouter(t)

	req := httptest.NewRequest("POST", "/refresh", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusUnauthorized)
}

func TestBackofficeAuth_Refresh_RevokedToken(t *testing.T) {
	c := qt.New(t)
	router, bo, rt, _ := newBackofficeAuthRouter(t)
	user := seedBackofficeUser(t, bo, "ops@example.com", "S3cretPass!")

	// Login.
	body, _ := json.Marshal(apiserver.BackofficeLoginRequest{Email: "ops@example.com", Password: "S3cretPass!"})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	c.Assert(rec.Code, qt.Equals, http.StatusOK)

	var refreshCookie *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "backoffice_refresh_token" {
			refreshCookie = cookie
		}
	}
	c.Assert(refreshCookie, qt.IsNotNil)

	// Revoke every refresh token for the user.
	c.Assert(rt.RevokeByBackofficeUserID(context.Background(), user.ID), qt.IsNil)

	refreshReq := httptest.NewRequest("POST", "/refresh", nil)
	refreshReq.AddCookie(refreshCookie)
	refreshRec := httptest.NewRecorder()
	router.ServeHTTP(refreshRec, refreshReq)

	c.Assert(refreshRec.Code, qt.Equals, http.StatusUnauthorized)
}

func TestBackofficeAuth_Refresh_ExpiredToken(t *testing.T) {
	c := qt.New(t)
	router, bo, rt, _ := newBackofficeAuthRouter(t)
	user := seedBackofficeUser(t, bo, "ops@example.com", "S3cretPass!")

	// Persist an already-expired refresh-token row directly so the
	// test doesn't have to wait for a real TTL.
	expired := models.BackofficeRefreshToken{
		BackofficeUserID: user.ID,
		TokenHash:        hashCookie("expired-raw-token"),
		ExpiresAt:        time.Now().Add(-time.Hour),
	}
	_, err := rt.Create(context.Background(), expired)
	c.Assert(err, qt.IsNil)

	req := httptest.NewRequest("POST", "/refresh", nil)
	// #nosec G124 -- test cookie attached to an httptest.Request; not transmitted over the wire.
	req.AddCookie(&http.Cookie{
		Name:     "backoffice_refresh_token",
		Value:    "expired-raw-token",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusUnauthorized)
}

func TestBackofficeAuth_Logout_RevokesRefreshToken(t *testing.T) {
	c := qt.New(t)
	router, bo, rt, _ := newBackofficeAuthRouter(t)
	user := seedBackofficeUser(t, bo, "ops@example.com", "S3cretPass!")

	// Login to obtain a session.
	body, _ := json.Marshal(apiserver.BackofficeLoginRequest{Email: "ops@example.com", Password: "S3cretPass!"})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	c.Assert(rec.Code, qt.Equals, http.StatusOK)

	var resp apiserver.BackofficeLoginResponse
	c.Assert(json.Unmarshal(rec.Body.Bytes(), &resp), qt.IsNil)
	var refreshCookie *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "backoffice_refresh_token" {
			refreshCookie = cookie
		}
	}
	c.Assert(refreshCookie, qt.IsNotNil)

	// Logout with both the cookie and the bearer access token.
	logoutReq := httptest.NewRequest("POST", "/logout", nil)
	logoutReq.AddCookie(refreshCookie)
	logoutReq.Header.Set("Authorization", "Bearer "+resp.AccessToken)
	logoutRec := httptest.NewRecorder()
	router.ServeHTTP(logoutRec, logoutReq)

	c.Assert(logoutRec.Code, qt.Equals, http.StatusOK)

	// Refresh-token row is now revoked.
	out, err := rt.ListActiveByBackofficeUserID(context.Background(), user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.HasLen, 0)

	// Cookie cleared (MaxAge negative).
	var clearedCookie *http.Cookie
	for _, cookie := range logoutRec.Result().Cookies() {
		if cookie.Name == "backoffice_refresh_token" {
			clearedCookie = cookie
		}
	}
	c.Assert(clearedCookie, qt.IsNotNil)
	c.Assert(clearedCookie.MaxAge < 0, qt.IsTrue)
}

func TestBackofficeAuth_Me_HappyPath(t *testing.T) {
	c := qt.New(t)
	router, bo, _, _ := newBackofficeAuthRouter(t)
	seedBackofficeUser(t, bo, "ops@example.com", "S3cretPass!")

	// Login.
	body, _ := json.Marshal(apiserver.BackofficeLoginRequest{Email: "ops@example.com", Password: "S3cretPass!"})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	c.Assert(rec.Code, qt.Equals, http.StatusOK)
	var resp apiserver.BackofficeLoginResponse
	c.Assert(json.Unmarshal(rec.Body.Bytes(), &resp), qt.IsNil)

	// GET /me.
	meReq := httptest.NewRequest("GET", "/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+resp.AccessToken)
	meRec := httptest.NewRecorder()
	router.ServeHTTP(meRec, meReq)

	c.Assert(meRec.Code, qt.Equals, http.StatusOK)
	var profile apiserver.BackofficeProfile
	c.Assert(json.Unmarshal(meRec.Body.Bytes(), &profile), qt.IsNil)
	c.Assert(profile.Email, qt.Equals, "ops@example.com")
	c.Assert(profile.Role, qt.Equals, string(models.BackofficeRolePlatformAdmin))
}

func TestBackofficeAuth_Me_NoToken(t *testing.T) {
	c := qt.New(t)
	router, _, _, _ := newBackofficeAuthRouter(t)

	req := httptest.NewRequest("GET", "/me", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusUnauthorized)
}

// TestBackofficeAuth_Login_RejectsMalformedEmail pins that a request body
// whose `email` is syntactically invalid is rejected at the registry
// validation step rather than slipping through to bcrypt + audit. The
// current handler only checks for empty strings up-front; a malformed
// email still maps to "user not found" (same 401 as a wrong-password
// attempt) because the registry's GetByEmail returns ErrBackofficeUserNotFound
// for any non-matching key. Pinning this stops a future "let's add
// strict email-syntax validation up-front" refactor from changing the
// wire shape and breaking the FE's error-handling.
func TestBackofficeAuth_Login_RejectsMalformedEmail(t *testing.T) {
	c := qt.New(t)
	router, bo, _, _ := newBackofficeAuthRouter(t)
	seedBackofficeUser(t, bo, "ops@example.com", "S3cretPass!")

	body, _ := json.Marshal(apiserver.BackofficeLoginRequest{
		Email:    "not-an-email",
		Password: "doesntmatter",
	})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Malformed email collapses into the same "Invalid credentials" 401
	// as a wrong-password / unknown-user attempt: the registry's
	// GetByEmail returns not-found for any non-matching key, including
	// syntactically invalid ones, and the handler treats not-found as
	// "wrong credentials" by design (to prevent enumeration).
	c.Assert(rec.Code, qt.Equals, http.StatusUnauthorized)
}

// TestBackofficeAuth_Login_CaseInsensitiveEmail pins the registry-layer
// lowercasing contract from the HTTP surface: seeding "ops@example.com"
// and authenticating as "OPS@Example.COM" must succeed. Otherwise a
// future "we lowercase emails but only on Create" refactor would silently
// break login for any operator typing the wrong case.
func TestBackofficeAuth_Login_CaseInsensitiveEmail(t *testing.T) {
	c := qt.New(t)
	router, bo, _, _ := newBackofficeAuthRouter(t)
	seedBackofficeUser(t, bo, "ops@example.com", "S3cretPass!")

	body, _ := json.Marshal(apiserver.BackofficeLoginRequest{
		Email:    "OPS@Example.COM",
		Password: "S3cretPass!",
	})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusOK)
	var resp apiserver.BackofficeLoginResponse
	c.Assert(json.Unmarshal(rec.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.User.Email, qt.Equals, "ops@example.com")
}

// TestBackofficeAuth_Login_RejectsMFAEnforced_UntilPhase4 pins the
// Phase-2 fail-closed behaviour: any back-office user with
// MFAEnforced=true must NOT be able to log in (the challenge flow lands
// in Phase 4). The schema + bootstrap default `mfa_enforced=false` so
// this branch is dormant in production, but if an operator manually
// flips the column to true (e.g. as part of a manual security
// readiness exercise) the login must return 501 with a typed body,
// NOT silently mint a fully-privileged token.
//
// Phase 4 will replace the 501 with a real challenge mint that returns
// the same response shape on the success path; this test should remain
// green but the wire status will become 200.
func TestBackofficeAuth_Login_RejectsMFAEnforced_UntilPhase4(t *testing.T) {
	c := qt.New(t)
	router, bo, _, audit := newBackofficeAuthRouter(t)
	seedBackofficeUser(t, bo, "ops@example.com", "S3cretPass!", func(u *models.BackofficeUser) {
		u.MFAEnforced = true
	})

	body, _ := json.Marshal(apiserver.BackofficeLoginRequest{
		Email:    "ops@example.com",
		Password: "S3cretPass!",
	})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusNotImplemented)

	var resp apiserver.BackofficeMFARequiredResponse
	c.Assert(json.Unmarshal(rec.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.MFARequired, qt.IsTrue)
	c.Assert(resp.Email, qt.Equals, "ops@example.com")
	c.Assert(resp.Code, qt.Equals, "backoffice.mfa_not_implemented")

	// Audit log records the MFA-required outcome so platform admins can
	// see operators tripping the fail-closed branch.
	logs, err := audit.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(logs, qt.HasLen, 1)
	c.Assert(logs[0].Action, qt.Equals, "backoffice.login_mfa_required")
	c.Assert(logs[0].Success, qt.IsFalse)
}

// TestBackofficeAuth_Logout_IgnoresTenantTokenInHeader pins the
// cross-plane revocation gadget fix: posting a valid tenant-shaped JWT
// (no `aud=backoffice`, no `admin_id`) at /backoffice/auth/logout must
// NOT blacklist the JWT's `jti` in the shared blacklist. Without this
// guard, an attacker holding a victim's tenant JWT could forcibly
// invalidate that tenant session via a back-office endpoint that runs
// without authentication. Audit log entry must also NOT include the
// tenant token's `user_id` as a back-office `admin_id`.
func TestBackofficeAuth_Logout_IgnoresTenantTokenInHeader(t *testing.T) {
	c := qt.New(t)
	bo := memory.NewBackofficeUserRegistry()
	rt := memory.NewBackofficeRefreshTokenRegistry()
	audit := memory.NewAuditLogRegistry()
	auditSvc := services.NewAuditService(audit)
	rateLimiter := services.NewNoOpAuthRateLimiter()
	blacklist := services.NewInMemoryTokenBlacklister()

	r := chi.NewRouter()
	r.Route("/", apiserver.BackofficeAuth(apiserver.BackofficeAuthParams{
		BackofficeUserRegistry:         bo,
		BackofficeRefreshTokenRegistry: rt,
		BlacklistService:               blacklist,
		RateLimiter:                    rateLimiter,
		AuditService:                   auditSvc,
		JWTSecret:                      backofficeTestSecret,
	}))

	// Mint a tenant-shaped token (no aud, user_id instead of admin_id)
	// signed with the SHARED secret. The signature is valid; the only
	// thing that protects us is the aud guard inside
	// parseBackofficeAccessTokenClaims.
	tenantJTI := "tenant-jti-victim-session"
	tenantClaims := jwt.MapClaims{
		"jti":        tenantJTI,
		"user_id":    "tenant-user-id",
		"token_type": "access",
		"exp":        time.Now().Add(15 * time.Minute).Unix(),
		"iat":        time.Now().Unix(),
		// Intentionally NO aud claim — mimics the historical tenant mint.
	}
	tenantToken := jwt.NewWithClaims(jwt.SigningMethodHS256, tenantClaims)
	tenantSigned, err := tenantToken.SignedString(backofficeTestSecret)
	c.Assert(err, qt.IsNil)

	// Post the tenant token at the back-office logout endpoint.
	logoutReq := httptest.NewRequest("POST", "/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+tenantSigned)
	logoutRec := httptest.NewRecorder()
	r.ServeHTTP(logoutRec, logoutReq)

	// Logout itself succeeds (the endpoint is intentionally
	// unauthenticated and idempotent) but the tenant jti MUST NOT be
	// in the blacklist.
	c.Assert(logoutRec.Code, qt.Equals, http.StatusOK)

	blacklisted, blErr := blacklist.IsBlacklisted(context.Background(), tenantJTI)
	c.Assert(blErr, qt.IsNil)
	c.Assert(blacklisted, qt.IsFalse)

	// And no audit log was written under a back-office admin_id (the
	// tenant token has none, so adminIDFromAccessTokenHeader returned
	// empty + the logout audit branch was skipped).
	logs, err := audit.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(logs, qt.HasLen, 0)
}

// TestBackofficeAuth_Logout_BlacklistsBackofficeAccessToken closes the
// coverage gap the reviewer flagged: logout must blacklist the access
// token's jti so a subsequent /me call with the same Bearer fails. The
// existing TestBackofficeAuth_Logout_RevokesRefreshToken only covers
// the refresh-token side of the contract.
func TestBackofficeAuth_Logout_BlacklistsBackofficeAccessToken(t *testing.T) {
	c := qt.New(t)
	router, bo, _, _ := newBackofficeAuthRouter(t)
	seedBackofficeUser(t, bo, "ops@example.com", "S3cretPass!")

	// 1. Login → obtain access token.
	body, _ := json.Marshal(apiserver.BackofficeLoginRequest{Email: "ops@example.com", Password: "S3cretPass!"})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	c.Assert(rec.Code, qt.Equals, http.StatusOK)
	var resp apiserver.BackofficeLoginResponse
	c.Assert(json.Unmarshal(rec.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.AccessToken, qt.Not(qt.Equals), "")

	// Sanity: /me works with this token before logout.
	meReq := httptest.NewRequest("GET", "/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+resp.AccessToken)
	meRec := httptest.NewRecorder()
	router.ServeHTTP(meRec, meReq)
	c.Assert(meRec.Code, qt.Equals, http.StatusOK)

	// 2. Logout with the bearer.
	logoutReq := httptest.NewRequest("POST", "/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+resp.AccessToken)
	logoutRec := httptest.NewRecorder()
	router.ServeHTTP(logoutRec, logoutReq)
	c.Assert(logoutRec.Code, qt.Equals, http.StatusOK)

	// 3. SAME bearer is now rejected.
	meReq2 := httptest.NewRequest("GET", "/me", nil)
	meReq2.Header.Set("Authorization", "Bearer "+resp.AccessToken)
	meRec2 := httptest.NewRecorder()
	router.ServeHTTP(meRec2, meReq2)
	c.Assert(meRec2.Code, qt.Equals, http.StatusUnauthorized)
}

// TestBackofficeAuth_Refresh_RotatesRefreshToken pins the rotation
// contract: every successful refresh revokes the consumed refresh-token
// row and issues a new one. The first call with the seed cookie must
// succeed and return a new cookie; the second call with the OLD cookie
// must return 401 (the old token hashes to a now-revoked row).
func TestBackofficeAuth_Refresh_RotatesRefreshToken(t *testing.T) {
	c := qt.New(t)
	router, bo, _, _ := newBackofficeAuthRouter(t)
	seedBackofficeUser(t, bo, "ops@example.com", "S3cretPass!")

	// Login → first refresh cookie.
	body, _ := json.Marshal(apiserver.BackofficeLoginRequest{Email: "ops@example.com", Password: "S3cretPass!"})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	c.Assert(rec.Code, qt.Equals, http.StatusOK)
	var originalCookie *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "backoffice_refresh_token" {
			originalCookie = cookie
		}
	}
	c.Assert(originalCookie, qt.IsNotNil)

	// First refresh: succeeds, returns a NEW cookie value.
	refreshReq := httptest.NewRequest("POST", "/refresh", nil)
	refreshReq.AddCookie(originalCookie)
	refreshRec := httptest.NewRecorder()
	router.ServeHTTP(refreshRec, refreshReq)
	c.Assert(refreshRec.Code, qt.Equals, http.StatusOK)

	var rotatedCookie *http.Cookie
	for _, cookie := range refreshRec.Result().Cookies() {
		if cookie.Name == "backoffice_refresh_token" {
			rotatedCookie = cookie
		}
	}
	c.Assert(rotatedCookie, qt.IsNotNil)
	c.Assert(rotatedCookie.Value, qt.Not(qt.Equals), originalCookie.Value)

	// Second refresh with the ORIGINAL cookie: must fail with 401
	// because the original row is now revoked. This is the
	// replay-after-rotation defence: a stolen cookie stays valid only
	// until the legitimate operator next refreshes.
	replayReq := httptest.NewRequest("POST", "/refresh", nil)
	replayReq.AddCookie(originalCookie)
	replayRec := httptest.NewRecorder()
	router.ServeHTTP(replayRec, replayReq)
	c.Assert(replayRec.Code, qt.Equals, http.StatusUnauthorized)

	// The rotated cookie still works (third call uses the new value).
	thirdReq := httptest.NewRequest("POST", "/refresh", nil)
	thirdReq.AddCookie(rotatedCookie)
	thirdRec := httptest.NewRecorder()
	router.ServeHTTP(thirdRec, thirdReq)
	c.Assert(thirdRec.Code, qt.Equals, http.StatusOK)
}

// TestBackofficeAuth_Refresh_RejectsTenantRefreshCookie pins the
// cross-plane cookie isolation: posting a `refresh_token=<value>`
// cookie (the tenant cookie name) at /backoffice/auth/refresh must
// return 401 "Refresh token required" because the back-office handler
// reads ONLY the `backoffice_refresh_token` cookie name. Without this
// guard a browser holding both a tenant and a back-office cookie could
// accidentally let one plane act on the other plane's session.
func TestBackofficeAuth_Refresh_RejectsTenantRefreshCookie(t *testing.T) {
	c := qt.New(t)
	router, _, _, _ := newBackofficeAuthRouter(t)

	req := httptest.NewRequest("POST", "/refresh", nil)
	// #nosec G124 -- test cookie attached to an httptest.Request; not transmitted over the wire.
	req.AddCookie(&http.Cookie{
		Name:     "refresh_token", // the TENANT cookie name
		Value:    "some-tenant-cookie-value",
		HttpOnly: true,
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusUnauthorized)
}

// TestBackofficeAuth_Login_RateLimited pins the per-email lockout
// contract: 5 consecutive failed logins against the same email trigger
// a lockout, and the 6th attempt returns 429 with a `Retry-After`
// header. Uses the in-memory rate limiter (production wires Redis or
// in-memory depending on config) so the test is deterministic without
// external state.
func TestBackofficeAuth_Login_RateLimited(t *testing.T) {
	c := qt.New(t)
	bo := memory.NewBackofficeUserRegistry()
	rt := memory.NewBackofficeRefreshTokenRegistry()
	audit := memory.NewAuditLogRegistry()
	auditSvc := services.NewAuditService(audit)
	// Use the REAL in-memory rate limiter (not no-op) so the lockout
	// branch in api.login fires.
	rateLimiter := services.NewInMemoryAuthRateLimiter()
	blacklist := services.NewInMemoryTokenBlacklister()

	r := chi.NewRouter()
	r.Route("/", apiserver.BackofficeAuth(apiserver.BackofficeAuthParams{
		BackofficeUserRegistry:         bo,
		BackofficeRefreshTokenRegistry: rt,
		BlacklistService:               blacklist,
		RateLimiter:                    rateLimiter,
		AuditService:                   auditSvc,
		JWTSecret:                      backofficeTestSecret,
	}))
	seedBackofficeUser(t, bo, "ops@example.com", "S3cretPass!")

	wrongBody, _ := json.Marshal(apiserver.BackofficeLoginRequest{
		Email:    "ops@example.com",
		Password: "wrong-password",
	})

	// 5 wrong-password attempts to exhaust the failed-login budget.
	for range 5 {
		req := httptest.NewRequest("POST", "/login", bytes.NewReader(wrongBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		c.Assert(rec.Code, qt.Equals, http.StatusUnauthorized)
	}

	// 6th attempt — even with the CORRECT password — must be locked out.
	rightBody, _ := json.Marshal(apiserver.BackofficeLoginRequest{
		Email:    "ops@example.com",
		Password: "S3cretPass!",
	})
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(rightBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	c.Assert(rec.Code, qt.Equals, http.StatusTooManyRequests)
	c.Assert(rec.Header().Get("Retry-After"), qt.Not(qt.Equals), "")
}

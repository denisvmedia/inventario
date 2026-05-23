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

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
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

	c.Assert(rec.Code, qt.Equals, http.StatusForbidden)

	logs, err := audit.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(logs, qt.HasLen, 1)
	c.Assert(logs[0].Action, qt.Equals, "backoffice.login_failed")
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

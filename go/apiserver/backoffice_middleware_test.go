package apiserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
)

// mintBackofficeTokenFor signs a back-office access token for the
// supplied admin_id with the canonical default claim set, overridden by
// `override`. Setting a key to nil in override deletes it (used to
// exercise "missing aud" / "missing admin_id" rejection paths).
func mintBackofficeTokenFor(t *testing.T, adminID string, override jwt.MapClaims) string {
	t.Helper()
	c := qt.New(t)
	claims := jwt.MapClaims{
		"jti":        "test-jti",
		"admin_id":   adminID,
		"role":       string(models.BackofficeRolePlatformAdmin),
		"aud":        "backoffice",
		"token_type": "access",
		"exp":        time.Now().Add(15 * time.Minute).Unix(),
		"iat":        time.Now().Unix(),
	}
	for k, v := range override {
		if v == nil {
			delete(claims, k)
			continue
		}
		claims[k] = v
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(backofficeTestSecret)
	c.Assert(err, qt.IsNil)
	return signed
}

// newRequireBackofficeAuthSetup builds a middleware + a registered
// active back-office user. Returns the middleware, registry, and the
// user's id so callers can mint matching tokens (the registry assigns
// a server-side id — there's no override knob and a test must not
// invent one).
func newRequireBackofficeAuthSetup(t *testing.T) (func(http.Handler) http.Handler, *memory.BackofficeUserRegistry, string) {
	t.Helper()
	c := qt.New(t)
	bo := memory.NewBackofficeUserRegistry()
	hash, err := bcrypt.GenerateFromPassword([]byte("ignored"), bcrypt.MinCost)
	c.Assert(err, qt.IsNil)
	created, err := bo.Create(context.Background(), models.BackofficeUser{
		Email:        "ops@example.com",
		Name:         "Operator",
		PasswordHash: string(hash),
		Role:         models.BackofficeRolePlatformAdmin,
		IsActive:     true,
	})
	c.Assert(err, qt.IsNil)

	mw := apiserver.RequireBackofficeAuth(backofficeTestSecret, bo, nil)
	return mw, bo, created.ID
}

func TestRequireBackofficeAuth_HappyPath(t *testing.T) {
	c := qt.New(t)
	mw, _, adminID := newRequireBackofficeAuthSetup(t)

	var captured *http.Request
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r
		w.WriteHeader(http.StatusOK)
	})
	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+mintBackofficeTokenFor(t, adminID, nil))
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusOK)
	c.Assert(captured, qt.IsNotNil)
	user := appctx.BackofficeUserFromContext(captured.Context())
	c.Assert(user, qt.IsNotNil)
	c.Assert(user.ID, qt.Equals, adminID)

	// MUST NOT leak into the tenant user context — the two universes
	// live on separate keys, so tenant code looking for a User finds
	// nothing.
	c.Assert(appctx.UserFromContext(captured.Context()), qt.IsNil)
}

func TestRequireBackofficeAuth_RejectsMissingAud(t *testing.T) {
	c := qt.New(t)
	mw, _, adminID := newRequireBackofficeAuthSetup(t)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := mw(handler)

	tok := mintBackofficeTokenFor(t, adminID, jwt.MapClaims{"aud": nil})
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusUnauthorized)
}

func TestRequireBackofficeAuth_RejectsTenantAud(t *testing.T) {
	c := qt.New(t)
	mw, _, adminID := newRequireBackofficeAuthSetup(t)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := mw(handler)

	tok := mintBackofficeTokenFor(t, adminID, jwt.MapClaims{"aud": "tenant"})
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusUnauthorized)
}

func TestRequireBackofficeAuth_RejectsMissingAdminID(t *testing.T) {
	c := qt.New(t)
	mw, _, adminID := newRequireBackofficeAuthSetup(t)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := mw(handler)

	tok := mintBackofficeTokenFor(t, adminID, jwt.MapClaims{"admin_id": nil})
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusUnauthorized)
}

// TestRequireBackofficeAuth_RejectsUserIDClaim asserts the paranoid
// guard: a token carrying user_id MUST NEVER satisfy the back-office
// middleware even if aud and admin_id are otherwise correct.
func TestRequireBackofficeAuth_RejectsUserIDClaim(t *testing.T) {
	c := qt.New(t)
	mw, _, adminID := newRequireBackofficeAuthSetup(t)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := mw(handler)

	tok := mintBackofficeTokenFor(t, adminID, jwt.MapClaims{"user_id": "user-123"})
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusUnauthorized)
}

func TestRequireBackofficeAuth_RejectsExpired(t *testing.T) {
	c := qt.New(t)
	mw, _, adminID := newRequireBackofficeAuthSetup(t)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := mw(handler)

	tok := mintBackofficeTokenFor(t, adminID, jwt.MapClaims{"exp": time.Now().Add(-time.Hour).Unix()})
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusUnauthorized)
}

func TestRequireBackofficeAuth_RejectsInactiveAccount(t *testing.T) {
	c := qt.New(t)
	mw, bo, adminID := newRequireBackofficeAuthSetup(t)
	c.Assert(bo.SetActive(context.Background(), adminID, false), qt.IsNil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := mw(handler)

	tok := mintBackofficeTokenFor(t, adminID, nil)
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusForbidden)
}

func TestRequireBackofficeAuth_RejectsMissingUser(t *testing.T) {
	c := qt.New(t)
	mw, _, _ := newRequireBackofficeAuthSetup(t)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := mw(handler)

	tok := mintBackofficeTokenFor(t, "no-such-admin", nil)
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusUnauthorized)
}

// TestJWTMiddleware_RejectsBackofficeToken pins the cross-plane guard
// on the TENANT middleware: a back-office token MUST never satisfy the
// tenant JWTMiddleware. This is the symmetric guard to the dedicated
// back-office middleware's checks above.
func TestJWTMiddleware_RejectsBackofficeToken(t *testing.T) {
	c := qt.New(t)

	tenantUser := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-123"},
			TenantID: "test-tenant-id",
		},
		Email:    "user@example.com",
		IsActive: true,
	}
	tenantReg := &mockUserRegistryForAuth{
		users: map[string]*models.User{"user-123": tenantUser},
	}

	mw := apiserver.JWTMiddleware(backofficeTestSecret, tenantReg, nil)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := mw(handler)

	// A token minted exactly as the back-office plane mints it.
	tok := mintBackofficeTokenFor(t, "admin-1", nil)
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusUnauthorized)
}

// TestJWTMiddleware_RejectsAdminIDClaim pins the paranoid guard on the
// tenant side: a token carrying admin_id (regardless of aud) MUST never
// satisfy the tenant middleware.
func TestJWTMiddleware_RejectsAdminIDClaim(t *testing.T) {
	c := qt.New(t)

	tenantReg := &mockUserRegistryForAuth{
		users: map[string]*models.User{},
	}
	mw := apiserver.JWTMiddleware(backofficeTestSecret, tenantReg, nil)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := mw(handler)

	// admin_id present, no aud (simulates a forged token that elides
	// the audience claim to slip past the aud guard).
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"jti":        "test-jti",
		"user_id":    "user-123",
		"admin_id":   "admin-1",
		"token_type": "access",
		"exp":        time.Now().Add(15 * time.Minute).Unix(),
		"iat":        time.Now().Unix(),
	})
	signed, err := token.SignedString(backofficeTestSecret)
	c.Assert(err, qt.IsNil)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusUnauthorized)
}

// TestCrossPlane_TenantTokenFailsBackofficeAuth pins the symmetric
// guard from the other direction: a normal tenant access token MUST
// NEVER satisfy RequireBackofficeAuth.
func TestCrossPlane_TenantTokenFailsBackofficeAuth(t *testing.T) {
	c := qt.New(t)
	mw, _, _ := newRequireBackofficeAuthSetup(t)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := mw(handler)

	// A tenant token: user_id, no admin_id, no aud (matches the
	// historical mint shape).
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"jti":        "tenant-jti",
		"user_id":    "user-123",
		"token_type": "access",
		"exp":        time.Now().Add(15 * time.Minute).Unix(),
		"iat":        time.Now().Unix(),
	})
	signed, err := token.SignedString(backofficeTestSecret)
	c.Assert(err, qt.IsNil)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	c.Assert(rec.Code, qt.Equals, http.StatusUnauthorized)
}

package apiserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// newPasswordResetRouter builds a chi router wired with PasswordReset routes.
func newPasswordResetRouter(params apiserver.PasswordResetParams) chi.Router {
	r := chi.NewRouter()
	r.Group(apiserver.PasswordReset(params))
	return r
}

// makePasswordResetUser creates a test user in the mock user registry.
func makePasswordResetUser() *models.User {
	u := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "pr-user-1"},
			TenantID: apiserver.DefaultTenantID,
		},
		Email:    "reset@example.com",
		Name:     "Reset User",
		Role:     models.UserRoleUser,
		IsActive: true,
	}
	if err := u.SetPassword("OldPassword123"); err != nil {
		panic(err)
	}
	return u
}

func TestHandleForgotPassword_AlwaysReturns200(t *testing.T) {
	t.Run("unknown email returns 200", func(t *testing.T) {
		c := qt.New(t)
		userReg := &mockUserRegistryForAuth{users: map[string]*models.User{}}
		r := newPasswordResetRouter(apiserver.PasswordResetParams{
			UserRegistry:          userReg,
			PasswordResetRegistry: memory.NewPasswordResetRegistry(),
			EmailService:          services.NewStubEmailService(),
		})
		body, _ := json.Marshal(map[string]string{"email": "nobody@example.com"})
		req := httptest.NewRequest(http.MethodPost, "/forgot-password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		c.Assert(w.Code, qt.Equals, http.StatusOK)
	})

	t.Run("known email returns 200", func(t *testing.T) {
		c := qt.New(t)
		user := makePasswordResetUser()
		userReg := &mockUserRegistryForAuth{users: map[string]*models.User{user.ID: user}}
		r := newPasswordResetRouter(apiserver.PasswordResetParams{
			UserRegistry:          userReg,
			PasswordResetRegistry: memory.NewPasswordResetRegistry(),
			EmailService:          services.NewStubEmailService(),
		})
		body, _ := json.Marshal(map[string]string{"email": user.Email})
		req := httptest.NewRequest(http.MethodPost, "/forgot-password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		c.Assert(w.Code, qt.Equals, http.StatusOK)
	})

	t.Run("empty email returns 400", func(t *testing.T) {
		c := qt.New(t)
		r := newPasswordResetRouter(apiserver.PasswordResetParams{
			UserRegistry:          &mockUserRegistryForAuth{users: map[string]*models.User{}},
			PasswordResetRegistry: memory.NewPasswordResetRegistry(),
			EmailService:          services.NewStubEmailService(),
		})
		body, _ := json.Marshal(map[string]string{"email": ""})
		req := httptest.NewRequest(http.MethodPost, "/forgot-password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		c.Assert(w.Code, qt.Equals, http.StatusBadRequest)
	})
}

func TestHandleForgotPassword_RateLimit(t *testing.T) {
	c := qt.New(t)
	userReg := &mockUserRegistryForAuth{users: map[string]*models.User{}}
	limiter := services.NewInMemoryAuthRateLimiter()
	r := newPasswordResetRouter(apiserver.PasswordResetParams{
		UserRegistry:          userReg,
		PasswordResetRegistry: memory.NewPasswordResetRegistry(),
		EmailService:          services.NewStubEmailService(),
		RateLimiter:           limiter,
	})

	makeReq := func() int {
		body, _ := json.Marshal(map[string]string{"email": "throttled@example.com"})
		req := httptest.NewRequest(http.MethodPost, "/forgot-password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	// First 3 requests should be allowed (limit is 3/hour).
	for i := 0; i < 3; i++ {
		c.Assert(makeReq(), qt.Equals, http.StatusOK)
	}
	// 4th should be rate-limited.
	c.Assert(makeReq(), qt.Equals, http.StatusTooManyRequests)
}

func TestHandleResetPassword(t *testing.T) {
	makeRouter := func(prReg *memory.PasswordResetRegistry, userReg *mockUserRegistryForAuth) chi.Router {
		return newPasswordResetRouter(apiserver.PasswordResetParams{
			UserRegistry:          userReg,
			PasswordResetRegistry: prReg,
			EmailService:          services.NewStubEmailService(),
			BlacklistService:      &mockTokenBlacklisterForAuth{},
		})
	}

	t.Run("valid token resets password", func(t *testing.T) {
		c := qt.New(t)
		user := makePasswordResetUser()
		userReg := &mockUserRegistryForAuth{users: map[string]*models.User{user.ID: user}}
		prReg := memory.NewPasswordResetRegistry()
		token, err := models.GeneratePasswordResetToken()
		c.Assert(err, qt.IsNil)
		_, err = prReg.Create(t.Context(), models.PasswordReset{
			UserID:    user.ID,
			TenantID:  apiserver.DefaultTenantID,
			Email:     user.Email,
			Token:     token,
			ExpiresAt: time.Now().Add(time.Hour),
		})
		c.Assert(err, qt.IsNil)

		r := makeRouter(prReg, userReg)
		body, _ := json.Marshal(map[string]string{"token": token, "new_password": "NewPassword456"})
		req := httptest.NewRequest(http.MethodPost, "/reset-password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		c.Assert(w.Code, qt.Equals, http.StatusOK)

		// Password must actually be updated.
		updated, _ := userReg.Get(t.Context(), user.ID)
		c.Assert(updated.CheckPassword("NewPassword456"), qt.IsTrue)
		c.Assert(updated.CheckPassword("OldPassword123"), qt.IsFalse)

		// Token must be consumed (deleted).
		_, err = prReg.GetByToken(t.Context(), token)
		c.Assert(err, qt.IsNotNil)
	})

	t.Run("invalid token returns 400", func(t *testing.T) {
		c := qt.New(t)
		r := makeRouter(memory.NewPasswordResetRegistry(), &mockUserRegistryForAuth{users: map[string]*models.User{}})
		body, _ := json.Marshal(map[string]string{"token": "no-such-token", "new_password": "NewPassword456"})
		req := httptest.NewRequest(http.MethodPost, "/reset-password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		c.Assert(w.Code, qt.Equals, http.StatusBadRequest)
	})

	t.Run("expired token returns 400", func(t *testing.T) {
		c := qt.New(t)
		user := makePasswordResetUser()
		userReg := &mockUserRegistryForAuth{users: map[string]*models.User{user.ID: user}}
		prReg := memory.NewPasswordResetRegistry()
		token, err := models.GeneratePasswordResetToken()
		c.Assert(err, qt.IsNil)
		_, err = prReg.Create(t.Context(), models.PasswordReset{
			UserID:    user.ID,
			TenantID:  apiserver.DefaultTenantID,
			Email:     user.Email,
			Token:     token,
			ExpiresAt: time.Now().Add(-time.Minute), // already expired
		})
		c.Assert(err, qt.IsNil)

		r := makeRouter(prReg, userReg)
		body, _ := json.Marshal(map[string]string{"token": token, "new_password": "NewPassword456"})
		req := httptest.NewRequest(http.MethodPost, "/reset-password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		c.Assert(w.Code, qt.Equals, http.StatusBadRequest)
	})

	t.Run("missing token returns 400", func(t *testing.T) {
		c := qt.New(t)
		r := makeRouter(memory.NewPasswordResetRegistry(), &mockUserRegistryForAuth{users: map[string]*models.User{}})
		body, _ := json.Marshal(map[string]string{"new_password": "NewPassword456"})
		req := httptest.NewRequest(http.MethodPost, "/reset-password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		c.Assert(w.Code, qt.Equals, http.StatusBadRequest)
	})

	t.Run("weak password returns 400", func(t *testing.T) {
		c := qt.New(t)
		user := makePasswordResetUser()
		userReg := &mockUserRegistryForAuth{users: map[string]*models.User{user.ID: user}}
		prReg := memory.NewPasswordResetRegistry()
		token, err := models.GeneratePasswordResetToken()
		c.Assert(err, qt.IsNil)
		_, err = prReg.Create(t.Context(), models.PasswordReset{
			UserID:    user.ID,
			TenantID:  apiserver.DefaultTenantID,
			Email:     user.Email,
			Token:     token,
			ExpiresAt: time.Now().Add(time.Hour),
		})
		c.Assert(err, qt.IsNil)

		r := makeRouter(prReg, userReg)
		body, _ := json.Marshal(map[string]string{"token": token, "new_password": "weak"})
		req := httptest.NewRequest(http.MethodPost, "/reset-password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		c.Assert(w.Code, qt.Equals, http.StatusBadRequest)
	})
}

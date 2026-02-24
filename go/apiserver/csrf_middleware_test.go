package apiserver_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/golang-jwt/jwt/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// errCSRFService is a CSRFService that always returns an error (to test fail-open behaviour).
type errCSRFService struct{}

func (errCSRFService) GenerateToken(_ context.Context, _ string) (string, error) {
	return "", errors.New("redis unavailable")
}

func (errCSRFService) GetToken(_ context.Context, _ string) (string, error) {
	return "", errors.New("redis unavailable")
}

func (errCSRFService) DeleteToken(_ context.Context, _ string) error {
	return errors.New("redis unavailable")
}

// makeCSRFTestUser creates a minimal active user and a signed JWT for it.
func makeCSRFTestUser(t *testing.T, jwtSecret []byte) (*models.User, string) {
	t.Helper()
	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "csrf-user-1"},
			TenantID: "tenant-1",
		},
		Email:    "csrf@example.com",
		Name:     "CSRF User",
		Role:     models.UserRoleUser,
		IsActive: true,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"role":    string(user.Role),
		"exp":     time.Now().Add(time.Hour).Unix(),
	})
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		t.Fatalf("Failed to sign test JWT: %v", err)
	}
	return user, tokenString
}

// makeCSRFRouter sets up a chi router with JWT auth + CSRF middleware wrapping a 200 OK handler.
func makeCSRFRouter(jwtSecret []byte, userRegistry registry.UserRegistry, csrfSvc services.CSRFService) http.Handler {
	authMiddleware := apiserver.JWTMiddleware(jwtSecret, userRegistry, nil)
	csrfMiddleware := apiserver.CSRFMiddleware(csrfSvc)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return authMiddleware(csrfMiddleware(inner))
}

func TestCSRFMiddleware_SafeMethodsBypass(t *testing.T) {
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	user, tokenString := makeCSRFTestUser(t, jwtSecret)

	csrfSvc := services.NewInMemoryCSRFService()
	defer csrfSvc.Stop()

	userReg := &mockUserRegistryForSecurityTests{
		users: map[string]*models.User{user.ID: user},
	}

	router := makeCSRFRouter(jwtSecret, userReg, csrfSvc)

	safeMethods := []string{http.MethodGet, http.MethodHead, http.MethodOptions}
	for _, method := range safeMethods {
		t.Run(method, func(t *testing.T) {
			c := qt.New(t)
			req := httptest.NewRequest(method, "/test", nil)
			req.Header.Set("Authorization", "Bearer "+tokenString)
			// No X-CSRF-Token header — should still succeed for safe methods.
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, http.StatusOK)
		})
	}
}

func TestCSRFMiddleware_ValidToken(t *testing.T) {
	c := qt.New(t)
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	user, tokenString := makeCSRFTestUser(t, jwtSecret)

	csrfSvc := services.NewInMemoryCSRFService()
	defer csrfSvc.Stop()

	// Pre-generate a CSRF token for the user.
	csrfToken, err := csrfSvc.GenerateToken(context.Background(), user.ID)
	c.Assert(err, qt.IsNil)

	userReg := &mockUserRegistryForSecurityTests{
		users: map[string]*models.User{user.ID: user},
	}

	router := makeCSRFRouter(jwtSecret, userReg, csrfSvc)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	req.Header.Set("X-CSRF-Token", csrfToken)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK)
}

func TestCSRFMiddleware_MissingToken(t *testing.T) {
	c := qt.New(t)
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	user, tokenString := makeCSRFTestUser(t, jwtSecret)

	csrfSvc := services.NewInMemoryCSRFService()
	defer csrfSvc.Stop()
	_, err := csrfSvc.GenerateToken(context.Background(), user.ID)
	c.Assert(err, qt.IsNil)

	userReg := &mockUserRegistryForSecurityTests{
		users: map[string]*models.User{user.ID: user},
	}

	router := makeCSRFRouter(jwtSecret, userReg, csrfSvc)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	// No X-CSRF-Token header.
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusForbidden)
}

func TestCSRFMiddleware_InvalidToken(t *testing.T) {
	c := qt.New(t)
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	user, tokenString := makeCSRFTestUser(t, jwtSecret)

	csrfSvc := services.NewInMemoryCSRFService()
	defer csrfSvc.Stop()
	_, err := csrfSvc.GenerateToken(context.Background(), user.ID)
	c.Assert(err, qt.IsNil)

	userReg := &mockUserRegistryForSecurityTests{
		users: map[string]*models.User{user.ID: user},
	}

	router := makeCSRFRouter(jwtSecret, userReg, csrfSvc)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	req.Header.Set("X-CSRF-Token", "this-is-the-wrong-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusForbidden)
}

func TestCSRFMiddleware_NoStoredToken(t *testing.T) {
	c := qt.New(t)
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	user, tokenString := makeCSRFTestUser(t, jwtSecret)

	csrfSvc := services.NewInMemoryCSRFService()
	defer csrfSvc.Stop()
	// Do NOT generate a token for this user — simulates a session that never logged in
	// through the CSRF-aware login endpoint.

	userReg := &mockUserRegistryForSecurityTests{
		users: map[string]*models.User{user.ID: user},
	}

	router := makeCSRFRouter(jwtSecret, userReg, csrfSvc)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	req.Header.Set("X-CSRF-Token", "any-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusForbidden)
}

func TestCSRFMiddleware_NilServiceDisablesCSRF(t *testing.T) {
	c := qt.New(t)
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	user, tokenString := makeCSRFTestUser(t, jwtSecret)

	userReg := &mockUserRegistryForSecurityTests{
		users: map[string]*models.User{user.ID: user},
	}

	// nil csrfService disables CSRF validation entirely.
	router := makeCSRFRouter(jwtSecret, userReg, nil)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	// No X-CSRF-Token — should be allowed when service is nil.
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK)
}

func TestCSRFMiddleware_ServiceErrorFailsOpen(t *testing.T) {
	c := qt.New(t)
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	user, tokenString := makeCSRFTestUser(t, jwtSecret)

	userReg := &mockUserRegistryForSecurityTests{
		users: map[string]*models.User{user.ID: user},
	}

	// Use the error service — GetToken will always return an error.
	router := makeCSRFRouter(jwtSecret, userReg, errCSRFService{})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	req.Header.Set("X-CSRF-Token", "some-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	// Fail-open: backend errors must not block valid requests.
	c.Assert(w.Code, qt.Equals, http.StatusOK)
}

func TestCSRFMiddleware_AllMutatingMethodsRequireToken(t *testing.T) {
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	user, tokenString := makeCSRFTestUser(t, jwtSecret)

	csrfSvc := services.NewInMemoryCSRFService()
	defer csrfSvc.Stop()
	csrfToken, err := csrfSvc.GenerateToken(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Failed to generate CSRF token: %v", err)
	}

	userReg := &mockUserRegistryForSecurityTests{
		users: map[string]*models.User{user.ID: user},
	}

	router := makeCSRFRouter(jwtSecret, userReg, csrfSvc)

	mutatingMethods := []string{
		http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete,
	}
	for _, method := range mutatingMethods {
		t.Run(method+" without token should fail", func(t *testing.T) {
			c := qt.New(t)
			req := httptest.NewRequest(method, "/test", nil)
			req.Header.Set("Authorization", "Bearer "+tokenString)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, http.StatusForbidden)
		})
		t.Run(method+" with valid token should succeed", func(t *testing.T) {
			c := qt.New(t)
			req := httptest.NewRequest(method, "/test", nil)
			req.Header.Set("Authorization", "Bearer "+tokenString)
			req.Header.Set("X-CSRF-Token", csrfToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, http.StatusOK)
		})
	}
}

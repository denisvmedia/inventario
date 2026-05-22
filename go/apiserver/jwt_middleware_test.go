package apiserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/golang-jwt/jwt/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
)

func TestJWTMiddleware(t *testing.T) {
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")

	// Create test user
	testUser := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-123"},
			TenantID: "test-tenant-id",
		},
		Email:    "test@example.com",
		Name:     "Test User",
		IsActive: true,
	}

	inactiveUser := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-456"},
			TenantID: "test-tenant-id",
		},
		Email:    "inactive@example.com",
		Name:     "Inactive User",
		IsActive: false,
	}

	userRegistry := &mockUserRegistryForAuth{
		users: map[string]*models.User{
			"user-123": testUser,
			"user-456": inactiveUser,
		},
	}

	// Helper function to create a valid JWT token. Carries token_type=access
	// to match real access tokens — validateJWTToken rejects tokens without
	// it (#1778).
	createToken := func(userID string) string {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id":    userID,
			"role":       "user",
			"token_type": "access",
			"exp":        time.Now().Add(24 * time.Hour).Unix(),
		})
		tokenString, _ := token.SignedString(jwtSecret)
		return tokenString
	}

	// Helper function to create an expired JWT token
	createExpiredToken := func(userID string) string {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id":    userID,
			"role":       "user",
			"token_type": "access",
			"exp":        time.Now().Add(-1 * time.Hour).Unix(), // Expired 1 hour ago
		})
		tokenString, _ := token.SignedString(jwtSecret)
		return tokenString
	}

	// Test successful authentication cases
	successTests := []struct {
		name         string
		setupRequest func(*http.Request)
		checkContext func(t *testing.T, r *http.Request)
	}{
		{
			name: "valid token with active user",
			setupRequest: func(req *http.Request) {
				token := createToken("user-123")
				req.Header.Set("Authorization", "Bearer "+token)
			},
			checkContext: func(t *testing.T, r *http.Request) {
				c := qt.New(t)
				user := appctx.UserFromContext(r.Context())
				c.Assert(user, qt.IsNotNil)
				c.Assert(user.ID, qt.Equals, "user-123")
				c.Assert(user.Email, qt.Equals, "test@example.com")
			},
		},
		{
			// #1778: an impersonation token issued before the token_type
			// change lacks the claim but carries imp=true. The explicit
			// imp allowance keeps such in-flight tokens working across a
			// deploy.
			name: "legacy impersonation token (imp=true, no token_type)",
			setupRequest: func(req *http.Request) {
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"user_id": "user-123",
					"imp":     true,
					"exp":     time.Now().Add(24 * time.Hour).Unix(),
				})
				tokenString, _ := token.SignedString(jwtSecret)
				req.Header.Set("Authorization", "Bearer "+tokenString)
			},
			checkContext: func(t *testing.T, r *http.Request) {
				c := qt.New(t)
				user := appctx.UserFromContext(r.Context())
				c.Assert(user, qt.IsNotNil)
				c.Assert(user.ID, qt.Equals, "user-123")
			},
		},
		{
			// Steady-state impersonation token: carries both imp=true and
			// token_type=access. Accepted via the token_type check.
			name: "impersonation token with imp=true and token_type=access",
			setupRequest: func(req *http.Request) {
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"user_id":    "user-123",
					"imp":        true,
					"token_type": "access",
					"exp":        time.Now().Add(24 * time.Hour).Unix(),
				})
				tokenString, _ := token.SignedString(jwtSecret)
				req.Header.Set("Authorization", "Bearer "+tokenString)
			},
			checkContext: func(t *testing.T, r *http.Request) {
				c := qt.New(t)
				user := appctx.UserFromContext(r.Context())
				c.Assert(user, qt.IsNotNil)
				c.Assert(user.ID, qt.Equals, "user-123")
			},
		},
	}

	// Test authentication failure cases
	failureTests := []struct {
		name           string
		setupRequest   func(*http.Request)
		expectedStatus int
	}{
		{
			name: "missing authorization header",
			setupRequest: func(req *http.Request) {
				// Don't set Authorization header
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "empty authorization header",
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "")
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "invalid bearer format",
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "InvalidFormat token")
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "invalid JWT token",
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer invalid.jwt.token")
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "expired JWT token",
			setupRequest: func(req *http.Request) {
				token := createExpiredToken("user-123")
				req.Header.Set("Authorization", "Bearer "+token)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "token with wrong secret",
			setupRequest: func(req *http.Request) {
				wrongSecret := []byte("wrong-secret")
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"user_id": "user-123",
					"role":    "user",
					"exp":     time.Now().Add(24 * time.Hour).Unix(),
				})
				tokenString, _ := token.SignedString(wrongSecret)
				req.Header.Set("Authorization", "Bearer "+tokenString)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "token without user_id claim",
			setupRequest: func(req *http.Request) {
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"role": "user",
					"exp":  time.Now().Add(24 * time.Hour).Unix(),
				})
				tokenString, _ := token.SignedString(jwtSecret)
				req.Header.Set("Authorization", "Bearer "+tokenString)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "token with non-existent user",
			setupRequest: func(req *http.Request) {
				token := createToken("non-existent-user")
				req.Header.Set("Authorization", "Bearer "+token)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "token with inactive user",
			setupRequest: func(req *http.Request) {
				token := createToken("user-456")
				req.Header.Set("Authorization", "Bearer "+token)
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			// #1778 regression test: the step-1 mfa_token is signed with
			// the same secret as access tokens. Replaying it verbatim as a
			// Bearer token must be rejected — otherwise an attacker with
			// only username+password (no TOTP code) bypasses the second
			// factor for the token's ~5-minute TTL.
			name: "mfa_token replayed as access token is rejected",
			setupRequest: func(req *http.Request) {
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"jti":        "mfa-jti",
					"user_id":    "user-123",
					"tenant_id":  "test-tenant-id",
					"token_type": "mfa_challenge",
					"exp":        time.Now().Add(5 * time.Minute).Unix(),
					"iat":        time.Now().Unix(),
				})
				tokenString, _ := token.SignedString(jwtSecret)
				req.Header.Set("Authorization", "Bearer "+tokenString)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			// A token missing the token_type claim entirely is rejected:
			// genuine access tokens always stamp it, so its absence means
			// the token was not minted for the access path (#1778).
			name: "token without token_type claim is rejected",
			setupRequest: func(req *http.Request) {
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"user_id": "user-123",
					"role":    "user",
					"exp":     time.Now().Add(24 * time.Hour).Unix(),
				})
				tokenString, _ := token.SignedString(jwtSecret)
				req.Header.Set("Authorization", "Bearer "+tokenString)
			},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	// Test successful authentication cases
	for _, tt := range successTests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Create middleware
			middleware := apiserver.JWTMiddleware(jwtSecret, userRegistry, nil)

			// Create test handler that captures the request
			var capturedRequest *http.Request
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedRequest = r
				w.WriteHeader(http.StatusOK)
			})

			// Wrap handler with middleware
			wrappedHandler := middleware(handler)

			// Create request
			req := httptest.NewRequest("GET", "/test", nil)
			tt.setupRequest(req)
			resp := httptest.NewRecorder()

			// Execute request
			wrappedHandler.ServeHTTP(resp, req)

			// Check status code
			c.Assert(resp.Code, qt.Equals, http.StatusOK)

			// Check context (always run for success tests)
			tt.checkContext(t, capturedRequest)
		})
	}

	// Test authentication failure cases
	for _, tt := range failureTests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Create middleware
			middleware := apiserver.JWTMiddleware(jwtSecret, userRegistry, nil)

			// Create test handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Wrap handler with middleware
			wrappedHandler := middleware(handler)

			// Create request
			req := httptest.NewRequest("GET", "/test", nil)
			tt.setupRequest(req)
			resp := httptest.NewRecorder()

			// Execute request
			wrappedHandler.ServeHTTP(resp, req)

			// Check status code
			c.Assert(resp.Code, qt.Equals, tt.expectedStatus)
		})
	}
}

func TestRequireAuth(t *testing.T) {
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{}}

	t.Run("RequireAuth is alias for JWTMiddleware", func(t *testing.T) {
		c := qt.New(t)

		// Create both middlewares
		jwtMiddleware := apiserver.JWTMiddleware(jwtSecret, userRegistry, nil)
		requireAuthMiddleware := apiserver.RequireAuth(jwtSecret, userRegistry, nil)

		// They should behave the same way (we can't directly compare functions,
		// but we can test that they both reject unauthorized requests)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// Test with no authorization header
		req := httptest.NewRequest("GET", "/test", nil)

		// Test JWT middleware
		resp1 := httptest.NewRecorder()
		jwtMiddleware(handler).ServeHTTP(resp1, req)

		// Test RequireAuth middleware
		resp2 := httptest.NewRecorder()
		requireAuthMiddleware(handler).ServeHTTP(resp2, req)

		// Both should return unauthorized
		c.Assert(resp1.Code, qt.Equals, http.StatusUnauthorized)
		c.Assert(resp2.Code, qt.Equals, http.StatusUnauthorized)
	})
}

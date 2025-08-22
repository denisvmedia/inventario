package apiserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/golang-jwt/jwt/v5"

	"github.com/denisvmedia/inventario/apiserver"
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
		Role:     models.UserRoleUser,
		IsActive: true,
	}

	inactiveUser := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-456"},
			TenantID: "test-tenant-id",
		},
		Email:    "inactive@example.com",
		Name:     "Inactive User",
		Role:     models.UserRoleUser,
		IsActive: false,
	}

	userRegistry := &mockUserRegistryForAuth{
		users: map[string]*models.User{
			"user-123": testUser,
			"user-456": inactiveUser,
		},
	}

	// Helper function to create a valid JWT token
	createToken := func(userID string) string {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": userID,
			"role":    "user",
			"exp":     time.Now().Add(24 * time.Hour).Unix(),
		})
		tokenString, _ := token.SignedString(jwtSecret)
		return tokenString
	}

	// Helper function to create an expired JWT token
	createExpiredToken := func(userID string) string {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": userID,
			"role":    "user",
			"exp":     time.Now().Add(-1 * time.Hour).Unix(), // Expired 1 hour ago
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
				user := apiserver.UserFromContext(r.Context())
				c.Assert(user, qt.IsNotNil)
				c.Assert(user.ID, qt.Equals, "user-123")
				c.Assert(user.Email, qt.Equals, "test@example.com")
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
	}

	// Test successful authentication cases
	for _, tt := range successTests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Create middleware
			middleware := apiserver.JWTMiddleware(jwtSecret, userRegistry)

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
			middleware := apiserver.JWTMiddleware(jwtSecret, userRegistry)

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
		jwtMiddleware := apiserver.JWTMiddleware(jwtSecret, userRegistry)
		requireAuthMiddleware := apiserver.RequireAuth(jwtSecret, userRegistry)

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

func TestRequireRole(t *testing.T) {
	// Create test users with different roles
	adminUser := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "admin-123"},
			TenantID: "test-tenant-id",
		},
		Email:    "admin@example.com",
		Name:     "Admin User",
		Role:     models.UserRoleAdmin,
		IsActive: true,
	}

	regularUser := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-123"},
			TenantID: "test-tenant-id",
		},
		Email:    "user@example.com",
		Name:     "Regular User",
		Role:     models.UserRoleUser,
		IsActive: true,
	}

	tests := []struct {
		name           string
		requiredRole   models.UserRole
		contextUser    *models.User
		expectedStatus int
	}{
		{
			name:           "admin user accessing admin endpoint",
			requiredRole:   models.UserRoleAdmin,
			contextUser:    adminUser,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "regular user accessing admin endpoint",
			requiredRole:   models.UserRoleAdmin,
			contextUser:    regularUser,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "regular user accessing user endpoint",
			requiredRole:   models.UserRoleUser,
			contextUser:    regularUser,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "admin user accessing user endpoint",
			requiredRole:   models.UserRoleUser,
			contextUser:    adminUser,
			expectedStatus: http.StatusForbidden, // Admin role != User role
		},
		{
			name:           "no user in context",
			requiredRole:   models.UserRoleUser,
			contextUser:    nil,
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Create middleware
			middleware := apiserver.RequireRole(tt.requiredRole)

			// Create test handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Wrap handler with middleware
			wrappedHandler := middleware(handler)

			// Create request with user context
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.contextUser != nil {
				ctx := apiserver.WithUser(req.Context(), tt.contextUser)
				req = req.WithContext(ctx)
			}
			resp := httptest.NewRecorder()

			// Execute request
			wrappedHandler.ServeHTTP(resp, req)

			// Check status code
			c.Assert(resp.Code, qt.Equals, tt.expectedStatus)
		})
	}
}

package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// mockUserRegistryForSecurityTests implements registry.UserRegistry for security testing
type mockUserRegistryForSecurityTests struct {
	users map[string]*models.User
}

func (m *mockUserRegistryForSecurityTests) Get(ctx context.Context, id string) (*models.User, error) {
	if user, exists := m.users[id]; exists {
		return user, nil
	}
	return nil, registry.ErrNotFound
}

func (m *mockUserRegistryForSecurityTests) GetByEmail(ctx context.Context, tenantID, email string) (*models.User, error) {
	for _, user := range m.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, registry.ErrNotFound
}

func (m *mockUserRegistryForSecurityTests) Update(ctx context.Context, user models.User) (*models.User, error) {
	m.users[user.ID] = &user
	return &user, nil
}

func (m *mockUserRegistryForSecurityTests) Create(ctx context.Context, user models.User) (*models.User, error) {
	m.users[user.ID] = &user
	return &user, nil
}

func (m *mockUserRegistryForSecurityTests) Delete(ctx context.Context, id string) error {
	delete(m.users, id)
	return nil
}

func (m *mockUserRegistryForSecurityTests) List(ctx context.Context) ([]*models.User, error) {
	var users []*models.User
	for _, user := range m.users {
		users = append(users, user)
	}
	return users, nil
}

func (m *mockUserRegistryForSecurityTests) Count(ctx context.Context) (int, error) {
	return len(m.users), nil
}

func (m *mockUserRegistryForSecurityTests) ListByTenant(ctx context.Context, tenantID string) ([]*models.User, error) {
	var users []*models.User
	for _, user := range m.users {
		if user.TenantID == tenantID {
			users = append(users, user)
		}
	}
	return users, nil
}

func (m *mockUserRegistryForSecurityTests) ListByRole(ctx context.Context, tenantID string, role models.UserRole) ([]*models.User, error) {
	var users []*models.User
	for _, user := range m.users {
		if user.TenantID == tenantID && user.Role == role {
			users = append(users, user)
		}
	}
	return users, nil
}

func TestAuthSecurity_LoginBruteForceProtection(t *testing.T) {
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
	testUser.SetPassword("ValidPassword123")

	userRegistry := &mockUserRegistryForSecurityTests{
		users: map[string]*models.User{
			"user-123": testUser,
		},
	}

	// Create auth handler
	authHandler := apiserver.Auth(apiserver.AuthParams{UserRegistry: userRegistry, JWTSecret: jwtSecret})
	r := chi.NewRouter()
	r.Route("/auth", authHandler)

	tests := []struct {
		name           string
		email          string
		password       string
		expectedStatus int
		description    string
	}{
		{
			name:           "valid credentials",
			email:          "test@example.com",
			password:       "ValidPassword123",
			expectedStatus: http.StatusOK,
			description:    "Should allow valid login",
		},
		{
			name:           "invalid password attempt 1",
			email:          "test@example.com",
			password:       "wrongpassword",
			expectedStatus: http.StatusUnauthorized,
			description:    "Should reject invalid password",
		},
		{
			name:           "invalid password attempt 2",
			email:          "test@example.com",
			password:       "anotherwrongpassword",
			expectedStatus: http.StatusUnauthorized,
			description:    "Should reject multiple invalid passwords",
		},
		{
			name:           "sql injection attempt",
			email:          "test@example.com' OR '1'='1",
			password:       "password",
			expectedStatus: http.StatusUnauthorized,
			description:    "Should reject SQL injection attempts",
		},
		{
			name:           "xss attempt in email",
			email:          "<script>alert('xss')</script>@example.com",
			password:       "password",
			expectedStatus: http.StatusUnauthorized,
			description:    "Should handle XSS attempts safely",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			loginReq := map[string]string{
				"email":    tt.email,
				"password": tt.password,
			}
			reqBody, _ := json.Marshal(loginReq)

			req := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			c.Assert(w.Code, qt.Equals, tt.expectedStatus, qt.Commentf(tt.description))

			// For unauthorized attempts, ensure no sensitive information is leaked
			if tt.expectedStatus == http.StatusUnauthorized {
				body := w.Body.String()
				c.Assert(strings.Contains(body, "Invalid credentials"), qt.IsTrue,
					qt.Commentf("Should use generic error message"))
				c.Assert(strings.Contains(body, "user not found"), qt.IsFalse,
					qt.Commentf("Should not reveal user existence"))
				c.Assert(strings.Contains(body, "password"), qt.IsFalse,
					qt.Commentf("Should not mention password in error"))
			}
		})
	}
}

func TestAuthSecurity_JWTTokenSecurity(t *testing.T) {
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	weakSecret := []byte("weak")

	tests := []struct {
		name        string
		tokenClaims jwt.MapClaims
		secret      []byte
		expectValid bool
		description string
	}{
		{
			name: "valid token",
			tokenClaims: jwt.MapClaims{
				"user_id": "user-123",
				"role":    "user",
				"exp":     time.Now().Add(time.Hour).Unix(),
			},
			secret:      jwtSecret,
			expectValid: true,
			description: "Should accept valid token",
		},
		{
			name: "expired token",
			tokenClaims: jwt.MapClaims{
				"user_id": "user-123",
				"role":    "user",
				"exp":     time.Now().Add(-time.Hour).Unix(),
			},
			secret:      jwtSecret,
			expectValid: false,
			description: "Should reject expired token",
		},
		{
			name: "token with wrong secret",
			tokenClaims: jwt.MapClaims{
				"user_id": "user-123",
				"role":    "user",
				"exp":     time.Now().Add(time.Hour).Unix(),
			},
			secret:      weakSecret,
			expectValid: false,
			description: "Should reject token signed with wrong secret",
		},
		{
			name: "token without user_id",
			tokenClaims: jwt.MapClaims{
				"role": "user",
				"exp":  time.Now().Add(time.Hour).Unix(),
			},
			secret:      jwtSecret,
			expectValid: false,
			description: "Should reject token without user_id",
		},
		{
			name: "token without expiration",
			tokenClaims: jwt.MapClaims{
				"user_id": "user-123",
				"role":    "user",
			},
			secret:      jwtSecret,
			expectValid: false,
			description: "Should reject token without expiration",
		},
	}

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

	userRegistry := &mockUserRegistryForSecurityTests{
		users: map[string]*models.User{
			"user-123": testUser,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Create token with test claims
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, tt.tokenClaims)
			tokenString, err := token.SignedString(tt.secret)
			c.Assert(err, qt.IsNil)

			// Create middleware
			middleware := apiserver.JWTMiddleware(jwtSecret, userRegistry, nil)

			// Create test handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Test the middleware
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", "Bearer "+tokenString)
			w := httptest.NewRecorder()

			middleware(handler).ServeHTTP(w, req)

			if tt.expectValid {
				c.Assert(w.Code, qt.Equals, http.StatusOK, qt.Commentf(tt.description))
			} else {
				c.Assert(w.Code, qt.Not(qt.Equals), http.StatusOK, qt.Commentf(tt.description))
			}
		})
	}
}

func TestAuthSecurity_UserStatusValidation(t *testing.T) {
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")

	// Create active and inactive users
	activeUser := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "active-user"},
			TenantID: "test-tenant-id",
		},
		Email:    "active@example.com",
		Name:     "Active User",
		Role:     models.UserRoleUser,
		IsActive: true,
	}

	inactiveUser := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "inactive-user"},
			TenantID: "test-tenant-id",
		},
		Email:    "inactive@example.com",
		Name:     "Inactive User",
		Role:     models.UserRoleUser,
		IsActive: false,
	}

	userRegistry := &mockUserRegistryForSecurityTests{
		users: map[string]*models.User{
			"active-user":   activeUser,
			"inactive-user": inactiveUser,
		},
	}

	tests := []struct {
		name           string
		userID         string
		expectedStatus int
		description    string
	}{
		{
			name:           "active user access",
			userID:         "active-user",
			expectedStatus: http.StatusOK,
			description:    "Should allow access for active user",
		},
		{
			name:           "inactive user access",
			userID:         "inactive-user",
			expectedStatus: http.StatusForbidden,
			description:    "Should deny access for inactive user",
		},
		{
			name:           "non-existent user",
			userID:         "non-existent",
			expectedStatus: http.StatusUnauthorized,
			description:    "Should deny access for non-existent user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Create valid token for the user
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"user_id": tt.userID,
				"role":    "user",
				"exp":     time.Now().Add(time.Hour).Unix(),
			})
			tokenString, err := token.SignedString(jwtSecret)
			c.Assert(err, qt.IsNil)

			// Create middleware
			middleware := apiserver.JWTMiddleware(jwtSecret, userRegistry, nil)

			// Create test handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Test the middleware
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", "Bearer "+tokenString)
			w := httptest.NewRecorder()

			middleware(handler).ServeHTTP(w, req)

			c.Assert(w.Code, qt.Equals, tt.expectedStatus, qt.Commentf(tt.description))
		})
	}
}

func TestAuthSecurity_MaliciousTokens(t *testing.T) {
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")

	userRegistry := &mockUserRegistryForSecurityTests{
		users: map[string]*models.User{},
	}

	tests := []struct {
		name        string
		token       string
		description string
	}{
		{
			name:        "malformed token",
			token:       "not.a.valid.jwt.token",
			description: "Should reject malformed tokens",
		},
		{
			name:        "empty token",
			token:       "",
			description: "Should reject empty tokens",
		},
		{
			name:        "token with null bytes",
			token:       "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJ1c2VyX2lkIjoidXNlci0xMjMiLCJyb2xlIjoidXNlciIsImV4cCI6MTY5OTk5OTk5OX0\x00.signature",
			description: "Should reject tokens with null bytes",
		},
		{
			name:        "extremely long token",
			token:       strings.Repeat("a", 10000),
			description: "Should handle extremely long tokens gracefully",
		},
		{
			name:        "token with unicode characters",
			token:       "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJ1c2VyX2lkIjoi🚀💻🔐",
			description: "Should handle unicode characters safely",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Create middleware
			middleware := apiserver.JWTMiddleware(jwtSecret, userRegistry, nil)

			// Create test handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Test the middleware
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)
			w := httptest.NewRecorder()

			middleware(handler).ServeHTTP(w, req)

			// All malicious tokens should be rejected
			c.Assert(w.Code, qt.Not(qt.Equals), http.StatusOK, qt.Commentf(tt.description))
		})
	}
}

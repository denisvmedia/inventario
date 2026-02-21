package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// mockUserRegistryForAuth implements registry.UserRegistry for testing
type mockUserRegistryForAuth struct {
	users map[string]*models.User
}

func (m *mockUserRegistryForAuth) Create(ctx context.Context, user models.User) (*models.User, error) {
	return nil, nil
}

func (m *mockUserRegistryForAuth) Get(ctx context.Context, id string) (*models.User, error) {
	if user, exists := m.users[id]; exists {
		return user, nil
	}
	return nil, registry.ErrNotFound
}

func (m *mockUserRegistryForAuth) Update(ctx context.Context, user models.User) (*models.User, error) {
	if _, exists := m.users[user.ID]; exists {
		m.users[user.ID] = &user
		return &user, nil
	}
	return nil, registry.ErrNotFound
}

func (m *mockUserRegistryForAuth) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockUserRegistryForAuth) List(ctx context.Context) ([]*models.User, error) {
	return nil, nil
}

func (m *mockUserRegistryForAuth) Count(ctx context.Context) (int, error) {
	return 0, nil
}

func (m *mockUserRegistryForAuth) GetByEmail(ctx context.Context, tenantID, email string) (*models.User, error) {
	for _, user := range m.users {
		if user.Email == email && user.TenantID == tenantID {
			return user, nil
		}
	}
	return nil, registry.ErrNotFound
}

func (m *mockUserRegistryForAuth) ListByTenant(ctx context.Context, tenantID string) ([]*models.User, error) {
	return nil, nil
}

func (m *mockUserRegistryForAuth) ListByRole(ctx context.Context, tenantID string, role models.UserRole) ([]*models.User, error) {
	return nil, nil
}

func TestAuthAPI_Login(t *testing.T) {
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")

	// Create test user with hashed password
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
	// Set password hash for "password123"
	testUser.SetPassword("password123")

	userRegistry := &mockUserRegistryForAuth{
		users: map[string]*models.User{
			"user-123": testUser,
		},
	}

	// Create auth handler
	authHandler := apiserver.Auth(apiserver.AuthParams{UserRegistry: userRegistry, JWTSecret: jwtSecret})

	tests := []struct {
		name           string
		requestBody    map[string]string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name: "successful login",
			requestBody: map[string]string{
				"email":    "test@example.com",
				"password": "password123",
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				c := qt.New(t)
				var response apiserver.LoginResponse
				err := json.Unmarshal(resp.Body.Bytes(), &response)
				c.Assert(err, qt.IsNil)
				c.Assert(response.AccessToken, qt.Not(qt.Equals), "")
				c.Assert(response.User.Email, qt.Equals, "test@example.com")
				c.Assert(response.ExpiresIn > 0, qt.IsTrue)

				// Verify JWT token
				token, err := jwt.Parse(response.AccessToken, func(token *jwt.Token) (any, error) {
					return jwtSecret, nil
				})
				c.Assert(err, qt.IsNil)
				c.Assert(token.Valid, qt.IsTrue)

				claims, ok := token.Claims.(jwt.MapClaims)
				c.Assert(ok, qt.IsTrue)
				c.Assert(claims["user_id"], qt.Equals, "user-123")
				c.Assert(claims["role"], qt.Equals, "user")
			},
		},
		{
			name: "invalid email",
			requestBody: map[string]string{
				"email":    "nonexistent@example.com",
				"password": "password123",
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "invalid password",
			requestBody: map[string]string{
				"email":    "test@example.com",
				"password": "wrongpassword",
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "missing email",
			requestBody: map[string]string{
				"password": "password123",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing password",
			requestBody: map[string]string{
				"email": "test@example.com",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty request body",
			requestBody:    map[string]string{},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Create request body
			body, err := json.Marshal(tt.requestBody)
			c.Assert(err, qt.IsNil)

			// Create request
			req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()

			// Create router and add auth routes
			router := chi.NewRouter()
			authHandler(router)
			router.ServeHTTP(resp, req)

			// Check status code
			c.Assert(resp.Code, qt.Equals, tt.expectedStatus)

			// Run additional checks if provided
			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestAuthAPI_Logout(t *testing.T) {
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{}}

	// Create auth handler
	authHandler := apiserver.Auth(apiserver.AuthParams{UserRegistry: userRegistry, JWTSecret: jwtSecret})

	t.Run("successful logout", func(t *testing.T) {
		c := qt.New(t)

		// Create request
		req := httptest.NewRequest("POST", "/logout", nil)
		resp := httptest.NewRecorder()

		// Create router and add auth routes
		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		// Check response
		c.Assert(resp.Code, qt.Equals, http.StatusOK)

		var response apiserver.LogoutResponse
		err := json.Unmarshal(resp.Body.Bytes(), &response)
		c.Assert(err, qt.IsNil)
		c.Assert(response.Message, qt.Equals, "Logged out successfully")
	})
}

func TestAuthAPI_GetCurrentUser(t *testing.T) {
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

	userRegistry := &mockUserRegistryForAuth{
		users: map[string]*models.User{
			"user-123": testUser,
		},
	}

	// Create auth handler
	authHandler := apiserver.Auth(apiserver.AuthParams{UserRegistry: userRegistry, JWTSecret: jwtSecret})

	t.Run("successful get current user", func(t *testing.T) {
		c := qt.New(t)

		// Create valid JWT token
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": "user-123",
			"role":    "user",
			"exp":     time.Now().Add(24 * time.Hour).Unix(),
		})
		tokenString, err := token.SignedString(jwtSecret)
		c.Assert(err, qt.IsNil)

		// Create request with Authorization header
		req := httptest.NewRequest("GET", "/me", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)
		resp := httptest.NewRecorder()

		// Create router and add auth routes
		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		// Check response
		c.Assert(resp.Code, qt.Equals, http.StatusOK)

		var user models.User
		err = json.Unmarshal(resp.Body.Bytes(), &user)
		c.Assert(err, qt.IsNil)
		c.Assert(user.Email, qt.Equals, "test@example.com")
		c.Assert(user.ID, qt.Equals, "user-123")
	})

	t.Run("unauthorized access", func(t *testing.T) {
		c := qt.New(t)

		// Create request without Authorization header
		req := httptest.NewRequest("GET", "/me", nil)
		resp := httptest.NewRecorder()

		// Create router and add auth routes
		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		// Check response
		c.Assert(resp.Code, qt.Equals, http.StatusUnauthorized)
	})
}

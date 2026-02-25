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

// mockRefreshTokenRegistryForAuth implements registry.RefreshTokenRegistry for testing.
// It records calls to RevokeByUserID so tests can assert it was invoked.
type mockRefreshTokenRegistryForAuth struct {
	revokeByUserIDCalled bool
	revokeByUserIDArg    string
}

func (m *mockRefreshTokenRegistryForAuth) Create(_ context.Context, _ models.RefreshToken) (*models.RefreshToken, error) {
	return nil, nil
}

func (m *mockRefreshTokenRegistryForAuth) Get(_ context.Context, _ string) (*models.RefreshToken, error) {
	return nil, registry.ErrNotFound
}

func (m *mockRefreshTokenRegistryForAuth) List(_ context.Context) ([]*models.RefreshToken, error) {
	return nil, nil
}

func (m *mockRefreshTokenRegistryForAuth) Update(_ context.Context, _ models.RefreshToken) (*models.RefreshToken, error) {
	return nil, nil
}

func (m *mockRefreshTokenRegistryForAuth) Delete(_ context.Context, _ string) error {
	return nil
}

func (m *mockRefreshTokenRegistryForAuth) Count(_ context.Context) (int, error) {
	return 0, nil
}

func (m *mockRefreshTokenRegistryForAuth) GetByTokenHash(_ context.Context, _ string) (*models.RefreshToken, error) {
	return nil, registry.ErrNotFound
}

func (m *mockRefreshTokenRegistryForAuth) GetByUserID(_ context.Context, _ string) ([]*models.RefreshToken, error) {
	return nil, nil
}

func (m *mockRefreshTokenRegistryForAuth) RevokeByUserID(_ context.Context, userID string) error {
	m.revokeByUserIDCalled = true
	m.revokeByUserIDArg = userID
	return nil
}

func (m *mockRefreshTokenRegistryForAuth) DeleteExpired(_ context.Context) error {
	return nil
}

// mockTokenBlacklisterForAuth implements services.TokenBlacklister for testing.
// It records calls to BlacklistUserTokens so tests can assert it was invoked.
type mockTokenBlacklisterForAuth struct {
	blacklistUserTokensCalled   bool
	blacklistUserTokensUserID   string
	blacklistUserTokensDuration time.Duration
}

func (m *mockTokenBlacklisterForAuth) BlacklistToken(_ context.Context, _ string, _ time.Time) error {
	return nil
}

func (m *mockTokenBlacklisterForAuth) IsBlacklisted(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (m *mockTokenBlacklisterForAuth) BlacklistUserTokens(_ context.Context, userID string, duration time.Duration) error {
	m.blacklistUserTokensCalled = true
	m.blacklistUserTokensUserID = userID
	m.blacklistUserTokensDuration = duration
	return nil
}

func (m *mockTokenBlacklisterForAuth) IsUserBlacklisted(_ context.Context, _ string) (bool, error) {
	return false, nil
}

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

func TestAuthAPI_ChangePassword(t *testing.T) {
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")

	setupUser := func(t *testing.T) *models.User {
		t.Helper()
		c := qt.New(t)
		user := &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "user-123"},
				TenantID: "test-tenant-id",
			},
			Email:    "test@example.com",
			Name:     "Test User",
			Role:     models.UserRoleUser,
			IsActive: true,
		}
		err := user.SetPassword("OldPassword123")
		c.Assert(err, qt.IsNil)
		return user
	}

	makeToken := func(t *testing.T) string {
		t.Helper()
		c := qt.New(t)
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": "user-123",
			"role":    "user",
			"exp":     time.Now().Add(24 * time.Hour).Unix(),
			"jti":     "test-change-pw-jti",
		})
		tokenString, err := token.SignedString(jwtSecret)
		c.Assert(err, qt.IsNil)
		return tokenString
	}

	makeRequest := func(t *testing.T, tokenString string, body any) (*http.Request, *httptest.ResponseRecorder) {
		t.Helper()
		c := qt.New(t)
		b, err := json.Marshal(body)
		c.Assert(err, qt.IsNil)
		req := httptest.NewRequest("POST", "/change-password", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		if tokenString != "" {
			req.Header.Set("Authorization", "Bearer "+tokenString)
		}
		return req, httptest.NewRecorder()
	}

	// Happy path
	t.Run("successful password change", func(t *testing.T) {
		c := qt.New(t)
		testUser := setupUser(t)
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{"user-123": testUser}}
		authHandler := apiserver.Auth(apiserver.AuthParams{UserRegistry: userRegistry, JWTSecret: jwtSecret})

		req, resp := makeRequest(t, makeToken(t), apiserver.ChangePasswordRequest{
			CurrentPassword: "OldPassword123",
			NewPassword:     "NewPassword456",
		})

		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		c.Assert(resp.Code, qt.Equals, http.StatusOK)

		// Verify the password was actually updated in the registry.
		updated, err := userRegistry.Get(context.Background(), "user-123")
		c.Assert(err, qt.IsNil)
		c.Assert(updated.CheckPassword("NewPassword456"), qt.IsTrue)
		c.Assert(updated.CheckPassword("OldPassword123"), qt.IsFalse)
	})

	// Unhappy paths
	t.Run("wrong current password", func(t *testing.T) {
		c := qt.New(t)
		testUser := setupUser(t)
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{"user-123": testUser}}
		authHandler := apiserver.Auth(apiserver.AuthParams{UserRegistry: userRegistry, JWTSecret: jwtSecret})

		req, resp := makeRequest(t, makeToken(t), apiserver.ChangePasswordRequest{
			CurrentPassword: "WrongPassword999",
			NewPassword:     "NewPassword456",
		})

		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		c.Assert(resp.Code, qt.Equals, http.StatusUnauthorized)
	})

	t.Run("new password fails complexity requirements", func(t *testing.T) {
		c := qt.New(t)
		testUser := setupUser(t)
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{"user-123": testUser}}
		authHandler := apiserver.Auth(apiserver.AuthParams{UserRegistry: userRegistry, JWTSecret: jwtSecret})

		req, resp := makeRequest(t, makeToken(t), apiserver.ChangePasswordRequest{
			CurrentPassword: "OldPassword123",
			NewPassword:     "alllowercase", // no uppercase, no digit
		})

		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		c.Assert(resp.Code, qt.Equals, http.StatusBadRequest)
	})

	t.Run("missing current password", func(t *testing.T) {
		c := qt.New(t)
		testUser := setupUser(t)
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{"user-123": testUser}}
		authHandler := apiserver.Auth(apiserver.AuthParams{UserRegistry: userRegistry, JWTSecret: jwtSecret})

		req, resp := makeRequest(t, makeToken(t), apiserver.ChangePasswordRequest{
			NewPassword: "NewPassword456",
		})

		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		c.Assert(resp.Code, qt.Equals, http.StatusBadRequest)
	})

	t.Run("missing new password", func(t *testing.T) {
		c := qt.New(t)
		testUser := setupUser(t)
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{"user-123": testUser}}
		authHandler := apiserver.Auth(apiserver.AuthParams{UserRegistry: userRegistry, JWTSecret: jwtSecret})

		req, resp := makeRequest(t, makeToken(t), apiserver.ChangePasswordRequest{
			CurrentPassword: "OldPassword123",
		})

		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		c.Assert(resp.Code, qt.Equals, http.StatusBadRequest)
	})

	t.Run("unauthenticated request", func(t *testing.T) {
		c := qt.New(t)
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{}}
		authHandler := apiserver.Auth(apiserver.AuthParams{UserRegistry: userRegistry, JWTSecret: jwtSecret})

		req, resp := makeRequest(t, "", apiserver.ChangePasswordRequest{
			CurrentPassword: "OldPassword123",
			NewPassword:     "NewPassword456",
		})

		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		c.Assert(resp.Code, qt.Equals, http.StatusUnauthorized)
	})

	t.Run("revokes tokens and blacklists user on success", func(t *testing.T) {
		c := qt.New(t)
		testUser := setupUser(t)
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{"user-123": testUser}}
		refreshRegistry := &mockRefreshTokenRegistryForAuth{}
		blacklister := &mockTokenBlacklisterForAuth{}
		authHandler := apiserver.Auth(apiserver.AuthParams{
			UserRegistry:         userRegistry,
			RefreshTokenRegistry: refreshRegistry,
			BlacklistService:     blacklister,
			JWTSecret:            jwtSecret,
		})

		req, resp := makeRequest(t, makeToken(t), apiserver.ChangePasswordRequest{
			CurrentPassword: "OldPassword123",
			NewPassword:     "NewPassword456",
		})

		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		c.Assert(resp.Code, qt.Equals, http.StatusOK)
		c.Assert(refreshRegistry.revokeByUserIDCalled, qt.IsTrue)
		c.Assert(refreshRegistry.revokeByUserIDArg, qt.Equals, "user-123")
		c.Assert(blacklister.blacklistUserTokensCalled, qt.IsTrue)
		c.Assert(blacklister.blacklistUserTokensUserID, qt.Equals, "user-123")
	})
}

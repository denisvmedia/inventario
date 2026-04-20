package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
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

func (m *mockTokenBlacklisterForAuth) UserBlacklistedSince(_ context.Context, _ string) (time.Time, bool, error) {
	return time.Time{}, false, nil
}

func (m *mockTokenBlacklisterForAuth) UnblacklistUser(_ context.Context, _ string) error {
	return nil
}

type mockEmailServiceForAuth struct {
	mu                   sync.Mutex
	passwordChangedCalls int
	passwordChangedCh    chan struct{}
}

func (m *mockEmailServiceForAuth) SendVerificationEmail(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

func (m *mockEmailServiceForAuth) SendPasswordResetEmail(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

func (m *mockEmailServiceForAuth) SendPasswordChangedEmail(_ context.Context, _ string, _ string, _ time.Time) error {
	m.mu.Lock()
	m.passwordChangedCalls++
	m.mu.Unlock()
	if m.passwordChangedCh != nil {
		select {
		case m.passwordChangedCh <- struct{}{}:
		default:
		}
	}
	return nil
}

func (m *mockEmailServiceForAuth) SendWelcomeEmail(_ context.Context, _ string, _ string) error {
	return nil
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
	users := make([]*models.User, 0, len(m.users))
	for _, user := range m.users {
		users = append(users, user)
	}
	return users, nil
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

// mockGroupMembershipRegistryForAuth satisfies registry.GroupMembershipRegistry for the
// default_group_id membership check (#1263). Only GetByGroupAndUser is exercised by the
// auth handler; the rest return zero values.
type mockGroupMembershipRegistryForAuth struct {
	members map[string]*models.GroupMembership // key: groupID|userID
}

func newMockGroupMembershipRegistryForAuth(pairs ...struct {
	groupID string
	userID  string
}) *mockGroupMembershipRegistryForAuth {
	m := &mockGroupMembershipRegistryForAuth{members: map[string]*models.GroupMembership{}}
	for _, p := range pairs {
		key := p.groupID + "|" + p.userID
		m.members[key] = &models.GroupMembership{
			TenantOnlyEntityID: models.TenantOnlyEntityID{
				EntityID: models.EntityID{ID: "membership-" + p.groupID + "-" + p.userID},
				TenantID: "test-tenant-id",
			},
			GroupID:      p.groupID,
			MemberUserID: p.userID,
			Role:         models.GroupRoleUser,
		}
	}
	return m
}

func (m *mockGroupMembershipRegistryForAuth) Create(_ context.Context, _ models.GroupMembership) (*models.GroupMembership, error) {
	return nil, nil
}

func (m *mockGroupMembershipRegistryForAuth) Get(_ context.Context, _ string) (*models.GroupMembership, error) {
	return nil, registry.ErrNotFound
}

func (m *mockGroupMembershipRegistryForAuth) List(_ context.Context) ([]*models.GroupMembership, error) {
	return nil, nil
}

func (m *mockGroupMembershipRegistryForAuth) Update(_ context.Context, _ models.GroupMembership) (*models.GroupMembership, error) {
	return nil, nil
}

func (m *mockGroupMembershipRegistryForAuth) Delete(_ context.Context, _ string) error {
	return nil
}

func (m *mockGroupMembershipRegistryForAuth) Count(_ context.Context) (int, error) {
	return 0, nil
}

func (m *mockGroupMembershipRegistryForAuth) GetByGroupAndUser(_ context.Context, groupID, userID string) (*models.GroupMembership, error) {
	if gm, ok := m.members[groupID+"|"+userID]; ok {
		return gm, nil
	}
	return nil, registry.ErrNotFound
}

func (m *mockGroupMembershipRegistryForAuth) ListByGroup(_ context.Context, _ string) ([]*models.GroupMembership, error) {
	return nil, nil
}

func (m *mockGroupMembershipRegistryForAuth) ListByUser(_ context.Context, _, _ string) ([]*models.GroupMembership, error) {
	return nil, nil
}

func (m *mockGroupMembershipRegistryForAuth) CountAdminsByGroup(_ context.Context, _ string) (int, error) {
	return 0, nil
}

// erroringGroupMembershipRegistry is a minimal GroupMembershipRegistry whose
// GetByGroupAndUser always returns a caller-supplied error. Used to exercise
// the "registry error ≠ ErrNotFound" branch that must surface as 500.
type erroringGroupMembershipRegistry struct {
	err error
}

func (m *erroringGroupMembershipRegistry) Create(_ context.Context, _ models.GroupMembership) (*models.GroupMembership, error) {
	return nil, nil
}

func (m *erroringGroupMembershipRegistry) Get(_ context.Context, _ string) (*models.GroupMembership, error) {
	return nil, registry.ErrNotFound
}

func (m *erroringGroupMembershipRegistry) List(_ context.Context) ([]*models.GroupMembership, error) {
	return nil, nil
}

func (m *erroringGroupMembershipRegistry) Update(_ context.Context, _ models.GroupMembership) (*models.GroupMembership, error) {
	return nil, nil
}

func (m *erroringGroupMembershipRegistry) Delete(_ context.Context, _ string) error {
	return nil
}

func (m *erroringGroupMembershipRegistry) Count(_ context.Context) (int, error) {
	return 0, nil
}

func (m *erroringGroupMembershipRegistry) GetByGroupAndUser(_ context.Context, _, _ string) (*models.GroupMembership, error) {
	return nil, m.err
}

func (m *erroringGroupMembershipRegistry) ListByGroup(_ context.Context, _ string) ([]*models.GroupMembership, error) {
	return nil, nil
}

func (m *erroringGroupMembershipRegistry) ListByUser(_ context.Context, _, _ string) ([]*models.GroupMembership, error) {
	return nil, nil
}

func (m *erroringGroupMembershipRegistry) CountAdminsByGroup(_ context.Context, _ string) (int, error) {
	return 0, nil
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

	// Tenant injected via middleware (normally done by PublicTenantMiddleware in APIServer).
	loginTenant := &models.Tenant{
		EntityID: models.EntityID{ID: "test-tenant-id"},
		Status:   models.TenantStatusActive,
	}

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

			// Create router and add auth routes, injecting tenant context.
			router := chi.NewRouter()
			router.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ctx := apiserver.WithTenant(r.Context(), loginTenant)
					next.ServeHTTP(w, r.WithContext(ctx))
				})
			})
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

func TestAuthAPI_UpdateCurrentUser(t *testing.T) {
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")

	setupUser := func(t *testing.T) *models.User {
		t.Helper()
		return &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "user-123"},
				TenantID: "test-tenant-id",
			},
			Email:    "test@example.com",
			Name:     "Original Name",
			IsActive: true,
		}
	}

	makeToken := func(t *testing.T) string {
		t.Helper()
		c := qt.New(t)
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": "user-123",
			"role":    "user",
			"exp":     time.Now().Add(24 * time.Hour).Unix(),
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
		req := httptest.NewRequest("PUT", "/me", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		if tokenString != "" {
			req.Header.Set("Authorization", "Bearer "+tokenString)
		}
		return req, httptest.NewRecorder()
	}

	t.Run("successful name update", func(t *testing.T) {
		c := qt.New(t)
		testUser := setupUser(t)
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{"user-123": testUser}}
		authHandler := apiserver.Auth(apiserver.AuthParams{UserRegistry: userRegistry, JWTSecret: jwtSecret})

		req, resp := makeRequest(t, makeToken(t), jsonapi.UpdateProfileRequest{Name: "New Name"})

		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		c.Assert(resp.Code, qt.Equals, http.StatusOK)

		var updated models.User
		err := json.Unmarshal(resp.Body.Bytes(), &updated)
		c.Assert(err, qt.IsNil)
		c.Assert(updated.Name, qt.Equals, "New Name")
		// Email and is_active must remain unchanged
		c.Assert(updated.Email, qt.Equals, "test@example.com")

		// Verify the registry was actually updated
		stored, err := userRegistry.Get(context.Background(), "user-123")
		c.Assert(err, qt.IsNil)
		c.Assert(stored.Name, qt.Equals, "New Name")
	})

	t.Run("name is trimmed of whitespace", func(t *testing.T) {
		c := qt.New(t)
		testUser := setupUser(t)
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{"user-123": testUser}}
		authHandler := apiserver.Auth(apiserver.AuthParams{UserRegistry: userRegistry, JWTSecret: jwtSecret})

		req, resp := makeRequest(t, makeToken(t), jsonapi.UpdateProfileRequest{Name: "  Trimmed Name  "})

		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		c.Assert(resp.Code, qt.Equals, http.StatusOK)

		var updated models.User
		err := json.Unmarshal(resp.Body.Bytes(), &updated)
		c.Assert(err, qt.IsNil)
		c.Assert(updated.Name, qt.Equals, "Trimmed Name")
	})

	t.Run("blank name is rejected", func(t *testing.T) {
		c := qt.New(t)
		testUser := setupUser(t)
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{"user-123": testUser}}
		authHandler := apiserver.Auth(apiserver.AuthParams{UserRegistry: userRegistry, JWTSecret: jwtSecret})

		req, resp := makeRequest(t, makeToken(t), jsonapi.UpdateProfileRequest{Name: "   "})

		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		c.Assert(resp.Code, qt.Equals, http.StatusBadRequest)
	})

	t.Run("name exceeding 100 chars is rejected", func(t *testing.T) {
		c := qt.New(t)
		testUser := setupUser(t)
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{"user-123": testUser}}
		authHandler := apiserver.Auth(apiserver.AuthParams{UserRegistry: userRegistry, JWTSecret: jwtSecret})

		longName := strings.Repeat("a", 101)
		req, resp := makeRequest(t, makeToken(t), jsonapi.UpdateProfileRequest{Name: longName})

		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		c.Assert(resp.Code, qt.Equals, http.StatusBadRequest)
	})

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		c := qt.New(t)
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{}}
		authHandler := apiserver.Auth(apiserver.AuthParams{UserRegistry: userRegistry, JWTSecret: jwtSecret})

		req, resp := makeRequest(t, "", jsonapi.UpdateProfileRequest{Name: "New Name"})

		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		c.Assert(resp.Code, qt.Equals, http.StatusUnauthorized)
	})

	t.Run("submitted email and role fields are ignored", func(t *testing.T) {
		c := qt.New(t)
		testUser := setupUser(t)
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{"user-123": testUser}}
		authHandler := apiserver.Auth(apiserver.AuthParams{UserRegistry: userRegistry, JWTSecret: jwtSecret})

		// Submit a body that includes extra fields alongside name — only name should be used.
		body := map[string]string{
			"name":      "Legit Name",
			"email":     "hacker@evil.com",
			"role":      "admin",
			"tenant_id": "other-tenant",
		}
		b, err := json.Marshal(body)
		c.Assert(err, qt.IsNil)
		req := httptest.NewRequest("PUT", "/me", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+makeToken(t))
		resp := httptest.NewRecorder()

		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		c.Assert(resp.Code, qt.Equals, http.StatusOK)

		var updated models.User
		err = json.Unmarshal(resp.Body.Bytes(), &updated)
		c.Assert(err, qt.IsNil)
		c.Assert(updated.Name, qt.Equals, "Legit Name")
		c.Assert(updated.Email, qt.Equals, "test@example.com")

		// TenantID is not serialized (json:"-") so we verify it was preserved in the registry.
		stored, storedErr := userRegistry.Get(context.Background(), "user-123")
		c.Assert(storedErr, qt.IsNil)
		c.Assert(stored.TenantID, qt.Equals, "test-tenant-id")
		c.Assert(stored.Email, qt.Equals, "test@example.com")
	})

	// default_group_id (#1263) — the profile endpoint is the write path for the
	// user's "land in this group on login" preference. The tests below cover the
	// four states the handler must distinguish: absent / null / valid / invalid,
	// plus the cross-tenant rejection that relies on GroupMembershipRegistry.
	t.Run("default_group_id absent leaves stored preference unchanged", func(t *testing.T) {
		c := qt.New(t)
		testUser := setupUser(t)
		existingGroupID := "11111111-1111-1111-1111-111111111111"
		testUser.DefaultGroupID = &existingGroupID
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{"user-123": testUser}}
		memberships := newMockGroupMembershipRegistryForAuth(struct {
			groupID string
			userID  string
		}{groupID: existingGroupID, userID: "user-123"})
		authHandler := apiserver.Auth(apiserver.AuthParams{
			UserRegistry:            userRegistry,
			GroupMembershipRegistry: memberships,
			JWTSecret:               jwtSecret,
		})

		// Send only name — default_group_id is not in the body at all.
		req, resp := makeRequest(t, makeToken(t), map[string]string{"name": "Renamed"})

		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		c.Assert(resp.Code, qt.Equals, http.StatusOK)
		stored, storedErr := userRegistry.Get(context.Background(), "user-123")
		c.Assert(storedErr, qt.IsNil)
		c.Assert(stored.DefaultGroupID, qt.IsNotNil)
		c.Assert(*stored.DefaultGroupID, qt.Equals, existingGroupID)
		c.Assert(stored.Name, qt.Equals, "Renamed")
	})

	t.Run("default_group_id null clears the stored preference", func(t *testing.T) {
		c := qt.New(t)
		testUser := setupUser(t)
		existingGroupID := "11111111-1111-1111-1111-111111111111"
		testUser.DefaultGroupID = &existingGroupID
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{"user-123": testUser}}
		authHandler := apiserver.Auth(apiserver.AuthParams{
			UserRegistry:            userRegistry,
			GroupMembershipRegistry: newMockGroupMembershipRegistryForAuth(),
			JWTSecret:               jwtSecret,
		})

		req, resp := makeRequest(t, makeToken(t), map[string]any{
			"name":             "Same Name",
			"default_group_id": nil,
		})

		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		c.Assert(resp.Code, qt.Equals, http.StatusOK)
		stored, storedErr := userRegistry.Get(context.Background(), "user-123")
		c.Assert(storedErr, qt.IsNil)
		c.Assert(stored.DefaultGroupID, qt.IsNil)
	})

	t.Run("default_group_id can be set to a group the user belongs to", func(t *testing.T) {
		c := qt.New(t)
		testUser := setupUser(t)
		groupID := "22222222-2222-2222-2222-222222222222"
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{"user-123": testUser}}
		memberships := newMockGroupMembershipRegistryForAuth(struct {
			groupID string
			userID  string
		}{groupID: groupID, userID: "user-123"})
		authHandler := apiserver.Auth(apiserver.AuthParams{
			UserRegistry:            userRegistry,
			GroupMembershipRegistry: memberships,
			JWTSecret:               jwtSecret,
		})

		req, resp := makeRequest(t, makeToken(t), map[string]any{
			"name":             "Same Name",
			"default_group_id": groupID,
		})

		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		c.Assert(resp.Code, qt.Equals, http.StatusOK)
		stored, storedErr := userRegistry.Get(context.Background(), "user-123")
		c.Assert(storedErr, qt.IsNil)
		c.Assert(stored.DefaultGroupID, qt.IsNotNil)
		c.Assert(*stored.DefaultGroupID, qt.Equals, groupID)
	})

	t.Run("default_group_id for a group the user does not belong to is rejected", func(t *testing.T) {
		c := qt.New(t)
		testUser := setupUser(t)
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{"user-123": testUser}}
		authHandler := apiserver.Auth(apiserver.AuthParams{
			UserRegistry:            userRegistry,
			GroupMembershipRegistry: newMockGroupMembershipRegistryForAuth(), // empty
			JWTSecret:               jwtSecret,
		})

		req, resp := makeRequest(t, makeToken(t), map[string]any{
			"name":             "Same Name",
			"default_group_id": "33333333-3333-3333-3333-333333333333",
		})

		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		c.Assert(resp.Code, qt.Equals, http.StatusBadRequest)
		// Preference must remain unchanged (nil from setupUser).
		stored, storedErr := userRegistry.Get(context.Background(), "user-123")
		c.Assert(storedErr, qt.IsNil)
		c.Assert(stored.DefaultGroupID, qt.IsNil)
	})

	t.Run("default_group_id with malformed UUID is rejected", func(t *testing.T) {
		c := qt.New(t)
		testUser := setupUser(t)
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{"user-123": testUser}}
		authHandler := apiserver.Auth(apiserver.AuthParams{
			UserRegistry:            userRegistry,
			GroupMembershipRegistry: newMockGroupMembershipRegistryForAuth(),
			JWTSecret:               jwtSecret,
		})

		req, resp := makeRequest(t, makeToken(t), map[string]any{
			"name":             "Same Name",
			"default_group_id": "not-a-uuid",
		})

		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		c.Assert(resp.Code, qt.Equals, http.StatusBadRequest)
	})

	t.Run("registry errors other than NotFound surface as 500, not 400", func(t *testing.T) {
		c := qt.New(t)
		testUser := setupUser(t)
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{"user-123": testUser}}
		// Explicit mock that returns a non-ErrNotFound error to prove the
		// handler distinguishes "you can't pick this group" (client error)
		// from "we couldn't check" (infrastructure error).
		failingMembership := &erroringGroupMembershipRegistry{err: errors.New("simulated DB outage")}
		authHandler := apiserver.Auth(apiserver.AuthParams{
			UserRegistry:            userRegistry,
			GroupMembershipRegistry: failingMembership,
			JWTSecret:               jwtSecret,
		})

		req, resp := makeRequest(t, makeToken(t), map[string]any{
			"name":             "Same Name",
			"default_group_id": "55555555-5555-5555-5555-555555555555",
		})

		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		c.Assert(resp.Code, qt.Equals, http.StatusInternalServerError)
		// Preference must remain unchanged.
		stored, storedErr := userRegistry.Get(context.Background(), "user-123")
		c.Assert(storedErr, qt.IsNil)
		c.Assert(stored.DefaultGroupID, qt.IsNil)
	})

	t.Run("default_group_id empty string clears the preference", func(t *testing.T) {
		c := qt.New(t)
		testUser := setupUser(t)
		existing := "44444444-4444-4444-4444-444444444444"
		testUser.DefaultGroupID = &existing
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{"user-123": testUser}}
		authHandler := apiserver.Auth(apiserver.AuthParams{
			UserRegistry:            userRegistry,
			GroupMembershipRegistry: newMockGroupMembershipRegistryForAuth(),
			JWTSecret:               jwtSecret,
		})

		req, resp := makeRequest(t, makeToken(t), map[string]any{
			"name":             "Same Name",
			"default_group_id": "",
		})

		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		c.Assert(resp.Code, qt.Equals, http.StatusOK)
		stored, storedErr := userRegistry.Get(context.Background(), "user-123")
		c.Assert(storedErr, qt.IsNil)
		c.Assert(stored.DefaultGroupID, qt.IsNil)
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

		c.Assert(resp.Code, qt.Equals, http.StatusUnprocessableEntity)
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
		emailSvc := &mockEmailServiceForAuth{passwordChangedCh: make(chan struct{}, 1)}
		authHandler := apiserver.Auth(apiserver.AuthParams{
			UserRegistry:         userRegistry,
			RefreshTokenRegistry: refreshRegistry,
			BlacklistService:     blacklister,
			EmailService:         emailSvc,
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
		select {
		case <-emailSvc.passwordChangedCh:
			// expected
		case <-time.After(500 * time.Millisecond):
			t.Fatal("expected password-changed email notification to be sent")
		}
	})
}

// TestCheckTokenBlacklist_IatBased verifies that the iat-based user blacklist correctly
// rejects tokens issued before the blacklist event while accepting tokens issued after.
// This is the core security property that allows re-authentication after a password change
// without needing to clear the blacklist entry on login.
func TestCheckTokenBlacklist_IatBased(t *testing.T) {
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	c := qt.New(t)

	testUser := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-iat-test"},
			TenantID: "tenant-1",
		},
		Email:    "iat@example.com",
		Name:     "IAT Test User",
		IsActive: true,
	}

	userRegistry := &mockUserRegistryForAuth{
		users: map[string]*models.User{"user-iat-test": testUser},
	}

	blacklister := services.NewInMemoryTokenBlacklister()
	defer blacklister.Stop()

	makeTokenWithIat := func(t *testing.T, iat time.Time) string {
		t.Helper()
		c := qt.New(t)
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": "user-iat-test",
			"role":    "user",
			"exp":     time.Now().Add(24 * time.Hour).Unix(),
			"iat":     iat.Unix(),
			"jti":     "jti-" + iat.String(),
		})
		tokenString, err := token.SignedString(jwtSecret)
		c.Assert(err, qt.IsNil)
		return tokenString
	}

	makeRequest := func(t *testing.T, tokenString string) *httptest.ResponseRecorder {
		t.Helper()
		middleware := apiserver.JWTMiddleware(jwtSecret, userRegistry, blacklister)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		req := httptest.NewRequest("GET", "/test", nil)
		if tokenString != "" {
			req.Header.Set("Authorization", "Bearer "+tokenString)
		}
		w := httptest.NewRecorder()
		middleware(handler).ServeHTTP(w, req)
		return w
	}

	// Record "before password change" time and issue an old token.
	beforeChange := time.Now().Add(-10 * time.Second)
	oldToken := makeTokenWithIat(t, beforeChange)

	// Blacklist all user tokens (simulates password change).
	err := blacklister.BlacklistUserTokens(context.Background(), "user-iat-test", 30*time.Minute)
	c.Assert(err, qt.IsNil)

	// Issue a new token (simulates fresh login after password change).
	newToken := makeTokenWithIat(t, time.Now().Add(time.Second))

	t.Run("old token rejected after password change", func(t *testing.T) {
		c := qt.New(t)
		w := makeRequest(t, oldToken)
		c.Assert(w.Code, qt.Equals, http.StatusUnauthorized)
	})

	t.Run("new token accepted after password change", func(t *testing.T) {
		c := qt.New(t)
		w := makeRequest(t, newToken)
		c.Assert(w.Code, qt.Equals, http.StatusOK)
	})

	t.Run("no token returns unauthorized", func(t *testing.T) {
		c := qt.New(t)
		w := makeRequest(t, "")
		c.Assert(w.Code, qt.Equals, http.StatusUnauthorized)
	})
}

// TestLogin_AfterPasswordChange verifies the end-to-end scenario:
// 1. User changes password → BlacklistUserTokens is called.
// 2. User logs in again → a new access token is issued.
// 3. The new token (iat > blacklist timestamp) passes the JWT middleware.
// 4. The old token (iat < blacklist timestamp) is rejected by the JWT middleware.
// This test exercises the regression that was originally fixed by UnblacklistUser on login;
// the iat-based approach solves it without clearing the blacklist entry.
func TestLogin_AfterPasswordChange(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		jwtSecret := []byte("test-secret-32-bytes-minimum-length")
		c := qt.New(t)

		testUser := &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "user-pw-change"},
				TenantID: "tenant-1",
			},
			Email:    "pwchange@example.com",
			Name:     "PW Change User",
			IsActive: true,
		}
		testUser.SetPassword("OldPassword123")

		userRegistry := &mockUserRegistryForAuth{
			users: map[string]*models.User{"user-pw-change": testUser},
		}

		blacklister := services.NewInMemoryTokenBlacklister()
		defer blacklister.Stop()

		authHandler := apiserver.Auth(apiserver.AuthParams{
			UserRegistry:     userRegistry,
			BlacklistService: blacklister,
			JWTSecret:        jwtSecret,
		})

		loginTenant := &models.Tenant{
			EntityID: models.EntityID{ID: "tenant-1"},
			Status:   models.TenantStatusActive,
		}

		doRequest := func(t *testing.T, method, path string, body any, token string) *httptest.ResponseRecorder {
			t.Helper()
			c := qt.New(t)
			var bodyBytes []byte
			if body != nil {
				var err error
				bodyBytes, err = json.Marshal(body)
				c.Assert(err, qt.IsNil)
			}
			req := httptest.NewRequest(method, path, bytes.NewBuffer(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			w := httptest.NewRecorder()
			router := chi.NewRouter()
			// Inject tenant context — normally done by PublicTenantMiddleware in APIServer.
			router.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ctx := apiserver.WithTenant(r.Context(), loginTenant)
					next.ServeHTTP(w, r.WithContext(ctx))
				})
			})
			authHandler(router)
			router.ServeHTTP(w, req)
			return w
		}

		// Step 1: Login to get an initial access token (represents "old session").
		loginResp := doRequest(t, "POST", "/login", map[string]string{
			"email":    "pwchange@example.com",
			"password": "OldPassword123",
		}, "")
		c.Assert(loginResp.Code, qt.Equals, http.StatusOK)

		var loginBody apiserver.LoginResponse
		c.Assert(json.Unmarshal(loginResp.Body.Bytes(), &loginBody), qt.IsNil)
		oldToken := loginBody.AccessToken
		c.Assert(oldToken, qt.Not(qt.Equals), "")

		// Access tokens use seconds-precision iat (time.Now().Unix()), and the
		// blacklist marker is stored at seconds precision. Advance the fake clock so
		// the blacklist timestamp is strictly after the old token's iat.
		time.Sleep(1 * time.Second)

		// Step 2: Simulate password change by blacklisting user tokens.
		// (In production this is done by handleChangePassword via blacklistService.BlacklistUserTokens.)
		err := blacklister.BlacklistUserTokens(context.Background(), "user-pw-change", 30*time.Minute)
		c.Assert(err, qt.IsNil)

		// Step 3: Advance the fake clock again so the new token's iat is strictly
		// after the blacklist timestamp.
		time.Sleep(1 * time.Second)

		newLoginResp := doRequest(t, "POST", "/login", map[string]string{
			"email":    "pwchange@example.com",
			"password": "OldPassword123",
		}, "")
		c.Assert(newLoginResp.Code, qt.Equals, http.StatusOK)

		var newLoginBody apiserver.LoginResponse
		c.Assert(json.Unmarshal(newLoginResp.Body.Bytes(), &newLoginBody), qt.IsNil)
		newToken := newLoginBody.AccessToken
		c.Assert(newToken, qt.Not(qt.Equals), "")

		// Step 4: Old token must be rejected by the JWT middleware.
		w := doRequest(t, "GET", "/me", nil, oldToken)
		c.Assert(w.Code, qt.Equals, http.StatusUnauthorized)

		// Step 5: New token must pass the JWT middleware.
		w = doRequest(t, "GET", "/me", nil, newToken)
		c.Assert(w.Code, qt.Equals, http.StatusOK)
	})
}

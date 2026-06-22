package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
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
	memreg "github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// mockRefreshTokenRegistryForAuth implements registry.RefreshTokenRegistry for testing.
// It records calls to RevokeByUserID and Update so tests can assert what was
// invoked. When tokensByHash is non-nil the mock acts as a real registry for
// GetByTokenHash / Create / RevokeByID / RevokeByUserID — enough for the #967
// refresh-rotation + reuse-detection flow to behave like a real store; when
// nil, GetByTokenHash returns ErrNotFound and Update still records the call but
// does not persist anything — so existing change-password tests, which only
// check RevokeByUserID, don't need to opt in.
//
// Create persists a fresh row (generated id, keyed by hash) so rotation's
// persist-then-revoke ordering exercises real lookups; RevokeByID /
// RevokeByUserID set RevokedAt on the stored rows so a subsequent
// GetByTokenHash sees the revoked row UNFILTERED — the exact property
// reuse detection depends on.
type mockRefreshTokenRegistryForAuth struct {
	revokeByUserIDCalled bool
	revokeByUserIDArg    string

	tokensByHash  map[string]*models.RefreshToken
	updates       []models.RefreshToken
	createCount   int
	createErr     error
	revokeByIDErr error
}

func (m *mockRefreshTokenRegistryForAuth) Create(_ context.Context, rt models.RefreshToken) (*models.RefreshToken, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	m.createCount++
	if rt.ID == "" {
		rt.ID = fmt.Sprintf("rt-created-%d", m.createCount)
	}
	stored := rt
	if m.tokensByHash != nil {
		m.tokensByHash[rt.TokenHash] = &stored
	}
	return &stored, nil
}

func (m *mockRefreshTokenRegistryForAuth) Get(_ context.Context, _ string) (*models.RefreshToken, error) {
	return nil, registry.ErrNotFound
}

func (m *mockRefreshTokenRegistryForAuth) List(_ context.Context) ([]*models.RefreshToken, error) {
	return nil, nil
}

func (m *mockRefreshTokenRegistryForAuth) Update(_ context.Context, rt models.RefreshToken) (*models.RefreshToken, error) {
	m.updates = append(m.updates, rt)
	if m.tokensByHash != nil {
		stored := rt
		m.tokensByHash[rt.TokenHash] = &stored
	}
	return &rt, nil
}

func (m *mockRefreshTokenRegistryForAuth) Delete(_ context.Context, _ string) error {
	return nil
}

func (m *mockRefreshTokenRegistryForAuth) Count(_ context.Context) (int, error) {
	return 0, nil
}

func (m *mockRefreshTokenRegistryForAuth) GetByTokenHash(_ context.Context, hash string) (*models.RefreshToken, error) {
	if rt, ok := m.tokensByHash[hash]; ok {
		return rt, nil
	}
	return nil, registry.ErrNotFound
}

func (m *mockRefreshTokenRegistryForAuth) GetByUserID(_ context.Context, _ string) ([]*models.RefreshToken, error) {
	return nil, nil
}

func (m *mockRefreshTokenRegistryForAuth) ListActiveByUserID(_ context.Context, _ string) ([]*models.RefreshToken, error) {
	return nil, nil
}

func (m *mockRefreshTokenRegistryForAuth) RevokeByUserID(_ context.Context, userID string) error {
	m.revokeByUserIDCalled = true
	m.revokeByUserIDArg = userID
	now := time.Now()
	for _, rt := range m.tokensByHash {
		if rt.UserID == userID && rt.RevokedAt == nil {
			rt.RevokedAt = &now
		}
	}
	return nil
}

func (m *mockRefreshTokenRegistryForAuth) RevokeByID(_ context.Context, userID, id string) error {
	if m.revokeByIDErr != nil {
		return m.revokeByIDErr
	}
	for _, rt := range m.tokensByHash {
		if rt.ID == id && rt.UserID == userID {
			if rt.RevokedAt == nil {
				now := time.Now()
				rt.RevokedAt = &now
			}
			return nil
		}
	}
	return registry.ErrNotFound
}

func (m *mockRefreshTokenRegistryForAuth) RevokeAllExceptID(_ context.Context, _ string, _ string) error {
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

func (m *mockEmailServiceForAuth) SendMagicLinkEmail(_ context.Context, _ string, _ string, _ string) error {
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

func (m *mockEmailServiceForAuth) SendWarrantyReminderEmail(_ context.Context, _ string, _ string, _ string, _ string, _ string, _ int) error {
	return nil
}

func (m *mockEmailServiceForAuth) SendGroupInviteEmail(_ context.Context, _, _, _, _, _ string, _ time.Time) error {
	return nil
}

func (m *mockEmailServiceForAuth) SendStorageQuotaWarningEmail(_ context.Context, _, _, _ string, _, _ int, _, _ string, _ []string, _, _ string) error {
	return nil
}

func (m *mockEmailServiceForAuth) SendLoanReminderEmail(_ context.Context, _, _, _, _, _, _, _, _ string, _ int) error {
	return nil
}

func (m *mockEmailServiceForAuth) SendMaintenanceReminderEmail(_ context.Context, _, _, _, _, _, _ string, _ int) error {
	return nil
}

func (m *mockEmailServiceForAuth) SendFeedbackEmail(_ context.Context, _, _, _, _, _, _, _ string, _ []string) error {
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

func (m *mockUserRegistryForAuth) ListAdminByTenant(ctx context.Context, tenantID string, opts registry.AdminUserListOptions) ([]*registry.AdminUserListItem, int, error) {
	return nil, 0, nil
}

func (m *mockUserRegistryForAuth) CountSessionsByUser(ctx context.Context, userID string) (int, error) {
	return 0, nil
}

// mockGroupMembershipRegistryForAuth satisfies registry.GroupMembershipRegistry for the
// default_group_id membership check (#1263). Only GetByGroupAndUser is exercised by the
// auth handler; the rest return zero values.
//
// Per #1592 the handler also calls ListByUser when clearing default_group_id;
// `byUser` is the slice it returns, defaulting to nil. Use
// newMockGroupMembershipRegistryForAuthWithList to populate that path.
type mockGroupMembershipRegistryForAuth struct {
	members map[string]*models.GroupMembership // key: groupID|userID
	byUser  []*models.GroupMembership
}

func newMockGroupMembershipRegistryForAuth(pairs ...struct {
	groupID string
	userID  string
}) *mockGroupMembershipRegistryForAuth {
	m := &mockGroupMembershipRegistryForAuth{members: map[string]*models.GroupMembership{}}
	for _, p := range pairs {
		key := p.groupID + "|" + p.userID
		m.members[key] = &models.GroupMembership{
			TenantAwareEntityID: models.TenantAwareEntityID{
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
	return m.byUser, nil
}

// newMockGroupMembershipRegistryForAuthWithList returns a registry whose
// ListByUser yields the given slice; useful for the #1592 "reject clear
// when memberships exist" test branch.
func newMockGroupMembershipRegistryForAuthWithList(rows []*models.GroupMembership) *mockGroupMembershipRegistryForAuth {
	return &mockGroupMembershipRegistryForAuth{
		members: map[string]*models.GroupMembership{},
		byUser:  rows,
	}
}

func (m *mockGroupMembershipRegistryForAuth) CountByUser(_ context.Context, _, _ string) (int, error) {
	return 0, nil
}

func (m *mockGroupMembershipRegistryForAuth) CountAdminsByGroup(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (m *mockGroupMembershipRegistryForAuth) CountOwnersByGroup(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (m *mockGroupMembershipRegistryForAuth) CountByGroup(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (m *mockGroupMembershipRegistryForAuth) CountByGroups(_ context.Context, ids []string) (map[string]int, error) {
	out := make(map[string]int, len(ids))
	for _, id := range ids {
		out[id] = 0
	}
	return out, nil
}

func (m *mockGroupMembershipRegistryForAuth) ListByGroupWithUsers(_ context.Context, _ string) ([]*models.MembershipWithUser, error) {
	return nil, nil
}

func (m *mockGroupMembershipRegistryForAuth) ListByGroupWithUsersAdmin(_ context.Context, _ string) ([]*models.MembershipWithUser, error) {
	return nil, nil
}

func (m *mockGroupMembershipRegistryForAuth) CreateUnderCap(_ context.Context, _ models.GroupMembership, _ int) (*models.GroupMembership, bool, error) {
	return nil, false, nil
}

func (m *mockGroupMembershipRegistryForAuth) DeleteWithMemberInvariants(_ context.Context, _ string) error {
	return nil
}

func (m *mockGroupMembershipRegistryForAuth) UpdateRoleWithMemberInvariants(_ context.Context, _ string, _ models.GroupRole) (*models.GroupMembership, error) {
	return nil, nil
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

func (m *erroringGroupMembershipRegistry) CountByUser(_ context.Context, _, _ string) (int, error) {
	return 0, m.err
}

func (m *erroringGroupMembershipRegistry) CountAdminsByGroup(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (m *erroringGroupMembershipRegistry) CountOwnersByGroup(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (m *erroringGroupMembershipRegistry) CountByGroup(_ context.Context, _ string) (int, error) {
	return 0, m.err
}

func (m *erroringGroupMembershipRegistry) CountByGroups(_ context.Context, ids []string) (map[string]int, error) {
	if m.err != nil {
		return nil, m.err
	}
	out := make(map[string]int, len(ids))
	for _, id := range ids {
		out[id] = 0
	}
	return out, nil
}

func (m *erroringGroupMembershipRegistry) ListByGroupWithUsers(_ context.Context, _ string) ([]*models.MembershipWithUser, error) {
	return nil, m.err
}

func (m *erroringGroupMembershipRegistry) ListByGroupWithUsersAdmin(_ context.Context, _ string) ([]*models.MembershipWithUser, error) {
	return nil, m.err
}

func (m *erroringGroupMembershipRegistry) CreateUnderCap(_ context.Context, _ models.GroupMembership, _ int) (*models.GroupMembership, bool, error) {
	return nil, false, m.err
}

func (m *erroringGroupMembershipRegistry) DeleteWithMemberInvariants(_ context.Context, _ string) error {
	return m.err
}

func (m *erroringGroupMembershipRegistry) UpdateRoleWithMemberInvariants(_ context.Context, _ string, _ models.GroupRole) (*models.GroupMembership, error) {
	return nil, m.err
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
	// Set password hash for "Password123"
	testUser.SetPassword("Password123")

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
				"password": "Password123",
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
				"password": "Password123",
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
				"password": "Password123",
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

func TestAuthAPI_Refresh(t *testing.T) {
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")

	const (
		userID   = "user-refresh"
		tenantID = "test-tenant-id"
	)

	makeUser := func(active bool) *models.User {
		return &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: userID},
				TenantID: tenantID,
			},
			Email:    "refresh@example.com",
			Name:     "Refresh User",
			IsActive: active,
		}
	}

	// storeToken creates a valid (or doctored, via mutate) refresh token in the
	// registry and returns the raw cookie value to send in the request.
	storeToken := func(reg *mockRefreshTokenRegistryForAuth, mutate func(rt *models.RefreshToken)) string {
		raw, hash, err := models.GenerateRefreshToken()
		if err != nil {
			t.Fatalf("GenerateRefreshToken: %v", err)
		}
		rt := &models.RefreshToken{
			TenantUserAwareEntityID: models.TenantUserAwareEntityID{
				EntityID: models.EntityID{ID: "rt-" + hash[:8]},
				TenantID: tenantID,
				UserID:   userID,
			},
			TokenHash: hash,
			ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
			CreatedAt: time.Now(),
		}
		if mutate != nil {
			mutate(rt)
		}
		reg.tokensByHash[hash] = rt
		return raw
	}

	doRefresh := func(t *testing.T, params apiserver.AuthParams, cookieValue string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest("POST", "/refresh", nil)
		if cookieValue != "" {
			// #nosec G124 -- test-only cookie added to a httptest.NewRequest; transport security is irrelevant here.
			req.AddCookie(&http.Cookie{Name: "refresh_token", Value: cookieValue})
		}
		resp := httptest.NewRecorder()
		router := chi.NewRouter()
		apiserver.Auth(params)(router)
		router.ServeHTTP(resp, req)
		return resp
	}

	t.Run("valid refresh token returns new access token", func(t *testing.T) {
		c := qt.New(t)
		userReg := &mockUserRegistryForAuth{users: map[string]*models.User{userID: makeUser(true)}}
		refreshReg := &mockRefreshTokenRegistryForAuth{tokensByHash: map[string]*models.RefreshToken{}}
		raw := storeToken(refreshReg, nil)

		resp := doRefresh(t, apiserver.AuthParams{
			UserRegistry:         userReg,
			RefreshTokenRegistry: refreshReg,
			JWTSecret:            jwtSecret,
		}, raw)

		c.Assert(resp.Code, qt.Equals, http.StatusOK)

		var body apiserver.LoginResponse
		c.Assert(json.Unmarshal(resp.Body.Bytes(), &body), qt.IsNil)
		c.Assert(body.AccessToken, qt.Not(qt.Equals), "")
		c.Assert(body.User, qt.IsNotNil)
		c.Assert(body.User.ID, qt.Equals, userID)

		// The new access token must be valid and signed with the same secret.
		token, err := jwt.Parse(body.AccessToken, func(*jwt.Token) (any, error) { return jwtSecret, nil })
		c.Assert(err, qt.IsNil)
		c.Assert(token.Valid, qt.IsTrue)
		claims, ok := token.Claims.(jwt.MapClaims)
		c.Assert(ok, qt.IsTrue)
		c.Assert(claims["user_id"], qt.Equals, userID)

		// Rotation (#967 H2): a fresh row is persisted and the consumed row is
		// not bumped via Update — the new no-rotation-removed contract.
		c.Assert(refreshReg.createCount, qt.Equals, 1)
		c.Assert(refreshReg.updates, qt.HasLen, 0)
	})

	t.Run("missing refresh cookie returns 401", func(t *testing.T) {
		c := qt.New(t)
		userReg := &mockUserRegistryForAuth{users: map[string]*models.User{userID: makeUser(true)}}
		refreshReg := &mockRefreshTokenRegistryForAuth{tokensByHash: map[string]*models.RefreshToken{}}

		resp := doRefresh(t, apiserver.AuthParams{
			UserRegistry:         userReg,
			RefreshTokenRegistry: refreshReg,
			JWTSecret:            jwtSecret,
		}, "")

		c.Assert(resp.Code, qt.Equals, http.StatusUnauthorized)
		// No cookie was sent — the handler must not emit a Set-Cookie clearing
		// header for a non-existent cookie.
		c.Assert(resp.Header().Values("Set-Cookie"), qt.HasLen, 0)
	})

	t.Run("revoked refresh token returns 401", func(t *testing.T) {
		c := qt.New(t)
		userReg := &mockUserRegistryForAuth{users: map[string]*models.User{userID: makeUser(true)}}
		refreshReg := &mockRefreshTokenRegistryForAuth{tokensByHash: map[string]*models.RefreshToken{}}
		revokedAt := time.Now().Add(-time.Minute)
		raw := storeToken(refreshReg, func(rt *models.RefreshToken) {
			rt.RevokedAt = &revokedAt
		})

		resp := doRefresh(t, apiserver.AuthParams{
			UserRegistry:         userReg,
			RefreshTokenRegistry: refreshReg,
			JWTSecret:            jwtSecret,
		}, raw)

		c.Assert(resp.Code, qt.Equals, http.StatusUnauthorized)
		// last_used_at must NOT be updated for an invalid token.
		c.Assert(refreshReg.updates, qt.HasLen, 0)
		// The cookie should be cleared so the browser stops re-sending it.
		c.Assert(refreshCookieCleared(resp), qt.IsTrue)
	})

	t.Run("expired refresh token returns 401", func(t *testing.T) {
		c := qt.New(t)
		userReg := &mockUserRegistryForAuth{users: map[string]*models.User{userID: makeUser(true)}}
		refreshReg := &mockRefreshTokenRegistryForAuth{tokensByHash: map[string]*models.RefreshToken{}}
		raw := storeToken(refreshReg, func(rt *models.RefreshToken) {
			rt.ExpiresAt = time.Now().Add(-time.Minute)
		})

		resp := doRefresh(t, apiserver.AuthParams{
			UserRegistry:         userReg,
			RefreshTokenRegistry: refreshReg,
			JWTSecret:            jwtSecret,
		}, raw)

		c.Assert(resp.Code, qt.Equals, http.StatusUnauthorized)
		c.Assert(refreshReg.updates, qt.HasLen, 0)
		c.Assert(refreshCookieCleared(resp), qt.IsTrue)
	})

	t.Run("inactive user returns 401", func(t *testing.T) {
		c := qt.New(t)
		userReg := &mockUserRegistryForAuth{users: map[string]*models.User{userID: makeUser(false)}}
		refreshReg := &mockRefreshTokenRegistryForAuth{tokensByHash: map[string]*models.RefreshToken{}}
		raw := storeToken(refreshReg, nil)

		resp := doRefresh(t, apiserver.AuthParams{
			UserRegistry:         userReg,
			RefreshTokenRegistry: refreshReg,
			JWTSecret:            jwtSecret,
		}, raw)

		c.Assert(resp.Code, qt.Equals, http.StatusUnauthorized)
		c.Assert(refreshReg.updates, qt.HasLen, 0)
		c.Assert(refreshCookieCleared(resp), qt.IsTrue)
	})

	t.Run("nil refresh token registry returns 501", func(t *testing.T) {
		c := qt.New(t)
		userReg := &mockUserRegistryForAuth{users: map[string]*models.User{userID: makeUser(true)}}

		// RefreshTokenRegistry intentionally omitted.
		resp := doRefresh(t, apiserver.AuthParams{
			UserRegistry: userReg,
			JWTSecret:    jwtSecret,
		}, "any-cookie-value")

		c.Assert(resp.Code, qt.Equals, http.StatusNotImplemented)
	})
}

// TestAuthAPI_Refresh_BearerSchemeIsCaseInsensitive is the #1812 regression
// test for accessTokenIsImpersonation. The refresh endpoint reads the
// Authorization header to detect impersonation access tokens (which it must
// reject). Before #1812 the scheme match was case-sensitive on the exact
// prefix `"Bearer "`, so an FE that sent `Authorization: bearer <token>` —
// permitted by RFC 7235 §2.1 — would bypass the impersonation guard and
// silently receive a fresh admin access token. This test asserts that a
// lower-case scheme is correctly classified as an impersonation token and
// rejected with 401 ErrImpersonationTokenCannotRefresh.
func TestAuthAPI_Refresh_BearerSchemeIsCaseInsensitive(t *testing.T) {
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	const (
		userID   = "user-refresh-imp"
		tenantID = "test-tenant-id"
	)

	userReg := &mockUserRegistryForAuth{users: map[string]*models.User{userID: {
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: userID},
			TenantID: tenantID,
		},
		Email:    "imp@example.com",
		Name:     "Imp Refresh User",
		IsActive: true,
	}}}
	refreshReg := &mockRefreshTokenRegistryForAuth{tokensByHash: map[string]*models.RefreshToken{}}

	// Mint an impersonation access token. accessTokenIsImpersonation reads only
	// the `imp` claim, so the rest of the claims can be minimal.
	impToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":         userID,
		"token_type":      "access",
		"imp":             true,
		"impersonated_by": "admin-user",
		"jti":             "imp-jti",
		"exp":             time.Now().Add(15 * time.Minute).Unix(),
	})
	impTokenString, err := impToken.SignedString(jwtSecret)
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}

	cases := []struct {
		name       string
		authHeader string
	}{
		{"lower-case bearer", "bearer " + impTokenString},
		{"upper-case BEARER", "BEARER " + impTokenString},
		{"mixed-case BeArEr", "BeArEr " + impTokenString},
		{"canonical Bearer", "Bearer " + impTokenString},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			req := httptest.NewRequest("POST", "/refresh", nil)
			req.Header.Set("Authorization", tc.authHeader)
			resp := httptest.NewRecorder()

			router := chi.NewRouter()
			apiserver.Auth(apiserver.AuthParams{
				UserRegistry:         userReg,
				RefreshTokenRegistry: refreshReg,
				JWTSecret:            jwtSecret,
			})(router)
			router.ServeHTTP(resp, req)

			// All four header forms must be classified as impersonation tokens
			// and rejected with 401 carrying the dedicated error message.
			c.Assert(resp.Code, qt.Equals, http.StatusUnauthorized)
			c.Assert(strings.TrimSpace(resp.Body.String()), qt.Equals, apiserver.ErrImpersonationTokenCannotRefresh.Error())
		})
	}
}

// legacyRefreshCookieDeleted reports whether the response includes a
// Set-Cookie header that deletes the refresh_token cookie pinned at the
// legacy /api/v1/auth path. http.SetCookie with MaxAge=-1 renders as
// "Max-Age=0" on the wire. This is the regression assertion for the PR
// #1771 review bug (#1750): widening the refresh cookie path to /api/v1
// without actively deleting a stale /api/v1/auth cookie leaves a browser
// carrying two refresh_token cookies, reopening the
// refresh-during-impersonation bypass for every pre-upgrade session.
func legacyRefreshCookieDeleted(resp *httptest.ResponseRecorder) bool {
	for _, sc := range resp.Header().Values("Set-Cookie") {
		if !strings.HasPrefix(sc, "refresh_token=") {
			continue
		}
		hasLegacyPath := false
		hasDeletion := false
		for attr := range strings.SplitSeq(sc, ";") {
			attr = strings.TrimSpace(attr)
			if attr == "Path=/api/v1/auth" {
				hasLegacyPath = true
			}
			if attr == "Max-Age=0" {
				hasDeletion = true
			}
		}
		if hasLegacyPath && hasDeletion {
			return true
		}
	}
	return false
}

// TestAuthAPI_Refresh_DeletesLegacyPathCookie is the regression test for the
// PR #1771 review bug (#1750): the /auth/refresh success path must emit a
// Set-Cookie that deletes the pre-upgrade refresh_token cookie scoped to the
// legacy /api/v1/auth path, so a browser holding a stale narrow-path cookie
// drops it instead of carrying two refresh_token cookies.
func TestAuthAPI_Refresh_DeletesLegacyPathCookie(t *testing.T) {
	c := qt.New(t)
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	const (
		userID   = "user-refresh-legacy"
		tenantID = "test-tenant-id"
	)

	userReg := &mockUserRegistryForAuth{users: map[string]*models.User{
		userID: {
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: userID},
				TenantID: tenantID,
			},
			Email:    "refresh-legacy@example.com",
			Name:     "Refresh Legacy User",
			IsActive: true,
		},
	}}
	refreshReg := &mockRefreshTokenRegistryForAuth{tokensByHash: map[string]*models.RefreshToken{}}

	raw, hash, err := models.GenerateRefreshToken()
	c.Assert(err, qt.IsNil)
	refreshReg.tokensByHash[hash] = &models.RefreshToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			EntityID: models.EntityID{ID: "rt-legacy"},
			TenantID: tenantID,
			UserID:   userID,
		},
		TokenHash: hash,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		CreatedAt: time.Now(),
	}

	req := httptest.NewRequest("POST", "/refresh", nil)
	// #nosec G124 -- test-only cookie added to a httptest.NewRequest; transport security is irrelevant here.
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: raw})
	resp := httptest.NewRecorder()
	router := chi.NewRouter()
	apiserver.Auth(apiserver.AuthParams{
		UserRegistry:         userReg,
		RefreshTokenRegistry: refreshReg,
		JWTSecret:            jwtSecret,
	})(router)
	router.ServeHTTP(resp, req)

	c.Assert(resp.Code, qt.Equals, http.StatusOK)
	c.Assert(legacyRefreshCookieDeleted(resp), qt.IsTrue,
		qt.Commentf("refresh success path must emit a Set-Cookie deleting the legacy /api/v1/auth cookie; got %v",
			resp.Header().Values("Set-Cookie")))
}

// refreshCookieValue extracts the value of the refresh_token cookie from a
// response's Set-Cookie headers, or "" when the response does not set one (or
// only clears it via Max-Age=0). Used by the #967 rotation tests to follow the
// cookie chain across refreshes.
func refreshCookieValue(resp *httptest.ResponseRecorder) string {
	for _, c := range resp.Result().Cookies() {
		if c.Name == "refresh_token" && c.MaxAge >= 0 && c.Value != "" {
			return c.Value
		}
	}
	return ""
}

// newRefreshTestUser builds a minimal active user for the #967 refresh tests.
func newRefreshTestUser(userID, tenantID string) *models.User {
	return &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: userID},
			TenantID: tenantID,
		},
		Email:    "rotate@example.com",
		Name:     "Rotate User",
		IsActive: true,
	}
}

// seedRefreshRow creates a live refresh-token row in a real memory registry and
// returns the raw cookie value the client would send. Mirrors how login()
// persists the row, so the #967 rotation/reuse tests exercise the real
// GetByTokenHash / Create / RevokeByID lockstep rather than a hand-rolled mock.
func seedRefreshRow(t *testing.T, reg registry.RefreshTokenRegistry, userID, tenantID string) string {
	t.Helper()
	raw, hash, err := models.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	_, err = reg.Create(context.Background(), models.RefreshToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: tenantID,
			UserID:   userID,
		},
		TokenHash: hash,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("seed refresh row: %v", err)
	}
	return raw
}

// postRefresh fires POST /refresh with the given cookie value against a freshly
// wired Auth router and returns the recorder.
func postRefresh(params apiserver.AuthParams, cookieValue string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/refresh", nil)
	if cookieValue != "" {
		// #nosec G124 -- test-only cookie added to a httptest.NewRequest.
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: cookieValue})
	}
	resp := httptest.NewRecorder()
	router := chi.NewRouter()
	apiserver.Auth(params)(router)
	router.ServeHTTP(resp, req)
	return resp
}

// TestAuthAPI_Refresh_RotatesRefreshToken pins the #967 H2 rotation contract on
// the tenant plane: a successful refresh issues a NEW refresh cookie (value !=
// original) and a fresh access token; replaying the ORIGINAL cookie returns
// 401 (it now hashes to a revoked row), while the ROTATED cookie still works.
func TestAuthAPI_Refresh_RotatesRefreshToken(t *testing.T) {
	c := qt.New(t)
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	const userID, tenantID = "user-rotate", "tenant-rotate"

	refreshReg := memreg.NewRefreshTokenRegistry()
	userReg := &mockUserRegistryForAuth{users: map[string]*models.User{userID: newRefreshTestUser(userID, tenantID)}}
	original := seedRefreshRow(t, refreshReg, userID, tenantID)

	params := apiserver.AuthParams{UserRegistry: userReg, RefreshTokenRegistry: refreshReg, JWTSecret: jwtSecret}

	// First refresh: 200, new cookie, fresh access token.
	resp := postRefresh(params, original)
	c.Assert(resp.Code, qt.Equals, http.StatusOK)
	rotated := refreshCookieValue(resp)
	c.Assert(rotated, qt.Not(qt.Equals), "")
	c.Assert(rotated, qt.Not(qt.Equals), original)

	var body apiserver.LoginResponse
	c.Assert(json.Unmarshal(resp.Body.Bytes(), &body), qt.IsNil)
	c.Assert(body.AccessToken, qt.Not(qt.Equals), "")

	// The rotated cookie works on its first use (asserted BEFORE the replay
	// below, which fires the H4 theft cascade and revokes everything).
	again := postRefresh(params, rotated)
	c.Assert(again.Code, qt.Equals, http.StatusOK)

	// Replaying the ORIGINAL cookie now fails: it hashes to a revoked row.
	replay := postRefresh(params, original)
	c.Assert(replay.Code, qt.Equals, http.StatusUnauthorized)
}

// TestAuthAPI_Refresh_RevokesConsumedRowAndStampsNewRTI pins two #967 H2
// invariants: after a refresh the consumed row carries RevokedAt, and the
// minted access token's "rti" claim points at the NEW row id (never the old
// one). The rti linkage is load-bearing for /users/me/sessions — stamping the
// stale id would make the live session look non-current.
func TestAuthAPI_Refresh_RevokesConsumedRowAndStampsNewRTI(t *testing.T) {
	c := qt.New(t)
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	const userID, tenantID = "user-rti", "tenant-rti"

	refreshReg := memreg.NewRefreshTokenRegistry()
	userReg := &mockUserRegistryForAuth{users: map[string]*models.User{userID: newRefreshTestUser(userID, tenantID)}}
	original := seedRefreshRow(t, refreshReg, userID, tenantID)

	// Capture the original row id before refresh.
	originalRow, err := refreshReg.GetByTokenHash(context.Background(), models.HashRefreshToken(original))
	c.Assert(err, qt.IsNil)
	oldRowID := originalRow.ID

	resp := postRefresh(apiserver.AuthParams{UserRegistry: userReg, RefreshTokenRegistry: refreshReg, JWTSecret: jwtSecret}, original)
	c.Assert(resp.Code, qt.Equals, http.StatusOK)

	// The consumed row must now be revoked.
	consumed, err := refreshReg.GetByTokenHash(context.Background(), models.HashRefreshToken(original))
	c.Assert(err, qt.IsNil)
	c.Assert(consumed.RevokedAt, qt.IsNotNil)

	// Resolve the new row id from the rotated cookie, then assert the access
	// token's rti claim equals it (and not the old row id).
	rotated := refreshCookieValue(resp)
	newRow, err := refreshReg.GetByTokenHash(context.Background(), models.HashRefreshToken(rotated))
	c.Assert(err, qt.IsNil)

	var body apiserver.LoginResponse
	c.Assert(json.Unmarshal(resp.Body.Bytes(), &body), qt.IsNil)
	token, err := jwt.Parse(body.AccessToken, func(*jwt.Token) (any, error) { return jwtSecret, nil })
	c.Assert(err, qt.IsNil)
	claims, ok := token.Claims.(jwt.MapClaims)
	c.Assert(ok, qt.IsTrue)
	c.Assert(claims["rti"], qt.Equals, newRow.ID)
	c.Assert(claims["rti"], qt.Not(qt.Equals), oldRowID)
}

// TestAuthAPI_Refresh_PersistFailurePreservesSession pins the #967 H2
// crash-safety contract: persist-BEFORE-revoke means a transient Create error
// during rotation returns 500 WITHOUT revoking the consumed row, so the
// caller's existing session stays usable rather than half-rotated / signed out.
func TestAuthAPI_Refresh_PersistFailurePreservesSession(t *testing.T) {
	c := qt.New(t)
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	const userID, tenantID = "user-persistfail", "tenant-persistfail"

	raw, hash, err := models.GenerateRefreshToken()
	c.Assert(err, qt.IsNil)
	refreshReg := &mockRefreshTokenRegistryForAuth{
		tokensByHash: map[string]*models.RefreshToken{
			hash: {
				TenantUserAwareEntityID: models.TenantUserAwareEntityID{
					TenantID: tenantID,
					UserID:   userID,
				},
				TokenHash: hash,
				ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
			},
		},
		createErr: errors.New("refresh_tokens insert failed"),
	}
	userReg := &mockUserRegistryForAuth{users: map[string]*models.User{userID: newRefreshTestUser(userID, tenantID)}}

	resp := postRefresh(apiserver.AuthParams{UserRegistry: userReg, RefreshTokenRegistry: refreshReg, JWTSecret: jwtSecret}, raw)

	c.Assert(resp.Code, qt.Equals, http.StatusInternalServerError)
	c.Assert(resp.Body.String(), qt.Contains, "Failed to rotate session")
	// The consumed row must NOT be revoked — a failed rotation leaves the
	// existing session intact (no reuse cascade fired either).
	c.Assert(refreshReg.tokensByHash[hash].RevokedAt, qt.IsNil)
	c.Assert(refreshReg.revokeByUserIDCalled, qt.IsFalse)
}

// TestAuthAPI_Refresh_RevokeOldFailureStill200 pins that revoking the consumed
// row is best-effort: a transient RevokeByID failure during rotation must NOT
// sign the user out — the new token + cookie are already minted, so the
// response stays 200 (a regression turning this into a 500 would log the user
// out on a revoke flake).
func TestAuthAPI_Refresh_RevokeOldFailureStill200(t *testing.T) {
	c := qt.New(t)
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	const userID, tenantID = "user-revokefail", "tenant-revokefail"

	raw, hash, err := models.GenerateRefreshToken()
	c.Assert(err, qt.IsNil)
	refreshReg := &mockRefreshTokenRegistryForAuth{
		tokensByHash: map[string]*models.RefreshToken{
			hash: {
				TenantUserAwareEntityID: models.TenantUserAwareEntityID{
					TenantID: tenantID,
					UserID:   userID,
				},
				TokenHash: hash,
				ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
			},
		},
		revokeByIDErr: errors.New("revoke consumed row flaked"),
	}
	userReg := &mockUserRegistryForAuth{users: map[string]*models.User{userID: newRefreshTestUser(userID, tenantID)}}

	resp := postRefresh(apiserver.AuthParams{UserRegistry: userReg, RefreshTokenRegistry: refreshReg, JWTSecret: jwtSecret}, raw)

	c.Assert(resp.Code, qt.Equals, http.StatusOK)
	c.Assert(refreshCookieValue(resp), qt.Not(qt.Equals), "")
}

// TestAuthAPI_Refresh_ReuseDetectionRevokesAllSessions pins the #967 H4 theft
// cascade: after one rotation, replaying the now-revoked original cookie is
// treated as theft — it 401s with the SAME body as the expired branch, ALL of
// the user's sessions are revoked (so even the legitimately-rotated cookie
// 401s afterwards), and exactly one audit row is written with
// Action=="refresh_token_reuse_detected", Success==false.
func TestAuthAPI_Refresh_ReuseDetectionRevokesAllSessions(t *testing.T) {
	c := qt.New(t)
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	const userID, tenantID = "user-theft", "tenant-theft"

	refreshReg := memreg.NewRefreshTokenRegistry()
	userReg := &mockUserRegistryForAuth{users: map[string]*models.User{userID: newRefreshTestUser(userID, tenantID)}}
	auditReg := memreg.NewAuditLogRegistry()
	auditSvc := services.NewAuditService(auditReg)
	original := seedRefreshRow(t, refreshReg, userID, tenantID)

	params := apiserver.AuthParams{
		UserRegistry:         userReg,
		RefreshTokenRegistry: refreshReg,
		AuditService:         auditSvc,
		JWTSecret:            jwtSecret,
	}

	// Rotate once.
	resp := postRefresh(params, original)
	c.Assert(resp.Code, qt.Equals, http.StatusOK)
	rotated := refreshCookieValue(resp)
	c.Assert(rotated, qt.Not(qt.Equals), "")

	// Replay the now-revoked original: theft detected. Body must be
	// byte-identical to the plain-expired branch (opaque to the attacker).
	theft := postRefresh(params, original)
	c.Assert(theft.Code, qt.Equals, http.StatusUnauthorized)
	c.Assert(strings.TrimSpace(theft.Body.String()), qt.Equals, "Refresh token expired or revoked")

	// Cascade: the rotated (legitimate) session row must ALSO be revoked now.
	// Asserted via the registry rather than by replaying the cookie — a replay
	// would itself be a second reuse event and inflate the audit count.
	rotatedRow, err := refreshReg.GetByTokenHash(context.Background(), models.HashRefreshToken(rotated))
	c.Assert(err, qt.IsNil)
	c.Assert(rotatedRow.RevokedAt, qt.IsNotNil)

	// Exactly one reuse audit row (from the single replay), marked unsuccessful.
	logs, err := auditReg.List(context.Background())
	c.Assert(err, qt.IsNil)
	reuseRows := 0
	for _, l := range logs {
		if l.Action == "refresh_token_reuse_detected" {
			reuseRows++
			c.Assert(l.Success, qt.IsFalse)
		}
	}
	c.Assert(reuseRows, qt.Equals, 1)
}

// TestAuthAPI_Refresh_ExpiredStays401NoCascade pins the #967 H4 boundary: a
// genuinely expired row (RevokedAt == nil, ExpiresAt in the past) must 401
// WITHOUT triggering the theft cascade — the user's OTHER active sessions stay
// live and no reuse audit row is written.
func TestAuthAPI_Refresh_ExpiredStays401NoCascade(t *testing.T) {
	c := qt.New(t)
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	const userID, tenantID = "user-expired", "tenant-expired"

	refreshReg := memreg.NewRefreshTokenRegistry()
	userReg := &mockUserRegistryForAuth{users: map[string]*models.User{userID: newRefreshTestUser(userID, tenantID)}}
	auditReg := memreg.NewAuditLogRegistry()
	auditSvc := services.NewAuditService(auditReg)

	// Seed one expired-but-active row (the cookie under test) and one healthy
	// active row (to prove the cascade did NOT fire).
	expiredRaw, expiredHash, err := models.GenerateRefreshToken()
	c.Assert(err, qt.IsNil)
	_, err = refreshReg.Create(context.Background(), models.RefreshToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: tenantID, UserID: userID},
		TokenHash:               expiredHash,
		ExpiresAt:               time.Now().Add(-time.Minute), // expired, not revoked
	})
	c.Assert(err, qt.IsNil)
	healthyRaw := seedRefreshRow(t, refreshReg, userID, tenantID)

	params := apiserver.AuthParams{
		UserRegistry:         userReg,
		RefreshTokenRegistry: refreshReg,
		AuditService:         auditSvc,
		JWTSecret:            jwtSecret,
	}

	resp := postRefresh(params, expiredRaw)
	c.Assert(resp.Code, qt.Equals, http.StatusUnauthorized)
	c.Assert(strings.TrimSpace(resp.Body.String()), qt.Equals, "Refresh token expired or revoked")

	// No cascade: the healthy row is untouched and still refreshes.
	healthyRow, err := refreshReg.GetByTokenHash(context.Background(), models.HashRefreshToken(healthyRaw))
	c.Assert(err, qt.IsNil)
	c.Assert(healthyRow.RevokedAt, qt.IsNil)

	// No reuse audit row was written.
	logs, err := auditReg.List(context.Background())
	c.Assert(err, qt.IsNil)
	for _, l := range logs {
		c.Assert(l.Action, qt.Not(qt.Equals), "refresh_token_reuse_detected")
	}
}

// TestAuthAPI_Refresh_RateLimited429 pins the #967 H1 dedicated refresh budget:
// hammering /auth/refresh past refreshAttemptsLimit from one IP returns 429
// with a Retry-After header. Uses the real in-memory limiter so the sliding
// window is exercised end-to-end.
func TestAuthAPI_Refresh_RateLimited429(t *testing.T) {
	c := qt.New(t)
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	const userID, tenantID = "user-rl", "tenant-rl"

	refreshReg := memreg.NewRefreshTokenRegistry()
	userReg := &mockUserRegistryForAuth{users: map[string]*models.User{userID: newRefreshTestUser(userID, tenantID)}}
	limiter := services.NewInMemoryAuthRateLimiter()

	params := apiserver.AuthParams{
		UserRegistry:         userReg,
		RefreshTokenRegistry: refreshReg,
		RateLimiter:          limiter,
		JWTSecret:            jwtSecret,
	}

	router := chi.NewRouter()
	apiserver.Auth(params)(router)

	// The refresh budget is 60/15min; fire a few past that from one IP. Each
	// request rotates, so re-seed a fresh cookie each loop to keep the handler
	// past the limiter rather than 401-ing on a revoked cookie — but the
	// limiter rejects before the handler runs once the budget is spent, so the
	// cookie value is irrelevant on the rejected call.
	var last *httptest.ResponseRecorder
	for range 65 {
		raw := seedRefreshRow(t, refreshReg, userID, tenantID)
		req := httptest.NewRequest("POST", "/refresh", nil)
		req.RemoteAddr = "203.0.113.7:54321"
		// #nosec G124 -- test-only cookie.
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: raw})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		last = rec
		if rec.Code == http.StatusTooManyRequests {
			break
		}
	}
	c.Assert(last.Code, qt.Equals, http.StatusTooManyRequests)
	retryAfter, err := strconv.Atoi(last.Header().Get("Retry-After"))
	c.Assert(err, qt.IsNil)
	c.Assert(retryAfter > 0, qt.IsTrue)
}

// TestAuthAPI_Refresh_ImpersonationNotRotated pins the #967 care point that the
// impersonation guard stays FIRST: an impersonation-marker refresh cookie is
// rejected with 401 ErrImpersonationTokenCannotRefresh before any cookie
// lookup, so no new row is created, no cookie is rotated, and no reuse audit
// row is written.
func TestAuthAPI_Refresh_ImpersonationNotRotated(t *testing.T) {
	c := qt.New(t)
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	const userID, tenantID = "user-imp", "tenant-imp"

	refreshReg := memreg.NewRefreshTokenRegistry()
	userReg := &mockUserRegistryForAuth{users: map[string]*models.User{userID: newRefreshTestUser(userID, tenantID)}}
	auditReg := memreg.NewAuditLogRegistry()
	auditSvc := services.NewAuditService(auditReg)

	params := apiserver.AuthParams{
		UserRegistry:         userReg,
		RefreshTokenRegistry: refreshReg,
		AuditService:         auditSvc,
		JWTSecret:            jwtSecret,
	}

	// The impersonation-start flow plants a marker value in the refresh cookie
	// (impersonationRefreshCookieMarker). Its detectable prefix is "imp:".
	resp := postRefresh(params, "imp:some-impersonation-marker-payload")
	c.Assert(resp.Code, qt.Equals, http.StatusUnauthorized)
	c.Assert(strings.TrimSpace(resp.Body.String()), qt.Equals, apiserver.ErrImpersonationTokenCannotRefresh.Error())

	// No row was created and no cookie was rotated.
	c.Assert(refreshCookieValue(resp), qt.Equals, "")
	rows, err := refreshReg.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(rows, qt.HasLen, 0)

	// No reuse audit row.
	logs, err := auditReg.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(logs, qt.HasLen, 0)
}

// refreshCookieCleared reports whether the response includes a Set-Cookie
// header that deletes the refresh_token cookie. http.SetCookie with MaxAge=-1
// is rendered as "Max-Age=0" on the wire, so that's what we match.
func refreshCookieCleared(resp *httptest.ResponseRecorder) bool {
	for _, sc := range resp.Header().Values("Set-Cookie") {
		if strings.HasPrefix(sc, "refresh_token=") && strings.Contains(sc, "Max-Age=0") {
			return true
		}
	}
	return false
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
			"user_id":    "user-123",
			"role":       "user",
			"token_type": "access",
			"exp":        time.Now().Add(24 * time.Hour).Unix(),
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
			"user_id":    "user-123",
			"role":       "user",
			"token_type": "access",
			"exp":        time.Now().Add(24 * time.Hour).Unix(),
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

	t.Run("default_group_id null clears the stored preference when the user has zero memberships", func(t *testing.T) {
		c := qt.New(t)
		testUser := setupUser(t)
		existingGroupID := "11111111-1111-1111-1111-111111111111"
		testUser.DefaultGroupID = &existingGroupID
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{"user-123": testUser}}
		authHandler := apiserver.Auth(apiserver.AuthParams{
			UserRegistry: userRegistry,
			// Empty membership registry → ListByUser returns no rows → clearing
			// the default is permitted by the #1592 invariant.
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

	t.Run("default_group_id null is rejected when the user still has memberships (#1592)", func(t *testing.T) {
		c := qt.New(t)
		testUser := setupUser(t)
		existingGroupID := "11111111-1111-1111-1111-111111111111"
		testUser.DefaultGroupID = &existingGroupID
		userRegistry := &mockUserRegistryForAuth{users: map[string]*models.User{"user-123": testUser}}
		// Membership registry that returns one membership for ListByUser.
		memberships := newMockGroupMembershipRegistryForAuthWithList([]*models.GroupMembership{{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "membership-1"},
				TenantID: "test-tenant-id",
			},
			GroupID:      existingGroupID,
			MemberUserID: "user-123",
			Role:         models.GroupRoleAdmin,
		}})
		authHandler := apiserver.Auth(apiserver.AuthParams{
			UserRegistry:            userRegistry,
			GroupMembershipRegistry: memberships,
			JWTSecret:               jwtSecret,
		})

		req, resp := makeRequest(t, makeToken(t), map[string]any{
			"name":             "Same Name",
			"default_group_id": nil,
		})

		router := chi.NewRouter()
		authHandler(router)
		router.ServeHTTP(resp, req)

		c.Assert(resp.Code, qt.Equals, http.StatusBadRequest)
		stored, storedErr := userRegistry.Get(context.Background(), "user-123")
		c.Assert(storedErr, qt.IsNil)
		c.Assert(stored.DefaultGroupID, qt.IsNotNil)
		c.Assert(*stored.DefaultGroupID, qt.Equals, existingGroupID)
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

	t.Run("default_group_id empty string clears the preference when the user has zero memberships", func(t *testing.T) {
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
			"user_id":    "user-123",
			"role":       "user",
			"token_type": "access",
			"exp":        time.Now().Add(24 * time.Hour).Unix(),
			"jti":        "test-change-pw-jti",
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
			"user_id":    "user-iat-test",
			"role":       "user",
			"token_type": "access",
			"exp":        time.Now().Add(24 * time.Hour).Unix(),
			"iat":        iat.Unix(),
			"jti":        "jti-" + iat.String(),
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

// TestAuthAPI_IsSystemAdminWireField pins the contract surfaced by #1784:
// the FE's `useIsSystemAdmin()` hook reads `is_system_admin` off the
// `/auth/me` and `/auth/login` payloads. The struct field is transient —
// see models.User.IsSystemAdmin — so handlers MUST populate it from
// SystemAdminGrantRegistry.Exists immediately before encoding.
//
// Coverage:
//   - GET /auth/me returns is_system_admin=true when the grant exists.
//   - GET /auth/me returns is_system_admin=false when no grant exists.
//   - POST /auth/login carries the same flag on LoginResponse.user.
//
// Per the "no redundant test assertions" rule, each subtest asserts the
// status code plus the single JSON field under test — no envelope-shape
// assertions.
func TestAuthAPI_IsSystemAdminWireField(t *testing.T) {
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")

	const (
		adminID    = "user-admin"
		nonAdminID = "user-nonadmin"
		tenantID   = "tenant-isa"
	)

	makeUser := func(id, email string) *models.User {
		u := &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: id},
				TenantID: tenantID,
			},
			Email:    email,
			Name:     "Test User",
			IsActive: true,
		}
		// All login tests use the same password so the table-driven
		// LoginResponse case can reuse a single credential.
		if err := u.SetPassword("S0lidPassword!"); err != nil {
			t.Fatalf("SetPassword: %v", err)
		}
		return u
	}

	setupHandler := func(t *testing.T, grantUserIDs ...string) (func(r chi.Router), *models.Tenant) {
		t.Helper()
		userRegistry := &mockUserRegistryForAuth{
			users: map[string]*models.User{
				adminID:    makeUser(adminID, "admin@example.com"),
				nonAdminID: makeUser(nonAdminID, "nonadmin@example.com"),
			},
		}
		grants := memreg.NewSystemAdminGrantRegistry()
		for _, uid := range grantUserIDs {
			if _, err := grants.Grant(context.Background(), uid, nil); err != nil {
				t.Fatalf("seed grant: %v", err)
			}
		}
		handler := apiserver.Auth(apiserver.AuthParams{
			UserRegistry:             userRegistry,
			SystemAdminGrantRegistry: grants,
			JWTSecret:                jwtSecret,
		})
		tenant := &models.Tenant{
			EntityID: models.EntityID{ID: tenantID},
			Status:   models.TenantStatusActive,
		}
		return handler, tenant
	}

	doRequest := func(t *testing.T, handler func(r chi.Router), tenant *models.Tenant, method, path string, body any, token string) *httptest.ResponseRecorder {
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
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := apiserver.WithTenant(r.Context(), tenant)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		handler(router)
		router.ServeHTTP(w, req)
		return w
	}

	signAccessToken := func(t *testing.T, userID string) string {
		t.Helper()
		c := qt.New(t)
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id":    userID,
			"token_type": "access",
			"exp":        time.Now().Add(1 * time.Hour).Unix(),
		})
		tokenString, err := token.SignedString(jwtSecret)
		c.Assert(err, qt.IsNil)
		return tokenString
	}

	t.Run("GetCurrentUser_AdminEmitsIsSystemAdminTrue", func(t *testing.T) {
		c := qt.New(t)
		handler, tenant := setupHandler(t, adminID)
		token := signAccessToken(t, adminID)

		resp := doRequest(t, handler, tenant, "GET", "/me", nil, token)

		c.Assert(resp.Code, qt.Equals, http.StatusOK)
		var user models.User
		c.Assert(json.Unmarshal(resp.Body.Bytes(), &user), qt.IsNil)
		c.Assert(user.IsSystemAdmin, qt.IsTrue)
	})

	t.Run("GetCurrentUser_NonAdminEmitsIsSystemAdminFalse", func(t *testing.T) {
		c := qt.New(t)
		handler, tenant := setupHandler(t /* no grants */)
		token := signAccessToken(t, nonAdminID)

		resp := doRequest(t, handler, tenant, "GET", "/me", nil, token)

		c.Assert(resp.Code, qt.Equals, http.StatusOK)
		var user models.User
		c.Assert(json.Unmarshal(resp.Body.Bytes(), &user), qt.IsNil)
		c.Assert(user.IsSystemAdmin, qt.IsFalse)
	})

	t.Run("Login_ResponseUserCarriesIsSystemAdmin", func(t *testing.T) {
		c := qt.New(t)
		handler, tenant := setupHandler(t, adminID)

		resp := doRequest(t, handler, tenant, "POST", "/login", map[string]string{
			"email":    "admin@example.com",
			"password": "S0lidPassword!",
		}, "")

		c.Assert(resp.Code, qt.Equals, http.StatusOK)
		var body apiserver.LoginResponse
		c.Assert(json.Unmarshal(resp.Body.Bytes(), &body), qt.IsNil)
		c.Assert(body.User, qt.IsNotNil)
		c.Assert(body.User.IsSystemAdmin, qt.IsTrue)
	})
}

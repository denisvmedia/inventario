package apiserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
)

// adminUserFixture extends adminTenantFixture with three additional
// users in the seeded tenant so paging/filter/sort assertions for the
// users endpoint have something to work with beyond the singleton
// `test@example.com`.
func adminUserFixture(c *qt.C) (apiserver.Params, *models.User) {
	c.Helper()
	params, adminUser, _ := newParams()
	promoteToSystemAdmin(c, params, adminUser)

	ctx := context.Background()
	seeds := []models.User{
		{TenantAwareEntityID: models.TenantAwareEntityID{TenantID: adminUser.TenantID}, Email: "alice@example.com", Name: "Alice", IsActive: true},
		{TenantAwareEntityID: models.TenantAwareEntityID{TenantID: adminUser.TenantID}, Email: "bob@example.com", Name: "Bob", IsActive: false},
		{TenantAwareEntityID: models.TenantAwareEntityID{TenantID: adminUser.TenantID}, Email: "carol@example.com", Name: "Carol", IsActive: true},
	}
	for _, u := range seeds {
		must.Assert(u.SetPassword("Password123"))
		must.Must(params.FactorySet.UserRegistry.Create(ctx, u))
	}
	return params, adminUser
}

func TestAdminListTenantUsers_AllowsSystemAdmin(t *testing.T) {
	c := qt.New(t)
	params, adminUser := adminUserFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/tenants/%s/users", adminUser.TenantID), nil)
	addTestUserAuthHeader(req, adminUser.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	var body jsonapi.AdminUsersResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
	// admin + 3 seeded users.
	c.Assert(body.Meta.Total, qt.Equals, 4)
	c.Assert(body.Data, qt.HasLen, 4)
}

func TestAdminListTenantUsers_DeniesNonAdmin(t *testing.T) {
	c := qt.New(t)
	params, user, _ := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/tenants/%s/users", user.TenantID), nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
}

func TestAdminListTenantUsers_FiltersAndSorts(t *testing.T) {
	c := qt.New(t)
	params, adminUser := adminUserFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	tests := []struct {
		name             string
		query            string
		expectedTotal    int
		expectedLen      int
		expectedFirstEml string
	}{
		{
			name:             "filter by q matches one user",
			query:            "?q=alice",
			expectedTotal:    1,
			expectedLen:      1,
			expectedFirstEml: "alice@example.com",
		},
		{
			name:             "is_active=false filters to inactive only",
			query:            "?is_active=false",
			expectedTotal:    1,
			expectedLen:      1,
			expectedFirstEml: "bob@example.com",
		},
		{
			name:             "is_active=true filters to active only",
			query:            "?is_active=true",
			expectedTotal:    3,
			expectedLen:      3,
			expectedFirstEml: "alice@example.com",
		},
		{
			name:             "sort by email desc",
			query:            "?sort=-email",
			expectedTotal:    4,
			expectedLen:      4,
			expectedFirstEml: "test@example.com",
		},
		{
			name:          "page beyond total returns empty data and total intact",
			query:         "?page=100",
			expectedTotal: 4,
			expectedLen:   0,
		},
	}
	for _, tc := range tests {
		c.Run(tc.name, func(c *qt.C) {
			req := httptest.NewRequest(http.MethodGet,
				fmt.Sprintf("/api/v1/admin/tenants/%s/users%s", adminUser.TenantID, tc.query), nil)
			addTestUserAuthHeader(req, adminUser.ID)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			c.Assert(rr.Code, qt.Equals, http.StatusOK)
			var body jsonapi.AdminUsersResponse
			c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
			c.Assert(body.Meta.Total, qt.Equals, tc.expectedTotal)
			c.Assert(body.Data, qt.HasLen, tc.expectedLen)
			if tc.expectedFirstEml != "" && len(body.Data) > 0 {
				c.Assert(body.Data[0].Email, qt.Equals, tc.expectedFirstEml)
			}
		})
	}
}

func TestAdminListTenantUsers_404OnMissingTenant(t *testing.T) {
	c := qt.New(t)
	params, adminUser := adminUserFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tenants/does-not-exist/users", nil)
	addTestUserAuthHeader(req, adminUser.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestAdminGetUser_ReturnsDetail(t *testing.T) {
	c := qt.New(t)
	params, adminUser := adminUserFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/users/%s", adminUser.ID), nil)
	addTestUserAuthHeader(req, adminUser.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	var body jsonapi.AdminUserResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
	c.Assert(body.Data, qt.IsNotNil)
	c.Assert(body.Data.ID, qt.Equals, adminUser.ID)
	c.Assert(body.Data.Email, qt.Equals, adminUser.Email)
	c.Assert(body.Data.IsSystemAdmin, qt.IsTrue)
	// The seeded admin user has one membership on the auto-created group.
	c.Assert(body.Data.GroupMemberships, qt.HasLen, 1)
	c.Assert(body.Data.GroupMemberships[0].Role, qt.Equals, models.GroupRoleAdmin)
	// No refresh tokens yet → zero active sessions.
	c.Assert(body.Data.ActiveSessionCount, qt.Equals, 0)
	// Password hash MUST NOT leak.
	c.Assert(rr.Body.String(), qt.Not(qt.Contains), "password_hash")
}

func TestAdminGetUser_404OnMissingID(t *testing.T) {
	c := qt.New(t)
	params, adminUser := adminUserFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/does-not-exist", nil)
	addTestUserAuthHeader(req, adminUser.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestAdminGetUser_CountsActiveSessions(t *testing.T) {
	c := qt.New(t)
	params, adminUser := adminUserFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	ctx := context.Background()
	// Two active sessions + one revoked + one expired → CountSessionsByUser must
	// return 2. ListActiveByUserID is the memory backend's implementation; it
	// filters revoked_at IS NULL AND expires_at > now.
	future := time.Now().Add(24 * time.Hour)
	past := time.Now().Add(-1 * time.Hour)
	revoked := time.Now()

	// Two valid refresh tokens.
	for i := range 2 {
		must.Must(params.FactorySet.RefreshTokenRegistry.Create(ctx, models.RefreshToken{
			TenantUserAwareEntityID: models.TenantUserAwareEntityID{
				TenantID: adminUser.TenantID,
				UserID:   adminUser.ID,
			},
			TokenHash: fmt.Sprintf("active-%d", i),
			ExpiresAt: future,
		}))
	}
	// One expired.
	must.Must(params.FactorySet.RefreshTokenRegistry.Create(ctx, models.RefreshToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: adminUser.TenantID,
			UserID:   adminUser.ID,
		},
		TokenHash: "expired",
		ExpiresAt: past,
	}))
	// One revoked.
	must.Must(params.FactorySet.RefreshTokenRegistry.Create(ctx, models.RefreshToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: adminUser.TenantID,
			UserID:   adminUser.ID,
		},
		TokenHash: "revoked",
		ExpiresAt: future,
		RevokedAt: &revoked,
	}))

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/users/%s", adminUser.ID), nil)
	addTestUserAuthHeader(req, adminUser.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	var body jsonapi.AdminUserResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
	c.Assert(body.Data.ActiveSessionCount, qt.Equals, 2)
}

func TestAdminGetUser_DeniesNonAdmin(t *testing.T) {
	c := qt.New(t)
	params, user, _ := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/users/%s", user.ID), nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
}

func TestAdminUsersEndpoints_AuditEntries(t *testing.T) {
	c := qt.New(t)
	params, adminUser := adminUserFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	listReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/tenants/%s/users", adminUser.TenantID), nil)
	addTestUserAuthHeader(listReq, adminUser.ID)
	listRR := httptest.NewRecorder()
	handler.ServeHTTP(listRR, listReq)
	c.Assert(listRR.Code, qt.Equals, http.StatusOK)

	getReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/users/%s", adminUser.ID), nil)
	addTestUserAuthHeader(getReq, adminUser.ID)
	getRR := httptest.NewRecorder()
	handler.ServeHTTP(getRR, getReq)
	c.Assert(getRR.Code, qt.Equals, http.StatusOK)

	listEntries := must.Must(params.FactorySet.AuditLogRegistry.ListByAction(context.Background(), "admin.list_tenant_users"))
	c.Assert(listEntries, qt.HasLen, 1)
	c.Assert(listEntries[0].Success, qt.IsTrue)

	getEntries := must.Must(params.FactorySet.AuditLogRegistry.ListByAction(context.Background(), "admin.get_user"))
	c.Assert(getEntries, qt.HasLen, 1)
	c.Assert(getEntries[0].Success, qt.IsTrue)
	c.Assert(getEntries[0].EntityID, qt.IsNotNil)
	c.Assert(*getEntries[0].EntityID, qt.Equals, adminUser.ID)
}

// TestAdminListTenants_CrossTenantBypass verifies the listing returns
// rows from tenants the admin is NOT a member of — the core invariant
// the issue spec asks for ("admin sees rows from a tenant they have
// no membership in").
func TestAdminListTenants_CrossTenantBypass(t *testing.T) {
	c := qt.New(t)
	params, adminUser, slugs := adminTenantFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tenants", nil)
	addTestUserAuthHeader(req, adminUser.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	var body jsonapi.AdminTenantsResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
	// Admin is a member only of "test-org" but the listing surfaces
	// every seeded slug — that's the RLS bypass the issue spec calls
	// for.
	seenSlugs := map[string]bool{}
	for _, item := range body.Data {
		seenSlugs[item.Slug] = true
	}
	for _, want := range slugs {
		c.Assert(seenSlugs[want], qt.IsTrue, qt.Commentf("expected slug %q in admin listing", want))
	}
}

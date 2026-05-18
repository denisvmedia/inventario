package apiserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
)

// adminTenantFixture creates a deterministic fixture used by the
// /admin/tenants endpoint tests. Returns the params with the seeded
// admin user already promoted to system admin and the slugs of the
// three additional tenants so each test can assert on shape rather
// than fragile string equality with auto-generated IDs.
func adminTenantFixture(c *qt.C) (apiserver.Params, *models.User, []string) {
	c.Helper()
	params, user, _ := newParams()
	promoteToSystemAdmin(c, params, user)

	// Seed three additional tenants so paging/filter/sort assertions
	// have something to work with beyond the singleton "Test
	// Organization" from newParams.
	ctx := context.Background()
	seeds := []models.Tenant{
		{Name: "Acme Corp", Slug: "acme", Status: models.TenantStatusActive, PlanID: models.PlanUnlimited.ID},
		{Name: "Bravo LLC", Slug: "bravo", Status: models.TenantStatusSuspended, PlanID: models.PlanUnlimited.ID},
		{Name: "Charlie Inc", Slug: "charlie", Status: models.TenantStatusActive, PlanID: models.PlanUnlimited.ID},
	}
	slugs := make([]string, 0, len(seeds))
	for _, t := range seeds {
		created := must.Must(params.FactorySet.TenantRegistry.Create(ctx, t))
		slugs = append(slugs, created.Slug)
	}
	return params, user, slugs
}

func TestAdminListTenants_AllowsSystemAdmin(t *testing.T) {
	c := qt.New(t)
	params, user, _ := adminTenantFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tenants", nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	var body jsonapi.AdminTenantsResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
	// newParams seeds "test-org" plus the three from the fixture.
	c.Assert(body.Meta.Total, qt.Equals, 4)
	c.Assert(body.Data, qt.HasLen, 4)
	// Pagination headers mirror the envelope.
	c.Assert(rr.Header().Get("X-Total"), qt.Equals, "4")
}

func TestAdminListTenants_DeniesNonAdmin(t *testing.T) {
	c := qt.New(t)
	params, user, _ := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tenants", nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
	c.Assert(rr.Body.String(), qt.Contains, "admin.forbidden")
}

func TestAdminListTenants_DeniesUnauthenticated(t *testing.T) {
	c := qt.New(t)
	params, _, _ := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tenants", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}

func TestAdminListTenants_FiltersAndPaginatesAndSorts(t *testing.T) {
	c := qt.New(t)
	params, user, _ := adminTenantFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	tests := []struct {
		name             string
		query            string
		expectedTotal    int
		expectedFirstSlg string
		expectedLen      int
	}{
		{
			name:             "filter by q narrows to one row",
			query:            "?q=acme",
			expectedTotal:    1,
			expectedFirstSlg: "acme",
			expectedLen:      1,
		},
		{
			name:             "pagination caps per_page",
			query:            "?per_page=2",
			expectedTotal:    4,
			expectedFirstSlg: "", // sort default is name asc → Acme first
			expectedLen:      2,
		},
		{
			name:             "sort descending by name via dash prefix",
			query:            "?sort=-name",
			expectedTotal:    4,
			expectedFirstSlg: "test-org", // "Test Organization" sorts last asc → first desc
			expectedLen:      4,
		},
		{
			name:             "sort by slug ascending via explicit order",
			query:            "?sort=slug&order=asc",
			expectedTotal:    4,
			expectedFirstSlg: "acme",
			expectedLen:      4,
		},
	}
	for _, tc := range tests {
		c.Run(tc.name, func(c *qt.C) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tenants"+tc.query, nil)
			addTestUserAuthHeader(req, user.ID)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			c.Assert(rr.Code, qt.Equals, http.StatusOK)
			var body jsonapi.AdminTenantsResponse
			c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
			c.Assert(body.Meta.Total, qt.Equals, tc.expectedTotal)
			c.Assert(body.Data, qt.HasLen, tc.expectedLen)
			if tc.expectedFirstSlg != "" {
				c.Assert(body.Data[0].Slug, qt.Equals, tc.expectedFirstSlg)
			}
		})
	}
}

func TestAdminGetTenant_ReturnsRowWithCounts(t *testing.T) {
	c := qt.New(t)
	params, user, _ := adminTenantFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/tenants/%s", user.TenantID), nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	var body jsonapi.AdminTenantResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
	c.Assert(body.Data, qt.IsNotNil)
	c.Assert(body.Data.ID, qt.Equals, user.TenantID)
	c.Assert(body.Data.UserCount, qt.Equals, 1)  // one seeded user
	c.Assert(body.Data.GroupCount, qt.Equals, 1) // one seeded group
}

func TestAdminGetTenant_404OnMissingID(t *testing.T) {
	c := qt.New(t)
	params, user, _ := adminTenantFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tenants/does-not-exist", nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestAdminGetTenant_DeniesNonAdmin(t *testing.T) {
	c := qt.New(t)
	params, user, _ := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/tenants/%s", user.TenantID), nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
}

func TestAdminListTenants_WritesAuditLog(t *testing.T) {
	c := qt.New(t)
	params, user, _ := adminTenantFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tenants", nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	entries := must.Must(params.FactorySet.AuditLogRegistry.ListByAction(context.Background(), "admin.list_tenants"))
	c.Assert(entries, qt.HasLen, 1)
	c.Assert(entries[0].Success, qt.IsTrue)
	c.Assert(entries[0].UserID, qt.IsNotNil)
	c.Assert(*entries[0].UserID, qt.Equals, user.ID)
}

func TestAdminGetTenant_AuditLog_Success(t *testing.T) {
	c := qt.New(t)
	params, user, _ := adminTenantFixture(c)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/tenants/%s", user.TenantID), nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	entries := must.Must(params.FactorySet.AuditLogRegistry.ListByAction(context.Background(), "admin.get_tenant"))
	c.Assert(entries, qt.HasLen, 1)
	c.Assert(entries[0].Success, qt.IsTrue)
	c.Assert(entries[0].EntityID, qt.IsNotNil)
	c.Assert(*entries[0].EntityID, qt.Equals, user.TenantID)
}
